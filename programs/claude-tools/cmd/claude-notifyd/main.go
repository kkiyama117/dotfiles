// Command claude-notifyd is the Unix-socket daemon that receives OpShow frames
// from claude-notify-hook clients and forwards them to the D-Bus notification
// service. It also reacts to D-Bus ActionInvoked / NotificationClosed signals
// to focus the originating tmux pane/terminal window.
//
// Lifecycle:
//  1. Parse flags / resolve listen path.
//  2. Acquire exclusive flock on <stateDir>/notifyd.lock (double-start guard).
//  3. Dial D-Bus session bus.
//  4. Start notifyd.Server (accept loop + bus dispatcher).
//  5. Send sd_notify READY=1 if NOTIFY_SOCKET is set.
//  6. Wait for SIGTERM / SIGINT; cancel ctx; wait for Serve to return.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"claude-tools/internal/notify"
	"claude-tools/internal/notifyd"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
)

var log = obslog.New("claude-notifyd")

// dialBus is the production D-Bus dial function. Tests may substitute a fake.
var dialBus = func(ctx context.Context) (notifyd.Bus, error) {
	return notifyd.DialSession(ctx)
}

func main() {
	var listenFlag string
	flag.StringVar(&listenFlag, "listen", "", "Unix socket path to listen on")
	flag.Parse()

	if err := run(resolveListenAddr(listenFlag)); err != nil {
		log.Error("fatal error", "err", err)
		os.Exit(1)
	}
}

// run is the testable entry point. It owns the full daemon lifecycle.
// exitCode 0 = normal exit (e.g. lock already held by another instance).
func run(sockPath string) error {
	stateDir := notify.StateDir()
	lockFile, err := acquireLock(filepath.Join(stateDir, "notifyd.lock"))
	if err != nil {
		log.Info("another instance is running, exiting", "err", err)
		return nil // double-start: graceful exit, not an error
	}
	defer lockFile.Close()

	ln, err := resolveListener(sockPath)
	if err != nil {
		return fmt.Errorf("open listener: %w", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus, err := dialBus(ctx)
	if err != nil {
		return fmt.Errorf("D-Bus dial: %w", err)
	}
	defer bus.Close()

	st, err := notifyd.NewState(stateDir)
	if err != nil {
		return fmt.Errorf("initialise state: %w", err)
	}

	srv := notifyd.NewServer(notifyd.ServerOptions{
		Listener: ln,
		Bus:      bus,
		State:    st,
		Runner:   proc.RealRunner{},
		LookPath: exec.LookPath,
		GetEnv:   os.Getenv,
	})

	srvDone := make(chan error, 1)
	go func() {
		srvDone <- srv.Serve(ctx)
	}()

	sdNotifyReady()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	select {
	case sig := <-sigCh:
		log.Info("received signal, shutting down", "signal", sig)
	case err := <-srvDone:
		return err
	}

	cancel()

	select {
	case <-srvDone:
	case <-time.After(5 * time.Second):
		log.Warn("shutdown timed out, forcing exit")
	}
	return nil
}

// resolveListenAddr returns the Unix socket path that the daemon should bind.
// Priority:
//  1. flagPath if non-empty (explicit --listen flag).
//  2. ${XDG_RUNTIME_DIR}/claude-notify/sock if XDG_RUNTIME_DIR is set.
//  3. /tmp/claude-notify-<uid>/sock as fallback.
func resolveListenAddr(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "claude-notify", "sock")
	}
	return fmt.Sprintf("/tmp/claude-notify-%d/sock", os.Getuid())
}

// resolveListener returns a net.Listener using either LISTEN_FDS socket
// activation (fd 3) or a freshly-bound Unix socket at sockPath.
func resolveListener(sockPath string) (net.Listener, error) {
	if os.Getenv("LISTEN_FDS") != "" {
		// systemd socket activation: fd 3 is the pre-bound socket.
		f := os.NewFile(3, "listen-fd")
		ln, err := net.FileListener(f)
		if err != nil {
			return nil, fmt.Errorf("FileListener from fd 3: %w", err)
		}
		return ln, nil
	}

	// Create parent directory with restricted permissions.
	dir := filepath.Dir(sockPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("listen unix %s: %w", sockPath, err)
	}
	return ln, nil
}

// acquireLock opens (creating if necessary) the file at path and acquires an
// exclusive non-blocking flock. Returns the open file on success; the caller
// must close it to release the lock. Returns an error if the lock is already
// held (double-start protection).
func acquireLock(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir for lock: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("flock %s: %w", path, err)
	}
	return f, nil
}

// sdNotifyReady sends "READY=1\n" to the socket at NOTIFY_SOCKET if set.
// Uses stdlib net.Dial("unixgram") — no external dependency required.
// Best-effort: errors are logged and swallowed.
func sdNotifyReady() {
	sock := os.Getenv("NOTIFY_SOCKET")
	if sock == "" {
		return
	}
	conn, err := net.Dial("unixgram", sock)
	if err != nil {
		log.Warn("sd_notify: dial failed", "err", err)
		return
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("READY=1\n")); err != nil {
		log.Warn("sd_notify: write failed", "err", err)
	}
}
