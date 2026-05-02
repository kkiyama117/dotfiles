package notify

import (
	"context"
	"fmt"

	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
)

var focusLogger = obslog.New("claude-notify-focus")

// PopupContext is the per-popup info needed by focus helpers.
type PopupContext struct {
	SessionID   string
	TmuxPane    string
	TmuxSession string
}

// TerminalClasses are X11 WM_CLASS / Wayland app_id values in priority order.
// xdotool / swaymsg search each in turn until one activates.
var TerminalClasses = []string{"kitty", "ghostty", "wezterm", "Alacritty"}

// FocusTmux runs `tmux switch-client -t <session> ; select-pane -t <pane>`
// when both context vars are present and the session exists.
// Errors are logged and swallowed (popup-shouldn't-block-claude rule).
func FocusTmux(ctx context.Context, runner proc.Runner, popup PopupContext) {
	if popup.TmuxPane == "" || popup.TmuxSession == "" {
		focusLogger.Info("focus skipped: no tmux context",
			"pane", popup.TmuxPane, "session", popup.TmuxSession, "sid", popup.SessionID)
		return
	}
	if _, err := runner.Run(ctx, "tmux", "has-session", "-t", popup.TmuxSession); err != nil {
		focusLogger.Info("focus skipped: tmux session gone",
			"session", popup.TmuxSession, "sid", popup.SessionID)
		return
	}
	if _, err := runner.Run(ctx, "tmux",
		"switch-client", "-t", popup.TmuxSession,
		";", "select-pane", "-t", popup.TmuxPane,
	); err != nil {
		focusLogger.Warn("tmux focus failed", "err", err, "sid", popup.SessionID)
		return
	}
	focusLogger.Info("focus tmux ok",
		"session", popup.TmuxSession, "pane", popup.TmuxPane, "sid", popup.SessionID)
}

// FocusWM raises the originating terminal window via xdotool (X11) or
// swaymsg (Wayland). No-op when neither is available, neither display
// env var is set, or when the search returns no window.
// Errors are logged and swallowed.
func FocusWM(
	ctx context.Context,
	runner proc.Runner,
	lookPath func(string) (string, error),
	getEnv func(string) string,
) {
	if getEnv("DISPLAY") != "" {
		if _, err := lookPath("xdotool"); err == nil {
			for _, cls := range TerminalClasses {
				if _, err := runner.Run(ctx, "xdotool", "search", "--class", cls, "windowactivate"); err == nil {
					focusLogger.Info("focus wm: xdotool", "class", cls)
					return
				}
			}
		}
	}
	if getEnv("WAYLAND_DISPLAY") != "" {
		if _, err := lookPath("swaymsg"); err == nil {
			cmd := `[app_id="kitty"] focus, [app_id="com.mitchellh.ghostty"] focus`
			if _, err := runner.Run(ctx, "swaymsg", "-t", "command", cmd); err == nil {
				focusLogger.Info("focus wm: swaymsg")
				return
			}
		}
	}
	focusLogger.Info("focus wm: no tool available")
}

// CloseNotification asks the notification daemon to dismiss popup id.
// Best-effort: errors are logged and swallowed.
func CloseNotification(ctx context.Context, runner proc.Runner, id uint32) {
	if id == 0 {
		return
	}
	if _, err := runner.Run(ctx, "gdbus", "call", "--session",
		"--dest=org.freedesktop.Notifications",
		"--object-path=/org/freedesktop/Notifications",
		"--method=org.freedesktop.Notifications.CloseNotification",
		fmt.Sprintf("%d", id),
	); err != nil {
		focusLogger.Warn("CloseNotification failed", "err", err, "id", id)
	}
}
