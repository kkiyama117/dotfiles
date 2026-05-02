package notify

import (
	"context"
	"errors"
	"testing"

	"claude-tools/internal/proc"
)

// helpers shared across focus tests

func envFromMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func denyLookPath(_ string) (string, error) {
	return "", errors.New("not in PATH")
}

func allowLookPath(_ string) (string, error) {
	return "/usr/bin/whatever", nil
}

func TestFocusTmux_HappyPath(t *testing.T) {
	fake := proc.NewFakeRunner()
	popup := PopupContext{TmuxPane: "%5", TmuxSession: "dev", SessionID: "sess-1"}
	fake.Register("tmux", []string{"has-session", "-t", "dev"}, []byte(""), nil)
	fake.Register("tmux",
		[]string{"switch-client", "-t", "dev", ";", "select-pane", "-t", "%5"},
		[]byte(""), nil)
	FocusTmux(context.Background(), fake, popup)
}

func TestFocusTmux_MissingContext(t *testing.T) {
	fake := proc.NewFakeRunner()
	FocusTmux(context.Background(), fake, PopupContext{TmuxPane: "", TmuxSession: ""})
}

func TestFocusTmux_SessionGone(t *testing.T) {
	fake := proc.NewFakeRunner()
	popup := PopupContext{TmuxPane: "%5", TmuxSession: "dev", SessionID: "sess-gone"}
	fake.Register("tmux", []string{"has-session", "-t", "dev"},
		nil, errors.New("session not found"))
	FocusTmux(context.Background(), fake, popup)
}

func TestFocusWM_X11Path(t *testing.T) {
	fake := proc.NewFakeRunner()
	fake.Register("xdotool", []string{"search", "--class", "kitty", "windowactivate"},
		[]byte(""), nil)
	getEnv := envFromMap(map[string]string{"DISPLAY": ":0"})
	FocusWM(context.Background(), fake, allowLookPath, getEnv)
}

func TestFocusWM_WaylandPath(t *testing.T) {
	fake := proc.NewFakeRunner()
	wantCmd := `[app_id="kitty"] focus, [app_id="com.mitchellh.ghostty"] focus`
	fake.Register("swaymsg", []string{"-t", "command", wantCmd},
		[]byte(""), nil)
	getEnv := envFromMap(map[string]string{"WAYLAND_DISPLAY": "wayland-0"})
	FocusWM(context.Background(), fake, allowLookPath, getEnv)
}

func TestFocusWM_NoTool(t *testing.T) {
	fake := proc.NewFakeRunner()
	getEnv := envFromMap(map[string]string{"DISPLAY": ":0"})
	FocusWM(context.Background(), fake, denyLookPath, getEnv)
}

func TestFocusWM_NoDisplay(t *testing.T) {
	fake := proc.NewFakeRunner()
	getEnv := envFromMap(map[string]string{})
	FocusWM(context.Background(), fake, allowLookPath, getEnv)
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
	CloseNotification(context.Background(), fake, 42)
}

func TestCloseNotification_ZeroID(t *testing.T) {
	fake := proc.NewFakeRunner()
	CloseNotification(context.Background(), fake, 0)
}

// TestTerminalClassesOrder asserts the exported TerminalClasses var has the
// correct priority order (load-bearing for xdotool iteration).
func TestTerminalClassesOrder(t *testing.T) {
	want := []string{"kitty", "ghostty", "wezterm", "Alacritty"}
	if len(TerminalClasses) != len(want) {
		t.Fatalf("TerminalClasses len = %d, want %d", len(TerminalClasses), len(want))
	}
	for i, c := range want {
		if TerminalClasses[i] != c {
			t.Errorf("TerminalClasses[%d] = %q, want %q", i, TerminalClasses[i], c)
		}
	}
}
