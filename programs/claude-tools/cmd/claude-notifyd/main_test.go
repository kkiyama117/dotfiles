package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"claude-tools/internal/notifyd"
)

// TestResolveListenAddr_XDGSet verifies that the listen path is
// ${XDG_RUNTIME_DIR}/claude-notify/sock when XDG_RUNTIME_DIR is set.
func TestResolveListenAddr_XDGSet(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	got := resolveListenAddr("")
	want := filepath.Join(dir, "claude-notify", "sock")
	if got != want {
		t.Errorf("resolveListenAddr: got %q, want %q", got, want)
	}
}

// TestResolveListenAddr_FlagOverride verifies that an explicit --listen flag
// takes precedence over XDG_RUNTIME_DIR.
func TestResolveListenAddr_FlagOverride(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/some/dir")

	got := resolveListenAddr("/tmp/x.sock")
	want := "/tmp/x.sock"
	if got != want {
		t.Errorf("resolveListenAddr: got %q, want %q", got, want)
	}
}

// TestResolveListenAddr_NoXDG verifies the fallback path when XDG_RUNTIME_DIR
// is unset: /tmp/claude-notify-<uid>/sock.
func TestResolveListenAddr_NoXDG(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")

	got := resolveListenAddr("")
	want := fmt.Sprintf("/tmp/claude-notify-%d/sock", os.Getuid())
	if got != want {
		t.Errorf("resolveListenAddr: got %q, want %q", got, want)
	}
}

// TestSdNotifyReady verifies that sdNotifyReady writes "READY=1\n" to the
// socket path stored in NOTIFY_SOCKET.
func TestSdNotifyReady(t *testing.T) {
	// Listen on a temporary unixgram socket to receive the notification.
	sockPath := filepath.Join(t.TempDir(), "notify.sock")
	conn, err := net.ListenPacket("unixgram", sockPath)
	if err != nil {
		t.Fatalf("listen unixgram: %v", err)
	}
	defer conn.Close()

	t.Setenv("NOTIFY_SOCKET", sockPath)

	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 128)
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			done <- ""
			return
		}
		done <- string(buf[:n])
	}()

	sdNotifyReady()

	select {
	case msg := <-done:
		if msg != "READY=1\n" {
			t.Errorf("sdNotifyReady: got %q, want %q", msg, "READY=1\n")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("sdNotifyReady: no message received within 200ms")
	}
}

// TestSdNotifyReady_NoSocket verifies that sdNotifyReady is a no-op when
// NOTIFY_SOCKET is not set.
func TestSdNotifyReady_NoSocket(t *testing.T) {
	t.Setenv("NOTIFY_SOCKET", "")
	// Must not panic or block.
	sdNotifyReady()
}

// TestAcquireLock_Success verifies that the first acquire succeeds.
func TestAcquireLock_Success(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")
	f, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("first acquireLock failed: %v", err)
	}
	if f == nil {
		t.Fatal("acquireLock returned nil file on success")
	}
	defer f.Close()
}

// TestFlock_DoubleStart verifies that a second acquireLock on the same path
// returns an error while the first lock is held, and succeeds after release.
func TestFlock_DoubleStart(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "double.lock")

	f1, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("first acquireLock failed: %v", err)
	}

	// Second attempt must fail while f1 holds the lock.
	_, err = acquireLock(lockPath)
	if err == nil {
		t.Fatal("second acquireLock should have failed but succeeded")
	}

	// Release the first lock.
	_ = f1.Close()

	// Third attempt must succeed after release.
	f3, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("third acquireLock failed after release: %v", err)
	}
	defer f3.Close()
}

// TestResolveListener_UnixSocket verifies that resolveListener creates a
// Unix socket at the given path when LISTEN_FDS is unset.
func TestResolveListener_UnixSocket(t *testing.T) {
	t.Setenv("LISTEN_FDS", "")
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	ln, err := resolveListener(sockPath)
	if err != nil {
		t.Fatalf("resolveListener: %v", err)
	}
	defer ln.Close()

	if ln.Addr().Network() != "unix" {
		t.Errorf("expected unix network, got %q", ln.Addr().Network())
	}
	if ln.Addr().String() != sockPath {
		t.Errorf("expected addr %q, got %q", sockPath, ln.Addr().String())
	}

	// Verify we can actually connect to the socket.
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.Close()
}

// TestResolveListener_MkdirParent verifies that resolveListener creates the
// parent directory when it does not exist.
func TestResolveListener_MkdirParent(t *testing.T) {
	t.Setenv("LISTEN_FDS", "")
	base := t.TempDir()
	sockPath := filepath.Join(base, "subdir", "nested", "test.sock")

	ln, err := resolveListener(sockPath)
	if err != nil {
		t.Fatalf("resolveListener with nested path: %v", err)
	}
	defer ln.Close()
}

// TestSdNotifyReady_DialError verifies that sdNotifyReady handles a bad
// socket path gracefully without panicking.
func TestSdNotifyReady_DialError(t *testing.T) {
	// Point NOTIFY_SOCKET at a path that cannot be dialled as unixgram.
	t.Setenv("NOTIFY_SOCKET", "/nonexistent/path/notify.sock")
	// Must not panic.
	sdNotifyReady()
}

// TestRun_DoubleStart verifies that run() exits gracefully (nil error) when
// the lock is already held by another caller.
func TestRun_DoubleStart(t *testing.T) {
	// Set up a temp state dir so notify.StateDir points there.
	stateDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", stateDir)

	// Manually hold the lock so run() sees a busy lock.
	lockPath := filepath.Join(stateDir, "claude-notify", "sessions", "notifyd.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	holder, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("pre-acquire lock: %v", err)
	}
	defer holder.Close()

	// Inject a fake dialBus so we don't need a real D-Bus.
	orig := dialBus
	dialBus = func(_ context.Context) (notifyd.Bus, error) {
		return notifyd.NewFakeBus(), nil
	}
	defer func() { dialBus = orig }()

	// run() should return nil (double-start exit) before reaching D-Bus dial.
	err = run(filepath.Join(stateDir, "claude-notify", "sock"))
	if err != nil {
		t.Errorf("run() with busy lock should return nil, got: %v", err)
	}
}

// TestRun_ListenerError verifies that run() returns an error when the
// listener cannot be created (e.g. path in a non-existent read-only dir).
func TestRun_ListenerError(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", stateDir)
	t.Setenv("LISTEN_FDS", "")

	// Inject a fake dialBus (not reached in this path, but restore cleanly).
	orig := dialBus
	dialBus = func(_ context.Context) (notifyd.Bus, error) {
		return notifyd.NewFakeBus(), nil
	}
	defer func() { dialBus = orig }()

	// Use a socket path whose parent is a file (not a directory) so MkdirAll
	// and net.Listen both fail.
	blocker := filepath.Join(stateDir, "blocking-file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("create blocker: %v", err)
	}
	badSock := filepath.Join(blocker, "sub", "sock")

	err := run(badSock)
	if err == nil {
		t.Error("run() with bad listener path should return an error")
	}
}

// TestRun_DBusError verifies that run() returns an error when D-Bus dial
// fails (e.g. no session bus available in CI).
func TestRun_DBusError(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", stateDir)
	t.Setenv("LISTEN_FDS", "")

	sockPath := filepath.Join(stateDir, "claude-notify", "sock")

	orig := dialBus
	dialBus = func(_ context.Context) (notifyd.Bus, error) {
		return nil, fmt.Errorf("no session bus available")
	}
	defer func() { dialBus = orig }()

	err := run(sockPath)
	if err == nil {
		t.Error("run() should return error when D-Bus dial fails")
	}
}

// TestResolveListener_LISTEN_FDS verifies that when LISTEN_FDS=1 is set,
// resolveListener attempts to use fd 3. Since we cannot easily pass a real
// pre-bound fd in tests, we just verify that it attempts the FileListener
// path (the error message indicates fd 3 was used, not net.Listen).
func TestResolveListener_LISTEN_FDS(t *testing.T) {
	t.Setenv("LISTEN_FDS", "1")
	// fd 3 in the test process is likely stdin/stdout/stderr+1, which may or
	// may not be a valid socket. We just verify the code path is taken.
	_, err := resolveListener("/unused/path")
	// Either succeeds (if fd 3 is a socket) or returns a FileListener error.
	// The important thing is no panic and no "listen unix /unused/path" error.
	if err != nil {
		if contains(err.Error(), "/unused/path") {
			t.Errorf("LISTEN_FDS set but fell through to net.Listen path: %v", err)
		}
		// FileListener error is expected in test environment — that's fine.
	}
	t.Setenv("LISTEN_FDS", "")
}

// contains is a local helper for TestResolveListener_LISTEN_FDS.
func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
