package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"claude-tools/internal/notify"
	"claude-tools/internal/proc"
)

func envFromMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func denyLookPath(_ string) (string, error) {
	return "", errors.New("not in PATH")
}

func allowLookPath(_ string) (string, error) {
	return "/usr/bin/whatever", nil
}

func TestParsePopupOutput(t *testing.T) {
	tests := []struct {
		name       string
		out        []byte
		wantID     uint32
		wantAction string
	}{
		{"empty", nil, 0, ""},
		{"id only", []byte("42\n"), 42, ""},
		{"id+action", []byte("42\ndefault\n"), 42, "default"},
		{"trailing whitespace", []byte("  42  \n  default  "), 42, "default"},
		{"unknown action", []byte("99\nclose\n"), 99, "close"},
		{"non-numeric id", []byte("abc\ndefault\n"), 0, "default"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, action := parsePopupOutput(tc.out)
			if id != tc.wantID || action != tc.wantAction {
				t.Errorf("parsePopupOutput(%q) = (%d, %q), want (%d, %q)",
					tc.out, id, action, tc.wantID, tc.wantAction)
			}
		})
	}
}

func TestShowPopup_BuildsArgs(t *testing.T) {
	fake := proc.NewFakeRunner()
	cfg := popupConfig{
		title:     "Title",
		body:      "Body",
		urgency:   "low",
		sessionID: "sess-1",
	}
	wantArgs := []string{
		"--app-name=ClaudeCode",
		"--urgency=low",
		"--expire-time=0",
		"--action=default=Focus",
		"--hint=string:x-claude-session:sess-1",
		"--print-id",
		"--wait",
		"--replace-id=7",
		"--",
		"Title",
		"Body",
	}
	fake.Register("notify-send", wantArgs, []byte("42\ndefault\n"), nil)
	id, action := showPopup(context.Background(), fake, cfg, 7)
	if id != 42 || action != "default" {
		t.Errorf("showPopup = (%d, %q), want (42, default)", id, action)
	}
}

func TestShowPopup_WithoutPrevID(t *testing.T) {
	fake := proc.NewFakeRunner()
	cfg := popupConfig{title: "T", body: "B", urgency: "normal", sessionID: ""}
	wantArgs := []string{
		"--app-name=ClaudeCode",
		"--urgency=normal",
		"--expire-time=0",
		"--action=default=Focus",
		"--hint=string:x-claude-session:unknown",
		"--print-id",
		"--wait",
		"--", "T", "B",
	}
	fake.Register("notify-send", wantArgs, []byte("12\n"), nil)
	id, action := showPopup(context.Background(), fake, cfg, 0)
	if id != 12 || action != "" {
		t.Errorf("showPopup = (%d, %q), want (12, '')", id, action)
	}
}

func TestShowPopup_NotifySendFailure(t *testing.T) {
	fake := proc.NewFakeRunner()
	cfg := popupConfig{title: "T", body: "B", urgency: "normal"}
	id, action := showPopup(context.Background(), fake, cfg, 0)
	if id != 0 || action != "" {
		t.Errorf("on error showPopup = (%d, %q), want (0, '')", id, action)
	}
}

func TestSessionHint(t *testing.T) {
	if sessionHint("") != "unknown" {
		t.Errorf("empty sid hint = %q, want unknown", sessionHint(""))
	}
	if sessionHint("abc") != "abc" {
		t.Errorf("non-empty sid passes through: got %q", sessionHint("abc"))
	}
}

func TestFocusTmux_HappyPath(t *testing.T) {
	fake := proc.NewFakeRunner()
	cfg := popupConfig{tmuxPane: "%5", tmuxSession: "dev"}
	fake.Register("tmux", []string{"has-session", "-t", "dev"}, []byte(""), nil)
	fake.Register("tmux",
		[]string{"switch-client", "-t", "dev", ";", "select-pane", "-t", "%5"},
		[]byte(""), nil)
	focusTmux(context.Background(), fake, cfg)
}

func TestFocusTmux_MissingContext(t *testing.T) {
	fake := proc.NewFakeRunner()
	focusTmux(context.Background(), fake, popupConfig{tmuxPane: "", tmuxSession: ""})
}

func TestFocusTmux_SessionGone(t *testing.T) {
	fake := proc.NewFakeRunner()
	cfg := popupConfig{tmuxPane: "%5", tmuxSession: "dev"}
	fake.Register("tmux", []string{"has-session", "-t", "dev"},
		nil, errors.New("session not found"))
	focusTmux(context.Background(), fake, cfg)
}

func TestFocusWM_X11Path(t *testing.T) {
	fake := proc.NewFakeRunner()
	fake.Register("xdotool", []string{"search", "--class", "kitty", "windowactivate"},
		[]byte(""), nil)
	getEnv := envFromMap(map[string]string{"DISPLAY": ":0"})
	focusWM(context.Background(), fake, allowLookPath, getEnv)
}

func TestFocusWM_WaylandPath(t *testing.T) {
	fake := proc.NewFakeRunner()
	wantCmd := `[app_id="kitty"] focus, [app_id="com.mitchellh.ghostty"] focus`
	fake.Register("swaymsg", []string{"-t", "command", wantCmd},
		[]byte(""), nil)
	getEnv := envFromMap(map[string]string{"WAYLAND_DISPLAY": "wayland-0"})
	focusWM(context.Background(), fake, allowLookPath, getEnv)
}

func TestFocusWM_NoTool(t *testing.T) {
	fake := proc.NewFakeRunner()
	getEnv := envFromMap(map[string]string{"DISPLAY": ":0"})
	focusWM(context.Background(), fake, denyLookPath, getEnv)
}

func TestFocusWM_NoDisplay(t *testing.T) {
	fake := proc.NewFakeRunner()
	getEnv := envFromMap(map[string]string{})
	focusWM(context.Background(), fake, allowLookPath, getEnv)
}

func TestCloseNotification(t *testing.T) {
	fake := proc.NewFakeRunner()
	fake.Register("gdbus", []string{
		"call", "--session",
		"--dest=org.freedesktop.Notifications",
		"--object-path=/org/freedesktop/Notifications",
		"--method=org.freedesktop.Notifications.CloseNotification",
		"42",
	}, []byte("()"), nil)
	closeNotification(context.Background(), fake, 42)
}

func TestCloseNotification_ZeroID(t *testing.T) {
	fake := proc.NewFakeRunner()
	closeNotification(context.Background(), fake, 0)
}

// TestDispatch_HappyPath_DefaultAction verifies the full state machine:
// load prev id (none) → notify-send → save new id → tmux focus → WM focus → close.
func TestDispatch_HappyPath_DefaultAction(t *testing.T) {
	stateDir := t.TempDir()
	cfg := popupConfig{
		title:       "T",
		body:        "B",
		urgency:     "normal",
		sessionID:   "sess-happy",
		tmuxPane:    "%5",
		tmuxSession: "dev",
	}

	fake := proc.NewFakeRunner()
	notifyArgs := []string{
		"--app-name=ClaudeCode",
		"--urgency=normal",
		"--expire-time=0",
		"--action=default=Focus",
		"--hint=string:x-claude-session:sess-happy",
		"--print-id",
		"--wait",
		"--", "T", "B",
	}
	fake.Register("notify-send", notifyArgs, []byte("100\ndefault\n"), nil)
	fake.Register("tmux", []string{"has-session", "-t", "dev"}, []byte(""), nil)
	fake.Register("tmux",
		[]string{"switch-client", "-t", "dev", ";", "select-pane", "-t", "%5"},
		[]byte(""), nil)
	fake.Register("xdotool", []string{"search", "--class", "kitty", "windowactivate"},
		[]byte(""), nil)
	fake.Register("gdbus", []string{
		"call", "--session",
		"--dest=org.freedesktop.Notifications",
		"--object-path=/org/freedesktop/Notifications",
		"--method=org.freedesktop.Notifications.CloseNotification",
		"100",
	}, []byte("()"), nil)

	getEnv := envFromMap(map[string]string{"DISPLAY": ":0"})
	dispatch(context.Background(), cfg, stateDir, fake, allowLookPath, getEnv)

	if got := notify.LoadReplaceID(stateDir, cfg.sessionID); got != 100 {
		t.Errorf("state file holds %d, want 100", got)
	}
}

// TestDispatch_NonDefaultAction verifies that right-click / close / timeout
// (no action key) skips focus + close calls but still saves the id.
func TestDispatch_NonDefaultAction(t *testing.T) {
	stateDir := t.TempDir()
	cfg := popupConfig{
		title:     "T",
		body:      "B",
		urgency:   "normal",
		sessionID: "sess-rclick",
	}
	fake := proc.NewFakeRunner()
	notifyArgs := []string{
		"--app-name=ClaudeCode",
		"--urgency=normal",
		"--expire-time=0",
		"--action=default=Focus",
		"--hint=string:x-claude-session:sess-rclick",
		"--print-id",
		"--wait",
		"--", "T", "B",
	}
	fake.Register("notify-send", notifyArgs, []byte("55\n"), nil)

	dispatch(context.Background(), cfg, stateDir, fake, denyLookPath, envFromMap(nil))

	if got := notify.LoadReplaceID(stateDir, cfg.sessionID); got != 55 {
		t.Errorf("state file holds %d, want 55", got)
	}
}

// TestDispatch_UsesPrevID verifies a previously-saved id is replayed
// as --replace-id on the next dispatch run.
func TestDispatch_UsesPrevID(t *testing.T) {
	stateDir := t.TempDir()
	sid := "sess-replace"
	if err := notify.SaveReplaceID(stateDir, sid, 12345); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	cfg := popupConfig{
		title:     "T",
		body:      "B",
		urgency:   "normal",
		sessionID: sid,
	}
	fake := proc.NewFakeRunner()
	wantArgs := []string{
		"--app-name=ClaudeCode",
		"--urgency=normal",
		"--expire-time=0",
		"--action=default=Focus",
		"--hint=string:x-claude-session:" + sid,
		"--print-id",
		"--wait",
		"--replace-id=12345",
		"--", "T", "B",
	}
	fake.Register("notify-send", wantArgs, []byte("12345\n"), nil)
	dispatch(context.Background(), cfg, stateDir, fake, denyLookPath, envFromMap(nil))
}

func TestDispatch_NotifySendFailureDoesNotPanic(t *testing.T) {
	stateDir := t.TempDir()
	cfg := popupConfig{title: "T", body: "B", urgency: "normal", sessionID: "sess-x"}
	fake := proc.NewFakeRunner()
	dispatch(context.Background(), cfg, stateDir, fake, denyLookPath, envFromMap(nil))
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("FOO", "bar")
	if envOrDefault("FOO", "fallback") != "bar" {
		t.Error("set env not honoured")
	}
	t.Setenv("EMPTY", "")
	if envOrDefault("EMPTY", "fallback") != "fallback" {
		t.Error("empty env should yield fallback")
	}
	if envOrDefault("UNSET_XXXX_KEY", "fallback") != "fallback" {
		t.Error("unset env should yield fallback")
	}
}

// Self-check: terminal class precedence is load-bearing.
func TestTerminalClassesOrder(t *testing.T) {
	want := []string{"kitty", "ghostty", "wezterm", "Alacritty"}
	if len(terminalClasses) != len(want) {
		t.Fatalf("classes len = %d, want %d", len(terminalClasses), len(want))
	}
	for i, c := range want {
		if terminalClasses[i] != c {
			t.Errorf("classes[%d] = %q, want %q", i, terminalClasses[i], c)
		}
	}
}

// Self-check: x-claude-session hint must include the literal "x-" prefix.
func TestXClaudeHintFormat(t *testing.T) {
	args := []string{
		"--app-name=ClaudeCode",
		"--urgency=normal",
		"--expire-time=0",
		"--action=default=Focus",
		"--hint=string:x-claude-session:foo",
		"--print-id",
		"--wait",
		"--", "T", "B",
	}
	cfg := popupConfig{title: "T", body: "B", urgency: "normal", sessionID: "foo"}
	fake := proc.NewFakeRunner()
	fake.Register("notify-send", args, []byte("1\n"), nil)
	if id, _ := showPopup(context.Background(), fake, cfg, 0); id != 1 {
		t.Errorf("hint format mismatch caused FakeRunner reject (id=%d)", id)
	}
	for _, a := range args {
		if strings.HasPrefix(a, "--hint=string:x-claude-session:") {
			return
		}
	}
	t.Errorf("expected x-claude-session hint in args: %v", args)
}
