package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"claude-tools/internal/proc"
)

// envFromMap returns an envLookup that reads from m only.
func envFromMap(m map[string]string) envLookup {
	return func(k string) string { return m[k] }
}

func emptyContextRunner() *proc.FakeRunner {
	return proc.NewFakeRunner()
}

func TestComposeNotification_EventMapping(t *testing.T) {
	tests := []struct {
		event       string
		wantSound   string
		wantTitle   string
		wantBody    string
		wantUrgency string
	}{
		{"notification", "message.oga", "Claude Code", "Awaiting input", "normal"},
		{"stop", "complete.oga", "Claude Code", "Turn complete", "normal"},
		{"subagent-stop", "bell.oga", "Claude Code", "Subagent finished", "low"},
		{"error", "dialog-error.oga", "Claude Code", "Error", "critical"},
		{"weird-custom", "message.oga", "Claude Code", "weird-custom", "normal"},
	}
	for _, tc := range tests {
		t.Run(tc.event, func(t *testing.T) {
			n := composeNotification(
				context.Background(),
				tc.event,
				nil,
				envFromMap(nil),
				emptyContextRunner(),
			)
			if filepath.Base(n.soundFile) != tc.wantSound {
				t.Errorf("soundFile = %q, want basename %q", n.soundFile, tc.wantSound)
			}
			if n.title != tc.wantTitle {
				t.Errorf("title = %q, want %q", n.title, tc.wantTitle)
			}
			if n.body != tc.wantBody {
				t.Errorf("body = %q, want %q", n.body, tc.wantBody)
			}
			if n.urgency != tc.wantUrgency {
				t.Errorf("urgency = %q, want %q", n.urgency, tc.wantUrgency)
			}
		})
	}
}

func TestComposeNotification_PayloadOverridesBody(t *testing.T) {
	payload := []byte(`{"message":"Custom prompt","session_id":"sess-42","cwd":""}`)
	n := composeNotification(
		context.Background(),
		"notification",
		payload,
		envFromMap(nil),
		emptyContextRunner(),
	)
	if n.body != "Custom prompt" {
		t.Errorf("body = %q, want %q", n.body, "Custom prompt")
	}
	if n.sessionID != "sess-42" {
		t.Errorf("sessionID = %q, want sess-42", n.sessionID)
	}
}

func TestComposeNotification_MalformedPayloadKeepsDefaults(t *testing.T) {
	n := composeNotification(
		context.Background(),
		"stop",
		[]byte("not json"),
		envFromMap(nil),
		emptyContextRunner(),
	)
	if n.body != "Turn complete" {
		t.Errorf("body = %q, want default 'Turn complete'", n.body)
	}
	if n.sessionID != "" {
		t.Errorf("sessionID should be empty, got %q", n.sessionID)
	}
}

func TestComposeNotification_CwdFallbackToPWD(t *testing.T) {
	pwdDir := t.TempDir()
	n := composeNotification(
		context.Background(),
		"notification",
		nil,
		envFromMap(map[string]string{"PWD": pwdDir}),
		emptyContextRunner(),
	)
	if n.cwd != pwdDir {
		t.Errorf("cwd = %q, want %q (fallback to PWD)", n.cwd, pwdDir)
	}
}

func TestComposeNotification_CustomSoundDir(t *testing.T) {
	n := composeNotification(
		context.Background(),
		"notification",
		nil,
		envFromMap(map[string]string{"CLAUDE_NOTIFY_SOUND_DIR": "/custom/sounds"}),
		emptyContextRunner(),
	)
	if filepath.Dir(n.soundFile) != "/custom/sounds" {
		t.Errorf("soundFile dir = %q, want /custom/sounds", filepath.Dir(n.soundFile))
	}
}

func TestComposeTitle(t *testing.T) {
	tests := []struct {
		base, project, branch, want string
	}{
		{"Claude Code", "myrepo", "main", "Claude Code · myrepo/main"},
		{"Claude Code", "myrepo", "", "Claude Code · myrepo"},
		{"Claude Code", "", "feature-x", "Claude Code · feature-x"},
		{"Claude Code", "", "", "Claude Code"},
	}
	for _, tc := range tests {
		got := composeTitle(tc.base, tc.project, tc.branch)
		if got != tc.want {
			t.Errorf("composeTitle(%q, %q, %q) = %q, want %q", tc.base, tc.project, tc.branch, got, tc.want)
		}
	}
}

func TestGitContext_WorktreeBasename(t *testing.T) {
	cwd := t.TempDir()
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", cwd, "worktree", "list", "--porcelain"},
		[]byte("worktree /home/user/projects/awesome-tool\nHEAD abc123\nbranch refs/heads/main\n"), nil)
	r.Register("git", []string{"-C", cwd, "branch", "--show-current"},
		[]byte("main\n"), nil)
	project, branch := gitContext(context.Background(), r, cwd)
	if project != "awesome-tool" {
		t.Errorf("project = %q, want awesome-tool", project)
	}
	if branch != "main" {
		t.Errorf("branch = %q, want main", branch)
	}
}

func TestGitContext_DetachedHEADFallback(t *testing.T) {
	cwd := t.TempDir()
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", cwd, "worktree", "list", "--porcelain"},
		[]byte("worktree /home/user/proj\n"), nil)
	r.Register("git", []string{"-C", cwd, "branch", "--show-current"},
		[]byte("\n"), nil)
	r.Register("git", []string{"-C", cwd, "rev-parse", "--short", "HEAD"},
		[]byte("deadbef\n"), nil)
	_, branch := gitContext(context.Background(), r, cwd)
	if branch != "deadbef" {
		t.Errorf("branch = %q, want deadbef", branch)
	}
}

func TestGitContext_NotARepo(t *testing.T) {
	cwd := t.TempDir()
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", cwd, "worktree", "list", "--porcelain"},
		nil, errors.New("not a git repo"))
	r.Register("git", []string{"-C", cwd, "branch", "--show-current"},
		nil, errors.New("not a git repo"))
	r.Register("git", []string{"-C", cwd, "rev-parse", "--short", "HEAD"},
		nil, errors.New("not a git repo"))
	project, branch := gitContext(context.Background(), r, cwd)
	if project != "" || branch != "" {
		t.Errorf("expected empty project/branch on non-repo, got %q/%q", project, branch)
	}
}

func TestGitContext_NonexistentCwd(t *testing.T) {
	r := proc.NewFakeRunner()
	project, branch := gitContext(context.Background(), r, "/no/such/path")
	if project != "" || branch != "" {
		t.Errorf("expected empty project/branch on missing cwd, got %q/%q", project, branch)
	}
}

func TestTmuxSessionFor(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"display-message", "-p", "-t", "%5", "#{session_name}"},
		[]byte("dev\n"), nil)
	got := tmuxSessionFor(context.Background(), r, "%5")
	if got != "dev" {
		t.Errorf("session = %q, want dev", got)
	}
}

func TestTmuxSessionFor_EmptyPane(t *testing.T) {
	r := proc.NewFakeRunner()
	got := tmuxSessionFor(context.Background(), r, "")
	if got != "" {
		t.Errorf("session = %q, want empty", got)
	}
}

func TestComposeNotification_TmuxAndGitIntegrated(t *testing.T) {
	cwd := t.TempDir()
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", cwd, "worktree", "list", "--porcelain"},
		[]byte("worktree /home/user/awesome-tool\n"), nil)
	r.Register("git", []string{"-C", cwd, "branch", "--show-current"},
		[]byte("feature-x\n"), nil)
	r.Register("tmux", []string{"display-message", "-p", "-t", "%9", "#{session_name}"},
		[]byte("dev\n"), nil)

	payload := []byte(`{"message":"Custom","session_id":"abc","cwd":"` + cwd + `"}`)
	n := composeNotification(
		context.Background(),
		"stop",
		payload,
		envFromMap(map[string]string{"TMUX_PANE": "%9"}),
		r,
	)

	if n.title != "Claude Code · awesome-tool/feature-x" {
		t.Errorf("title = %q", n.title)
	}
	if n.body != "Custom" {
		t.Errorf("body = %q, want Custom", n.body)
	}
	if n.tmuxPane != "%9" || n.tmuxSession != "dev" {
		t.Errorf("tmux pane/session = %q/%q, want %%9/dev", n.tmuxPane, n.tmuxSession)
	}
	if n.sessionID != "abc" {
		t.Errorf("sessionID = %q, want abc", n.sessionID)
	}
	if !strings.HasSuffix(n.soundFile, "complete.oga") {
		t.Errorf("soundFile = %q, want complete.oga suffix", n.soundFile)
	}
}

// recordedFork captures startBackground invocations.
type recordedFork struct {
	name   string
	args   []string
	env    []string
	setsid bool
}

// withFakeFork swaps in a fake startBackground for the duration of the
// test. Captured calls accumulate in the returned slice.
func withFakeFork(t *testing.T) *[]recordedFork {
	t.Helper()
	calls := &[]recordedFork{}
	orig := startBackground
	startBackground = func(name string, args []string, env []string, setsid bool) error {
		*calls = append(*calls, recordedFork{name: name, args: args, env: env, setsid: setsid})
		return nil
	}
	t.Cleanup(func() { startBackground = orig })
	return calls
}

func makeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}

func TestForkSound_StartsWhenExecutable(t *testing.T) {
	calls := withFakeFork(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "claude-notify-sound")
	makeExecutable(t, bin)

	n := notification{soundFile: "/snd/foo.oga"}
	forkSound(n, envFromMap(map[string]string{"CLAUDE_NOTIFY_SOUND_BIN": bin}))

	if len(*calls) != 1 {
		t.Fatalf("expected 1 fork call, got %d", len(*calls))
	}
	c := (*calls)[0]
	if c.name != bin {
		t.Errorf("name = %q, want %q", c.name, bin)
	}
	if len(c.args) != 1 || c.args[0] != "/snd/foo.oga" {
		t.Errorf("args = %v, want [/snd/foo.oga]", c.args)
	}
	if c.setsid {
		t.Error("sound fork should not setsid")
	}
}

func TestForkSound_SkipsWhenBinMissing(t *testing.T) {
	calls := withFakeFork(t)
	forkSound(notification{soundFile: "/snd/foo.oga"},
		envFromMap(map[string]string{"CLAUDE_NOTIFY_SOUND_BIN": "/no/such/bin"}))
	if len(*calls) != 0 {
		t.Errorf("expected 0 fork calls when bin missing, got %d", len(*calls))
	}
}

func TestForkDispatch_PassesEnvAndSetsid(t *testing.T) {
	calls := withFakeFork(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "claude-notify-dispatch")
	makeExecutable(t, bin)

	n := notification{
		title:       "T",
		body:        "B",
		urgency:     "low",
		sessionID:   "sid-1",
		tmuxPane:    "%5",
		tmuxSession: "dev",
	}
	forkDispatch(n, envFromMap(map[string]string{"CLAUDE_NOTIFY_DISPATCH": bin}))

	if len(*calls) != 1 {
		t.Fatalf("expected 1 fork call, got %d", len(*calls))
	}
	c := (*calls)[0]
	if !c.setsid {
		t.Error("dispatch fork must setsid")
	}
	wantPairs := map[string]string{
		"CLAUDE_NOTIFY_TITLE":        "T",
		"CLAUDE_NOTIFY_BODY":         "B",
		"CLAUDE_NOTIFY_URGENCY":      "low",
		"CLAUDE_NOTIFY_SESSION_ID":   "sid-1",
		"CLAUDE_NOTIFY_TMUX_PANE":    "%5",
		"CLAUDE_NOTIFY_TMUX_SESSION": "dev",
	}
	for k, want := range wantPairs {
		found := false
		for _, e := range c.env {
			if e == k+"="+want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("env missing %s=%s; env=%v", k, want, c.env)
		}
	}
}

func TestIsExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "x")
	makeExecutable(t, exe)
	if !isExecutable(exe) {
		t.Error("executable file should report true")
	}
	noExe := filepath.Join(dir, "plain")
	if err := os.WriteFile(noExe, []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if isExecutable(noExe) {
		t.Error("non-executable file should report false")
	}
	if isExecutable("") {
		t.Error("empty path should report false")
	}
	if isExecutable("/no/such") {
		t.Error("missing path should report false")
	}
	if isExecutable(dir) {
		t.Error("directory should report false")
	}
}

// --- dialDaemon tests (D4.1 RED → D4.2 GREEN) ---

// withFakeForkDispatch replaces the package-level forkDispatchFn with a
// counter-based fake for the duration of the test. Returns the call count.
func withFakeForkDispatch(t *testing.T) *int32 {
	t.Helper()
	var calls int32
	orig := forkDispatchFn
	forkDispatchFn = func(n notification, getEnv envLookup) {
		atomic.AddInt32(&calls, 1)
	}
	t.Cleanup(func() { forkDispatchFn = orig })
	return &calls
}

// TestDialDaemon_SuccessPath verifies that when a Unix socket listener is
// ready, dialDaemon writes a properly formed JSON frame and returns nil.
// forkDispatchFn must NOT be invoked when dialDaemon succeeds.
func TestDialDaemon_SuccessPath(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	// Received frames accumulate here.
	type received struct {
		line string
		err  error
	}
	frameCh := make(chan received, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			frameCh <- received{err: err}
			return
		}
		defer conn.Close()
		scanner := bufio.NewScanner(conn)
		if scanner.Scan() {
			frameCh <- received{line: scanner.Text()}
		} else {
			frameCh <- received{err: scanner.Err()}
		}
	}()

	n := notification{
		sessionID:   "sid-test",
		title:       "Claude Code",
		body:        "Turn complete",
		urgency:     "normal",
		tmuxPane:    "%7",
		tmuxSession: "dev",
	}

	ctx := context.Background()
	err = dialDaemon(ctx, sockPath, 100*time.Millisecond, n)
	if err != nil {
		t.Fatalf("dialDaemon returned error: %v", err)
	}

	select {
	case got := <-frameCh:
		if got.err != nil {
			t.Fatalf("server read error: %v", got.err)
		}
		// Validate the JSON frame contains expected fields.
		var frame map[string]interface{}
		if err := json.Unmarshal([]byte(got.line), &frame); err != nil {
			t.Fatalf("frame is not valid JSON: %v (line=%q)", err, got.line)
		}
		if v, _ := frame["v"].(float64); v != 1 {
			t.Errorf("frame v = %v, want 1", frame["v"])
		}
		if frame["op"] != "show" {
			t.Errorf("frame op = %q, want show", frame["op"])
		}
		if frame["sid"] != "sid-test" {
			t.Errorf("frame sid = %q, want sid-test", frame["sid"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server to receive frame")
	}
}

// TestDialDaemon_ConnectionRefused verifies that when the socket path does not
// exist, dialDaemon returns a non-nil error and falls through to forkDispatch.
func TestDialDaemon_ConnectionRefused(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "nonexistent.sock")

	n := notification{
		sessionID: "sid-refuse",
		title:     "Claude Code",
		body:      "Turn complete",
		urgency:   "normal",
	}

	ctx := context.Background()
	start := time.Now()
	err := dialDaemon(ctx, sockPath, 100*time.Millisecond, n)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for missing socket, got nil")
	}
	// Should fail fast (well under timeout).
	if elapsed > 500*time.Millisecond {
		t.Errorf("refused dial took %v, expected fast failure", elapsed)
	}
}

// TestDialDaemon_Timeout verifies that when the context is already cancelled
// before dialling, dialDaemon returns an error immediately (well within 50ms).
// This exercises the context-cancellation path in the dialler.
func TestDialDaemon_Timeout(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "blocked.sock")

	// Create a listening socket so the path exists and is reachable.
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	n := notification{
		sessionID: "sid-timeout",
		title:     "Claude Code",
		body:      "Turn complete",
		urgency:   "normal",
	}

	// Pre-cancel the context — dial must fail immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err = dialDaemon(ctx, sockPath, 100*time.Millisecond, n)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	// Should return within 50ms even on a loaded system.
	if elapsed > 50*time.Millisecond {
		t.Errorf("cancelled dial took %v, want ≤50ms", elapsed)
	}
}

// TestNotifyHook_DialSuccessSkipsForkDispatch is an integration-level test
// that drives notifyHookWithDial to verify that a successful daemon dial
// prevents forkDispatch from being invoked.
func TestNotifyHook_DialSuccessSkipsForkDispatch(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "hook.sock")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	// Drain the connection so dialDaemon can write and close.
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		conn.Read(buf) //nolint:errcheck
	}()

	dispatchCalls := withFakeForkDispatch(t)

	n := notification{
		sessionID: "sid-hook",
		title:     "Claude Code",
		body:      "Turn complete",
		urgency:   "normal",
	}

	notifyHookWithDial(context.Background(), n, sockPath, 100*time.Millisecond, os.Getenv)

	if atomic.LoadInt32(dispatchCalls) != 0 {
		t.Errorf("forkDispatchFn called %d times, want 0 (dial succeeded)", *dispatchCalls)
	}
}

// TestNotifyHook_DialFailureFallsBackToDispatch verifies that when the socket
// is absent, notifyHookWithDial falls through to forkDispatch.
func TestNotifyHook_DialFailureFallsBackToDispatch(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "absent.sock") // does not exist

	dispatchCalls := withFakeForkDispatch(t)

	// Provide a real-looking (but fake) dispatch binary so forkDispatch passes
	// isExecutable. We override forkDispatchFn, so the binary is never exec'd.
	n := notification{
		sessionID: "sid-fallback",
		title:     "Claude Code",
		body:      "Turn complete",
		urgency:   "normal",
	}

	notifyHookWithDial(context.Background(), n, sockPath, 100*time.Millisecond, os.Getenv)

	if atomic.LoadInt32(dispatchCalls) != 1 {
		t.Errorf("forkDispatchFn called %d times, want 1 (dial failed → fallback)", *dispatchCalls)
	}
}
