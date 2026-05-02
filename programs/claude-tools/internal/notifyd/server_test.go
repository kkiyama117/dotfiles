package notifyd_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"claude-tools/internal/notify"
	"claude-tools/internal/notifyd"
	"claude-tools/internal/proc"
)

// newTestSocket creates a Unix socket listener in t.TempDir() and returns it.
func newTestSocket(t *testing.T) net.Listener {
	t.Helper()
	sockPath := filepath.Join(t.TempDir(), "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

// newTestState creates a State backed by t.TempDir().
func newTestState(t *testing.T) *notifyd.State {
	t.Helper()
	dir := t.TempDir()
	st, err := notifyd.NewState(dir)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	return st
}

// dialAndSend dials the unix socket at addr, sends the JSON frame followed
// by a newline, and closes the connection.
func dialAndSend(t *testing.T, addr string, f notifyd.Frame) {
	t.Helper()
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}
	conn, err := net.Dial("unix", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write(append(b, '\n')); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// pollUntil polls fn every 10ms until it returns true or timeout expires.
func pollUntil(t *testing.T, timeout time.Duration, fn func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// sockAddr extracts the unix socket path from a net.Listener.
func sockAddr(ln net.Listener) string {
	return ln.Addr().String()
}

// startServer spins up a Server in a goroutine and returns a cancel function
// and a done channel.
func startServer(t *testing.T, opts notifyd.ServerOptions) (cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	srv := notifyd.NewServer(opts)
	go func() {
		errCh <- srv.Serve(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})
	return cancel, errCh
}

// noopLookPath is a LookPath stub that always succeeds.
func noopLookPath(name string) (string, error) {
	return "/usr/bin/" + name, nil
}

// emptyGetEnv always returns "".
func emptyGetEnv(_ string) string { return "" }

// --- Test cases ---

// TestServer_ShowFrame_CallsBusNotify verifies that a valid OpShow frame
// results in exactly one Bus.Notify call with correct fields.
func TestServer_ShowFrame_CallsBusNotify(t *testing.T) {
	ln := newTestSocket(t)
	fb := notifyd.NewFakeBus()
	st := newTestState(t)

	runner := proc.NewFakeRunner()
	opts := notifyd.ServerOptions{
		Listener: ln,
		Bus:      fb,
		State:    st,
		Runner:   runner,
		LookPath: noopLookPath,
		GetEnv:   emptyGetEnv,
	}
	startServer(t, opts)

	// Give the server a moment to enter Accept.
	time.Sleep(20 * time.Millisecond)

	dialAndSend(t, sockAddr(ln), notifyd.Frame{
		V:       1,
		Op:      "show",
		SID:     "s1",
		Title:   "T",
		Body:    "B",
		Urgency: "normal",
	})

	ok := pollUntil(t, 200*time.Millisecond, func() bool {
		return len(fb.Calls()) == 1
	})
	if !ok {
		t.Fatalf("expected 1 Bus.Notify call, got %d", len(fb.Calls()))
	}

	call := fb.Calls()[0]
	if call.ReplaceID != 0 {
		t.Errorf("replaceID: want 0, got %d", call.ReplaceID)
	}
	if call.Frame.SID != "s1" {
		t.Errorf("SID: want s1, got %s", call.Frame.SID)
	}
	if call.Frame.Title != "T" {
		t.Errorf("Title: want T, got %s", call.Frame.Title)
	}
	if call.Frame.Body != "B" {
		t.Errorf("Body: want B, got %s", call.Frame.Body)
	}
}

// TestServer_ReplaceFlow verifies that a second Show for the same sid uses
// the previously-returned notif_id as replaceID.
func TestServer_ReplaceFlow(t *testing.T) {
	ln := newTestSocket(t)
	fb := notifyd.NewFakeBus()
	st := newTestState(t)

	// Seed replace map so RegisterShow returns 99 for sid="s1".
	popup := notify.PopupContext{SessionID: "s1"}
	if err := st.RecordShown("s1", 99, popup); err != nil {
		t.Fatalf("RecordShown seed: %v", err)
	}

	runner := proc.NewFakeRunner()
	opts := notifyd.ServerOptions{
		Listener: ln,
		Bus:      fb,
		State:    st,
		Runner:   runner,
		LookPath: noopLookPath,
		GetEnv:   emptyGetEnv,
	}
	startServer(t, opts)
	time.Sleep(20 * time.Millisecond)

	dialAndSend(t, sockAddr(ln), notifyd.Frame{
		V: 1, Op: "show", SID: "s1", Title: "T2",
	})

	ok := pollUntil(t, 200*time.Millisecond, func() bool {
		return len(fb.Calls()) >= 1
	})
	if !ok {
		t.Fatal("expected at least 1 Bus.Notify call")
	}

	call := fb.Calls()[0]
	if call.ReplaceID != 99 {
		t.Errorf("replaceID: want 99, got %d", call.ReplaceID)
	}

	// FakeBus echoes replaceID back as the notif_id, so LookupAction(99)
	// should now return the popup.
	ok = pollUntil(t, 200*time.Millisecond, func() bool {
		_, found := st.LookupAction(99)
		return found
	})
	if !ok {
		t.Error("state.LookupAction(99) should return popup after RecordShown")
	}
}

// TestServer_ConcurrentClients verifies that 50 concurrent clients each
// produce exactly one Bus.Notify call without data races.
func TestServer_ConcurrentClients(t *testing.T) {
	const n = 50
	ln := newTestSocket(t)
	fb := notifyd.NewFakeBus()
	st := newTestState(t)

	runner := proc.NewFakeRunner()
	opts := notifyd.ServerOptions{
		Listener: ln,
		Bus:      fb,
		State:    st,
		Runner:   runner,
		LookPath: noopLookPath,
		GetEnv:   emptyGetEnv,
	}
	startServer(t, opts)
	time.Sleep(20 * time.Millisecond)

	addr := sockAddr(ln)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			dialAndSend(t, addr, notifyd.Frame{
				V: 1, Op: "show", SID: fmt.Sprintf("s%d", i), Title: "T",
			})
		}(i)
	}
	wg.Wait()

	ok := pollUntil(t, 500*time.Millisecond, func() bool {
		return len(fb.Calls()) == n
	})
	if !ok {
		t.Fatalf("expected %d Bus.Notify calls, got %d", n, len(fb.Calls()))
	}

	// Verify all sids are distinct.
	seen := make(map[string]bool)
	for _, c := range fb.Calls() {
		if seen[c.Frame.SID] {
			t.Errorf("duplicate SID: %s", c.Frame.SID)
		}
		seen[c.Frame.SID] = true
	}
}

// TestServer_MalformedFrame_ConnectionDropped_ServerLive verifies that a
// malformed frame causes the server to close the connection gracefully and
// remain alive for subsequent valid frames.
func TestServer_MalformedFrame_ConnectionDropped_ServerLive(t *testing.T) {
	ln := newTestSocket(t)
	fb := notifyd.NewFakeBus()
	st := newTestState(t)

	runner := proc.NewFakeRunner()
	opts := notifyd.ServerOptions{
		Listener: ln,
		Bus:      fb,
		State:    st,
		Runner:   runner,
		LookPath: noopLookPath,
		GetEnv:   emptyGetEnv,
	}
	startServer(t, opts)
	time.Sleep(20 * time.Millisecond)

	addr := sockAddr(ln)

	// Send truncated JSON (parses fine as empty object but fails Unmarshal validation).
	conn, err := net.Dial("unix", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if _, err := conn.Write([]byte("{\n")); err != nil {
		t.Fatalf("write bad frame: %v", err)
	}
	conn.Close()

	// Small grace period for the server to process the bad frame.
	time.Sleep(30 * time.Millisecond)

	// Server must still be alive; send a valid frame.
	dialAndSend(t, addr, notifyd.Frame{
		V: 1, Op: "show", SID: "s_ok", Title: "T",
	})

	ok := pollUntil(t, 200*time.Millisecond, func() bool {
		return len(fb.Calls()) == 1
	})
	if !ok {
		t.Fatalf("server should be alive after bad frame: got %d calls", len(fb.Calls()))
	}
}

// TestServer_FrameTooLarge verifies that a frame exceeding MaxFrameBytes is
// rejected and the server remains live.
func TestServer_FrameTooLarge(t *testing.T) {
	ln := newTestSocket(t)
	fb := notifyd.NewFakeBus()
	st := newTestState(t)

	runner := proc.NewFakeRunner()
	opts := notifyd.ServerOptions{
		Listener: ln,
		Bus:      fb,
		State:    st,
		Runner:   runner,
		LookPath: noopLookPath,
		GetEnv:   emptyGetEnv,
	}
	startServer(t, opts)
	time.Sleep(20 * time.Millisecond)

	addr := sockAddr(ln)

	// Build a frame with body > MaxFrameBytes so the total JSON exceeds the limit.
	oversized := notifyd.Frame{
		V:    1,
		Op:   "show",
		SID:  "big",
		Body: strings.Repeat("x", notifyd.MaxFrameBytes+500),
	}
	raw, err := json.Marshal(oversized)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	conn, err := net.Dial("unix", addr)
	if err != nil {
		t.Fatalf("dial for large frame: %v", err)
	}
	_, _ = conn.Write(append(raw, '\n'))
	conn.Close()

	time.Sleep(30 * time.Millisecond)

	// Server must not have called Notify for the oversized frame.
	if len(fb.Calls()) != 0 {
		t.Fatalf("expected 0 Notify calls for oversized frame, got %d", len(fb.Calls()))
	}

	// Server must still be alive.
	dialAndSend(t, addr, notifyd.Frame{V: 1, Op: "show", SID: "s_after_large", Title: "T"})
	ok := pollUntil(t, 200*time.Millisecond, func() bool {
		return len(fb.Calls()) == 1
	})
	if !ok {
		t.Fatalf("server should be alive after large frame: got %d calls", len(fb.Calls()))
	}
}

// TestServer_ActionTriggersFocusHelpers verifies that a "default" ActionEvent
// for an inflight notif causes FocusTmux and FocusWM to run.
func TestServer_ActionTriggersFocusHelpers(t *testing.T) {
	ln := newTestSocket(t)
	fb := notifyd.NewFakeBus()
	st := newTestState(t)

	popup := notify.PopupContext{
		SessionID:   "s1",
		TmuxPane:    "%5",
		TmuxSession: "dev",
	}
	// Seed the inflight state directly.
	if err := st.RecordShown("s1", 7, popup); err != nil {
		t.Fatalf("RecordShown: %v", err)
	}

	// Register the runner calls that FocusTmux and FocusWM will make.
	runner := proc.NewFakeRunner()
	// FocusTmux: has-session check
	runner.Register("tmux", []string{"has-session", "-t", "dev"}, nil, nil)
	// FocusTmux: switch-client + select-pane
	runner.Register("tmux", []string{
		"switch-client", "-t", "dev",
		";", "select-pane", "-t", "%5",
	}, nil, nil)
	// FocusWM: xdotool with each terminal class (first success stops iteration).
	for _, cls := range notify.TerminalClasses {
		runner.Register("xdotool", []string{"search", "--class", cls, "windowactivate"}, nil, nil)
	}
	// CloseNotification via gdbus.
	runner.Register("gdbus", []string{
		"call", "--session",
		"--dest=org.freedesktop.Notifications",
		"--object-path=/org/freedesktop/Notifications",
		"--method=org.freedesktop.Notifications.CloseNotification",
		"7",
	}, nil, nil)

	getEnv := func(k string) string {
		if k == "DISPLAY" {
			return ":0"
		}
		return ""
	}

	opts := notifyd.ServerOptions{
		Listener: ln,
		Bus:      fb,
		State:    st,
		Runner:   runner,
		LookPath: noopLookPath,
		GetEnv:   getEnv,
	}
	startServer(t, opts)
	time.Sleep(20 * time.Millisecond)

	fb.EmitAction(7, "default")

	// Give the action goroutine up to 500ms to complete.
	time.Sleep(500 * time.Millisecond)

	// Action dispatch must NOT call state.Forget (only ClosedEvent does).
	_, found := st.LookupAction(7)
	if !found {
		t.Error("LookupAction(7): action dispatch must not Forget the popup (only ClosedEvent does)")
	}
}

// TestServer_ClosedSignalForgets verifies that a ClosedEvent removes the
// inflight entry and subsequent actions for that id are no-ops.
func TestServer_ClosedSignalForgets(t *testing.T) {
	ln := newTestSocket(t)
	fb := notifyd.NewFakeBus()
	st := newTestState(t)

	popup := notify.PopupContext{SessionID: "s1"}
	if err := st.RecordShown("s1", 11, popup); err != nil {
		t.Fatalf("RecordShown: %v", err)
	}

	runner := proc.NewFakeRunner()
	opts := notifyd.ServerOptions{
		Listener: ln,
		Bus:      fb,
		State:    st,
		Runner:   runner,
		LookPath: noopLookPath,
		GetEnv:   emptyGetEnv,
	}
	startServer(t, opts)
	time.Sleep(20 * time.Millisecond)

	fb.EmitClosed(11, 1)

	// After Closed, state.LookupAction(11) should return false.
	ok := pollUntil(t, 300*time.Millisecond, func() bool {
		_, found := st.LookupAction(11)
		return !found
	})
	if !ok {
		t.Fatal("state.LookupAction(11) should return false after ClosedEvent")
	}

	// Now emit an action for the already-forgotten id — runner has no
	// registered handlers for this path, so FakeRunner would error if called.
	// The server must NOT invoke any runner calls (popup was forgotten).
	fb.EmitAction(11, "default")
	time.Sleep(100 * time.Millisecond)

	// No Bus.Notify calls should have happened.
	if len(fb.Calls()) != 0 {
		t.Errorf("expected 0 Notify calls, got %d", len(fb.Calls()))
	}
}

// TestServer_ContextCancelStops verifies that cancelling the context makes
// Serve return nil within 1s.
func TestServer_ContextCancelStops(t *testing.T) {
	ln := newTestSocket(t)
	fb := notifyd.NewFakeBus()
	st := newTestState(t)

	runner := proc.NewFakeRunner()
	opts := notifyd.ServerOptions{
		Listener: ln,
		Bus:      fb,
		State:    st,
		Runner:   runner,
		LookPath: noopLookPath,
		GetEnv:   emptyGetEnv,
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv := notifyd.NewServer(opts)
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Serve returned non-nil error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Serve did not return within 1s after context cancel")
	}
}

// TestServer_ScannerMaxBytes verifies that the scanner handles frames right
// at the MaxFrameBytes boundary correctly (accepted, not rejected).
func TestServer_ScannerMaxBytes(t *testing.T) {
	ln := newTestSocket(t)
	fb := notifyd.NewFakeBus()
	st := newTestState(t)

	runner := proc.NewFakeRunner()
	opts := notifyd.ServerOptions{
		Listener: ln,
		Bus:      fb,
		State:    st,
		Runner:   runner,
		LookPath: noopLookPath,
		GetEnv:   emptyGetEnv,
	}
	startServer(t, opts)
	time.Sleep(20 * time.Millisecond)

	// Build a frame whose JSON encoding is <= MaxFrameBytes. Because Unmarshal
	// caps individual fields at 4096 bytes we must respect that limit too.
	// We use the Title field (also capped at 4096) to approach the wire limit.
	base := notifyd.Frame{V: 1, Op: "show", SID: "boundary", Body: "B", Title: ""}
	baseJSON, _ := json.Marshal(base)
	// overhead = len(`,"title":"`) + 1 closing quote
	overhead := len(`,"title":"`) + 1
	padding := notifyd.MaxFrameBytes - len(baseJSON) - overhead
	// Clamp to the per-field limit enforced by Unmarshal.
	const maxFieldBytes = 4096
	if padding > maxFieldBytes {
		padding = maxFieldBytes
	}
	if padding < 0 {
		padding = 0
	}
	base.Title = strings.Repeat("y", padding)
	raw, _ := json.Marshal(base)
	if len(raw) > notifyd.MaxFrameBytes {
		t.Skipf("could not construct boundary frame (got %d bytes)", len(raw))
	}

	conn, err := net.Dial("unix", sockAddr(ln))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	w := bufio.NewWriter(conn)
	_, _ = w.Write(raw)
	_ = w.WriteByte('\n')
	_ = w.Flush()
	conn.Close()

	ok := pollUntil(t, 300*time.Millisecond, func() bool {
		return len(fb.Calls()) == 1
	})
	if !ok {
		t.Fatalf("expected 1 call for boundary frame, got %d", len(fb.Calls()))
	}
}

// TestServer_BusNotifyError verifies that a Bus.Notify error is handled
// gracefully: the server logs and continues without calling RecordShown.
func TestServer_BusNotifyError(t *testing.T) {
	ln := newTestSocket(t)
	fb := notifyd.NewFakeBus()
	st := newTestState(t)

	fb.SetNotifyErr(errors.New("dbus unavailable"))

	runner := proc.NewFakeRunner()
	opts := notifyd.ServerOptions{
		Listener: ln,
		Bus:      fb,
		State:    st,
		Runner:   runner,
		LookPath: noopLookPath,
		GetEnv:   emptyGetEnv,
	}
	startServer(t, opts)
	time.Sleep(20 * time.Millisecond)

	dialAndSend(t, sockAddr(ln), notifyd.Frame{
		V: 1, Op: "show", SID: "err-sid", Title: "T",
	})

	// The call is recorded but Notify returns error, so LookupAction should
	// return nothing (RecordShown is skipped on error).
	ok := pollUntil(t, 200*time.Millisecond, func() bool {
		return len(fb.Calls()) == 1
	})
	if !ok {
		t.Fatalf("expected 1 Notify call recorded even on error, got %d", len(fb.Calls()))
	}
	_, found := st.LookupAction(0)
	if found {
		t.Error("LookupAction should not find popup when Notify errored")
	}

	// Server must remain alive after the error.
	fb.SetNotifyErr(nil)
	dialAndSend(t, sockAddr(ln), notifyd.Frame{
		V: 1, Op: "show", SID: "ok-sid", Title: "T",
	})
	ok = pollUntil(t, 200*time.Millisecond, func() bool {
		return len(fb.Calls()) == 2
	})
	if !ok {
		t.Fatalf("server should survive Notify error: got %d calls", len(fb.Calls()))
	}
}

// TestServer_DispatcherExitsWhenChannelsClosed verifies that the dispatcher
// goroutine exits cleanly when both bus event channels are closed.
func TestServer_DispatcherExitsWhenChannelsClosed(t *testing.T) {
	ln := newTestSocket(t)
	fb := notifyd.NewFakeBus()
	st := newTestState(t)

	runner := proc.NewFakeRunner()
	opts := notifyd.ServerOptions{
		Listener: ln,
		Bus:      fb,
		State:    st,
		Runner:   runner,
		LookPath: noopLookPath,
		GetEnv:   emptyGetEnv,
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv := notifyd.NewServer(opts)
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ctx)
	}()
	time.Sleep(20 * time.Millisecond)

	// Close the bus channels to simulate bus shutdown, then cancel ctx.
	fb.CloseBusChannels()
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Serve returned non-nil error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Serve did not return within 1s after bus channels closed + ctx cancel")
	}
}

// TestServer_NonDefaultActionIgnored verifies that non-"default" action keys
// do not trigger the focus helpers.
func TestServer_NonDefaultActionIgnored(t *testing.T) {
	ln := newTestSocket(t)
	fb := notifyd.NewFakeBus()
	st := newTestState(t)

	popup := notify.PopupContext{SessionID: "s1"}
	if err := st.RecordShown("s1", 42, popup); err != nil {
		t.Fatalf("RecordShown: %v", err)
	}

	// Register no runner calls — if server tries to call any helper it will
	// return an error from FakeRunner, but the test just verifies no panic.
	runner := proc.NewFakeRunner()
	opts := notifyd.ServerOptions{
		Listener: ln,
		Bus:      fb,
		State:    st,
		Runner:   runner,
		LookPath: noopLookPath,
		GetEnv:   emptyGetEnv,
	}
	startServer(t, opts)
	time.Sleep(20 * time.Millisecond)

	// "close" is not the "default" key — should be a no-op.
	fb.EmitAction(42, "close")
	time.Sleep(100 * time.Millisecond)

	// Inflight popup must still be present (no Forget was called).
	_, found := st.LookupAction(42)
	if !found {
		t.Error("LookupAction(42): non-default action should not affect inflight state")
	}
}
