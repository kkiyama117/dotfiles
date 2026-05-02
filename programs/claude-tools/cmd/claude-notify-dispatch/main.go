// claude-notify-dispatch shows the Claude Code popup via notify-send,
// blocks on the popup's --wait stdout to receive the action key, and
// (on left-click "default") focuses the originating tmux pane and the
// terminal window before closing the popup.
//
// Inputs (env, populated by claude-notify-hook):
//
//	CLAUDE_NOTIFY_TITLE / BODY / URGENCY
//	CLAUDE_NOTIFY_SESSION_ID
//	CLAUDE_NOTIFY_TMUX_PANE / TMUX_SESSION
//
// State (per session_id, shared with shell era):
//
//	${XDG_RUNTIME_DIR:-/tmp}/claude-notify/sessions/<sid>.id
//
// Held open in flight: the previous notif_id. notify-send --replace-id
// updates the existing popup in place rather than stacking a new one.
//
// Exit code is informational: any error path silently exits 0 so the
// hook side never observes failure (popup-shouldn't-block-claude rule).
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"claude-tools/internal/notify"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
)

const progName = "claude-notify-dispatch"

var logger = obslog.New(progName)

type popupConfig struct {
	title       string
	body        string
	urgency     string
	sessionID   string
	tmuxPane    string
	tmuxSession string
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic recovered in dispatch", "panic", r)
		}
		os.Exit(0)
	}()

	cfg := popupConfig{
		title:       envOrDefault("CLAUDE_NOTIFY_TITLE", "Claude Code"),
		body:        os.Getenv("CLAUDE_NOTIFY_BODY"),
		urgency:     envOrDefault("CLAUDE_NOTIFY_URGENCY", "normal"),
		sessionID:   os.Getenv("CLAUDE_NOTIFY_SESSION_ID"),
		tmuxPane:    os.Getenv("CLAUDE_NOTIFY_TMUX_PANE"),
		tmuxSession: os.Getenv("CLAUDE_NOTIFY_TMUX_SESSION"),
	}

	if _, err := exec.LookPath("notify-send"); err != nil {
		// No popup backend → silent exit 0 (matches shell parity).
		return
	}

	ctx := context.Background()
	runner := proc.RealRunner{}
	dispatch(ctx, cfg, notify.StateDir(), runner, exec.LookPath, os.Getenv)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// dispatch runs the popup state machine end-to-end. All side effects
// are funnelled through `runner` and `lookPath` so tests can drive
// every branch deterministically.
func dispatch(
	ctx context.Context,
	cfg popupConfig,
	stateDir string,
	runner proc.Runner,
	lookPath func(string) (string, error),
	getEnv func(string) string,
) {
	prevID := notify.LoadReplaceID(stateDir, cfg.sessionID)

	notifID, action := showPopup(ctx, runner, cfg, prevID)

	if notifID > 0 {
		if err := notify.SaveReplaceID(stateDir, cfg.sessionID, notifID); err != nil {
			logger.Warn("save replace-id failed", "err", err, "sid", cfg.sessionID)
		}
	}

	if action != "default" {
		// Right-click / closeall / timeout / replaced — nothing to do.
		return
	}

	focusTmux(ctx, runner, cfg)
	focusWM(ctx, runner, lookPath, getEnv)
	closeNotification(ctx, runner, notifID)
}

// showPopup invokes notify-send --print-id --wait. stdout layout:
//
//	line 1: notif_id (uint32, always)
//	line 2: action key (e.g. "default") — only when ActionInvoked fires
//
// Returns (notifID, actionKey). Either may be zero / empty on early
// close, replace, or timeout.
func showPopup(
	ctx context.Context,
	runner proc.Runner,
	cfg popupConfig,
	prevID uint32,
) (uint32, string) {
	args := []string{
		"--app-name=ClaudeCode",
		"--urgency=" + cfg.urgency,
		"--expire-time=0",
		"--action=default=Focus",
		"--hint=string:x-claude-session:" + sessionHint(cfg.sessionID),
		"--print-id",
		"--wait",
	}
	if prevID > 0 {
		args = append(args, "--replace-id="+strconv.FormatUint(uint64(prevID), 10))
	}
	args = append(args, "--", cfg.title, cfg.body)

	out, err := runner.Run(ctx, "notify-send", args...)
	if err != nil {
		// notify-send returning non-zero usually means the popup was
		// rejected by the daemon (rare). Silent.
		logger.Warn("notify-send failed", "err", err)
		return 0, ""
	}
	return parsePopupOutput(out)
}

func sessionHint(sid string) string {
	if sid == "" {
		return "unknown"
	}
	return sid
}

// parsePopupOutput extracts (notif_id, action_key) from
// `notify-send --print-id --wait` stdout. action_key is empty when
// the popup closed without invoking an action.
func parsePopupOutput(out []byte) (uint32, string) {
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Buffer(make([]byte, 0, 64), 256)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) == 0 {
		return 0, ""
	}
	idStr := strings.TrimSpace(lines[0])
	var id uint32
	if n, err := strconv.ParseUint(idStr, 10, 32); err == nil {
		id = uint32(n)
	}
	var action string
	if len(lines) >= 2 {
		action = strings.TrimSpace(lines[1])
	}
	return id, action
}

// focusTmux runs `tmux switch-client -t <session> ; select-pane -t <pane>`
// when both context vars are present and the session exists.
func focusTmux(ctx context.Context, runner proc.Runner, cfg popupConfig) {
	if cfg.tmuxPane == "" || cfg.tmuxSession == "" {
		logger.Info("focus skipped: no tmux context",
			"pane", cfg.tmuxPane, "session", cfg.tmuxSession, "sid", cfg.sessionID)
		return
	}
	if _, err := runner.Run(ctx, "tmux", "has-session", "-t", cfg.tmuxSession); err != nil {
		logger.Info("focus skipped: tmux session gone",
			"session", cfg.tmuxSession, "sid", cfg.sessionID)
		return
	}
	if _, err := runner.Run(ctx, "tmux",
		"switch-client", "-t", cfg.tmuxSession,
		";", "select-pane", "-t", cfg.tmuxPane,
	); err != nil {
		logger.Warn("tmux focus failed", "err", err, "sid", cfg.sessionID)
		return
	}
	logger.Info("focus tmux ok",
		"session", cfg.tmuxSession, "pane", cfg.tmuxPane, "sid", cfg.sessionID)
}

// terminalClasses are the X11 WM_CLASS / Wayland app_id values for
// the supported terminals, in priority order. xdotool / swaymsg search
// each in turn until one activates.
var terminalClasses = []string{"kitty", "ghostty", "wezterm", "Alacritty"}

// focusWM raises the originating terminal window via xdotool (X11) or
// swaymsg (Wayland). No-op when neither is available, neither display
// env var is set, or when the search returns no window.
func focusWM(
	ctx context.Context,
	runner proc.Runner,
	lookPath func(string) (string, error),
	getEnv func(string) string,
) {
	if getEnv("DISPLAY") != "" {
		if _, err := lookPath("xdotool"); err == nil {
			for _, cls := range terminalClasses {
				if _, err := runner.Run(ctx, "xdotool", "search", "--class", cls, "windowactivate"); err == nil {
					logger.Info("focus wm: xdotool", "class", cls)
					return
				}
			}
		}
	}
	if getEnv("WAYLAND_DISPLAY") != "" {
		if _, err := lookPath("swaymsg"); err == nil {
			cmd := `[app_id="kitty"] focus, [app_id="com.mitchellh.ghostty"] focus`
			if _, err := runner.Run(ctx, "swaymsg", "-t", "command", cmd); err == nil {
				logger.Info("focus wm: swaymsg")
				return
			}
		}
	}
	logger.Info("focus wm: no tool available")
}

// closeNotification asks the notification daemon to dismiss popup id.
// Best-effort: errors are logged and swallowed.
func closeNotification(ctx context.Context, runner proc.Runner, id uint32) {
	if id == 0 {
		return
	}
	if _, err := runner.Run(ctx, "gdbus", "call", "--session",
		"--dest=org.freedesktop.Notifications",
		"--object-path=/org/freedesktop/Notifications",
		"--method=org.freedesktop.Notifications.CloseNotification",
		fmt.Sprintf("%d", id),
	); err != nil {
		logger.Warn("CloseNotification failed", "err", err, "id", id)
	}
}
