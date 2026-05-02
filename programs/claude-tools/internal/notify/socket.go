package notify

import "claude-tools/internal/xdg"

// SocketPath returns the Unix socket path used by claude-notifyd.
// It is the sibling of StateDir: ${XDG_RUNTIME_DIR}/claude-notify/sock
// (or /tmp/claude-notify/sock when XDG_RUNTIME_DIR is unset).
func SocketPath() string {
	return xdg.RuntimeDir() + "/claude-notify/sock"
}
