// claude-notify-hook is the Claude Code hook entry. It parses the
// event/payload, queries git/tmux for context, then forks the sound
// player and the popup dispatcher fire-and-forget so the hook itself
// returns immediately.
//
// Argument $1: event key (notification | stop | subagent-stop | error).
// Stdin: Claude Code hook payload (JSON).
//
// Hook contract: ALWAYS exit 0. Errors and panics are recovered/logged
// but never propagated back to Claude — same `exit 0` invariant as the
// previous shell version.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"claude-tools/internal/notify"
	"claude-tools/internal/notifyd"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
)

const (
	progName        = "claude-notify-hook"
	defaultSoundDir = "/usr/share/sounds/freedesktop/stereo"
)

var logger = obslog.New(progName)

// eventConfig holds the per-event UX defaults (sound, title, body, urgency).
// Mirrors the case statement in the previous shell hook.
type eventConfig struct {
	sound       string // basename within sound dir
	title       string
	defaultBody string
	urgency     string
}

var eventConfigs = map[string]eventConfig{
	"notification":  {"message.oga", "Claude Code", "Awaiting input", "normal"},
	"stop":          {"complete.oga", "Claude Code", "Turn complete", "normal"},
	"subagent-stop": {"bell.oga", "Claude Code", "Subagent finished", "low"},
	"error":         {"dialog-error.oga", "Claude Code", "Error", "critical"},
}

// notification is the composed payload handed off to sound + dispatcher.
// Fields map 1:1 to CLAUDE_NOTIFY_* env vars consumed by the dispatcher.
type notification struct {
	event       string
	soundFile   string // absolute path
	title       string
	body        string
	urgency     string
	sessionID   string
	cwd         string
	tmuxPane    string
	tmuxSession string
}

type payloadShape struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
}

// envLookup is the read-side of the environment, abstracted so tests can
// substitute a map without touching os.Environ.
type envLookup func(key string) string

// backgroundStarter starts an external process fire-and-forget. Setsid
// detaches the child from the hook's controlling terminal so the
// dispatcher can outlive the hook (popup + action loop blocks for minutes).
type backgroundStarter func(name string, args []string, env []string, setsid bool) error

// startBackground is the production fire-and-forget launcher. Tests
// override this package-level variable.
var startBackground backgroundStarter = realStartBackground

// forkDispatchFn is the dispatch fallback invoked when daemon dial fails.
// Tests replace this to observe whether fallback was triggered without
// actually exec-ing claude-notify-dispatch.
var forkDispatchFn func(n notification, getEnv envLookup) = forkDispatch

// daemonDialTimeout is the maximum time dialDaemon will wait to connect and
// write a frame to the daemon socket. Per spec §4.1: 100ms.
const daemonDialTimeout = 100 * time.Millisecond

func main() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic recovered in hook", "panic", r)
		}
		os.Exit(0)
	}()

	event := "notification"
	if len(os.Args) > 1 {
		event = os.Args[1]
	}

	payload, _ := io.ReadAll(os.Stdin)

	ctx := context.Background()
	runner := proc.RealRunner{}
	notif := composeNotification(ctx, event, payload, os.Getenv, runner)

	forkSound(notif, os.Getenv)
	notifyHookWithDial(ctx, notif, notify.SocketPath(), daemonDialTimeout, os.Getenv)
}

// notifyHookWithDial attempts to send the notification to the claude-notifyd
// daemon over sockPath. On success it returns without invoking forkDispatch.
// On any failure (socket absent, timeout, write error) it falls through to
// forkDispatchFn so that popup delivery is never silently dropped.
func notifyHookWithDial(ctx context.Context, n notification, sockPath string, timeout time.Duration, getEnv envLookup) {
	if err := dialDaemon(ctx, sockPath, timeout, n); err != nil {
		logger.Warn("daemon dial failed, falling back to dispatch", "err", err)
		forkDispatchFn(n, getEnv)
	}
}

// dialDaemon connects to the claude-notifyd Unix socket at sockPath, writes a
// single JSON frame (spec §4.2 wire format), then closes the connection.
//
// timeout caps the total time allowed for the dial + write. Per spec §4.1 the
// production value is 100ms so the hook exits quickly in all failure modes.
//
// Returns non-nil on any error: connection refused, timeout, or write failure.
func dialDaemon(ctx context.Context, sockPath string, timeout time.Duration, n notification) error {
	deadline := time.Now().Add(timeout)
	dialer := net.Dialer{Deadline: deadline}
	conn, err := dialer.DialContext(ctx, "unix", sockPath)
	if err != nil {
		return fmt.Errorf("dial %s: %w", sockPath, err)
	}
	defer conn.Close()

	// Enforce deadline on the write too.
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	frame := notifyd.Frame{
		V:           1,
		Op:          notifyd.OpShow,
		SID:         n.sessionID,
		Title:       n.title,
		Body:        n.body,
		Urgency:     n.urgency,
		TmuxPane:    n.tmuxPane,
		TmuxSession: n.tmuxSession,
	}
	data, err := notifyd.Marshal(frame)
	if err != nil {
		return fmt.Errorf("marshal frame: %w", err)
	}
	// Write frame bytes followed by newline delimiter (spec §4.2).
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("write frame: %w", err)
	}
	return nil
}

// composeNotification builds the full notification from event + payload +
// env + git/tmux context. All I/O is funnelled through `runner` and
// `getEnv` so tests can drive it deterministically.
func composeNotification(
	ctx context.Context,
	event string,
	payloadJSON []byte,
	getEnv envLookup,
	runner proc.Runner,
) notification {
	cfg, ok := eventConfigs[event]
	if !ok {
		// Default mirrors the shell wildcard: message.oga / event-as-body / normal.
		cfg = eventConfig{"message.oga", "Claude Code", event, "normal"}
	}

	sd := getEnv("CLAUDE_NOTIFY_SOUND_DIR")
	if sd == "" {
		sd = defaultSoundDir
	}

	body := cfg.defaultBody
	var sid, cwd string
	if len(payloadJSON) > 0 {
		var p payloadShape
		if err := json.Unmarshal(payloadJSON, &p); err == nil {
			if p.Message != "" {
				body = p.Message
			}
			sid = p.SessionID
			cwd = p.Cwd
		}
	}
	if cwd == "" {
		cwd = getEnv("PWD")
	}

	project, branch := gitContext(ctx, runner, cwd)

	tmuxPane := getEnv("TMUX_PANE")
	tmuxSession := tmuxSessionFor(ctx, runner, tmuxPane)

	return notification{
		event:       event,
		soundFile:   filepath.Join(sd, cfg.sound),
		title:       composeTitle(cfg.title, project, branch),
		body:        body,
		urgency:     cfg.urgency,
		sessionID:   sid,
		cwd:         cwd,
		tmuxPane:    tmuxPane,
		tmuxSession: tmuxSession,
	}
}

// gitContext returns (project, branch) for cwd:
//   - project = basename of the main worktree (matches F-6 tmux session name)
//   - branch  = `git branch --show-current`, falling back to short HEAD
//
// Either may be empty when cwd is not a repo or git is missing.
func gitContext(ctx context.Context, runner proc.Runner, cwd string) (string, string) {
	if cwd == "" {
		return "", ""
	}
	if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		return "", ""
	}
	project := mainWorktreeBasename(ctx, runner, cwd)
	branch := currentBranch(ctx, runner, cwd)
	return project, branch
}

func mainWorktreeBasename(ctx context.Context, runner proc.Runner, cwd string) string {
	out, err := runner.Run(ctx, "git", "-C", cwd, "worktree", "list", "--porcelain")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, "worktree "); ok {
			return filepath.Base(strings.TrimSpace(rest))
		}
	}
	return ""
}

func currentBranch(ctx context.Context, runner proc.Runner, cwd string) string {
	if out, err := runner.Run(ctx, "git", "-C", cwd, "branch", "--show-current"); err == nil {
		if b := strings.TrimSpace(string(out)); b != "" {
			return b
		}
	}
	if out, err := runner.Run(ctx, "git", "-C", cwd, "rev-parse", "--short", "HEAD"); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

func tmuxSessionFor(ctx context.Context, runner proc.Runner, pane string) string {
	if pane == "" {
		return ""
	}
	out, err := runner.Run(ctx, "tmux", "display-message", "-p", "-t", pane, "#{session_name}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func composeTitle(base, project, branch string) string {
	switch {
	case project != "" && branch != "":
		return base + " · " + project + "/" + branch
	case project != "":
		return base + " · " + project
	case branch != "":
		return base + " · " + branch
	default:
		return base
	}
}

// forkSound launches the sound player fire-and-forget. No setsid: the
// player lives a few seconds and terminates whether the hook detaches
// or not.
func forkSound(n notification, getEnv envLookup) {
	bin := getEnv("CLAUDE_NOTIFY_SOUND_BIN")
	if bin == "" {
		bin = filepath.Join(getEnv("HOME"), ".local", "bin", "claude-notify-sound")
	}
	if !isExecutable(bin) {
		return
	}
	if err := startBackground(bin, []string{n.soundFile}, os.Environ(), false); err != nil {
		logger.Warn("sound fork failed", "bin", bin, "err", err)
	}
}

// forkDispatch launches the popup dispatcher fire-and-forget with
// setsid so it survives the hook returning. Env carries the popup
// payload (CLAUDE_NOTIFY_*).
func forkDispatch(n notification, getEnv envLookup) {
	bin := getEnv("CLAUDE_NOTIFY_DISPATCH")
	if bin == "" {
		bin = filepath.Join(getEnv("HOME"), ".local", "bin", "claude-notify-dispatch")
	}
	if !isExecutable(bin) {
		// No fallback notify-send invocation here — PR-9 will deliver the
		// Go dispatcher and become canonical. Until then, missing binary
		// degrades to "sound only", same as the shell fallback path that
		// fired notify-send without action support.
		return
	}
	env := append(os.Environ(),
		"CLAUDE_NOTIFY_TITLE="+n.title,
		"CLAUDE_NOTIFY_BODY="+n.body,
		"CLAUDE_NOTIFY_URGENCY="+n.urgency,
		"CLAUDE_NOTIFY_SESSION_ID="+n.sessionID,
		"CLAUDE_NOTIFY_TMUX_PANE="+n.tmuxPane,
		"CLAUDE_NOTIFY_TMUX_SESSION="+n.tmuxSession,
	)
	if err := startBackground(bin, nil, env, true); err != nil {
		logger.Warn("dispatch fork failed", "bin", bin, "err", err)
	}
}

func isExecutable(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

// realStartBackground forks the binary fire-and-forget. With setsid=true
// the child becomes session leader, which is what the dispatcher needs
// to survive the hook returning.
func realStartBackground(name string, args []string, env []string, setsid bool) error {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	if setsid {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	cmd.Stdin = nil
	if dn, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
		cmd.Stdout = dn
		cmd.Stderr = dn
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Reap async so the child doesn't become a zombie if we exit before it does.
	go func() { _ = cmd.Wait() }()
	return nil
}
