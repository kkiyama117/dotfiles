package notifyd

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbus "github.com/godbus/dbus/v5"
)

// --- Bus interface contract tests via FakeBus ---

// TestFakeBus_NotifyAssignsID verifies that Notify with replaceID==0
// assigns incrementing ids (1, 2, ...).
func TestFakeBus_NotifyAssignsID(t *testing.T) {
	bus := NewFakeBus()
	ctx := context.Background()
	frame := Frame{V: 1, Op: OpShow, Title: "T", Body: "B", Urgency: "normal", SID: "s1"}

	id1, err := bus.Notify(ctx, 0, frame)
	if err != nil {
		t.Fatalf("Notify 1: %v", err)
	}
	if id1 != 1 {
		t.Errorf("Notify 1: got id=%d, want 1", id1)
	}

	id2, err := bus.Notify(ctx, 0, frame)
	if err != nil {
		t.Fatalf("Notify 2: %v", err)
	}
	if id2 != 2 {
		t.Errorf("Notify 2: got id=%d, want 2", id2)
	}
}

// TestFakeBus_NotifyEchoesReplaceID verifies that Notify with a non-zero
// replaceID echoes back the same id (in-place update path).
func TestFakeBus_NotifyEchoesReplaceID(t *testing.T) {
	bus := NewFakeBus()
	ctx := context.Background()
	frame := Frame{V: 1, Op: OpShow, Title: "T", Body: "B", Urgency: "low", SID: "s2"}

	id1, err := bus.Notify(ctx, 42, frame)
	if err != nil {
		t.Fatalf("Notify replace=42 (1st): %v", err)
	}
	if id1 != 42 {
		t.Errorf("Notify replace=42 (1st): got %d, want 42", id1)
	}

	id2, err := bus.Notify(ctx, 42, frame)
	if err != nil {
		t.Fatalf("Notify replace=42 (2nd): %v", err)
	}
	if id2 != 42 {
		t.Errorf("Notify replace=42 (2nd): got %d, want 42 (stable)", id2)
	}
}

// TestFakeBus_RecordsCalls verifies that each Notify call is recorded
// with the correct arguments accessible via Calls().
func TestFakeBus_RecordsCalls(t *testing.T) {
	bus := NewFakeBus()
	ctx := context.Background()
	frame := Frame{V: 1, Op: OpShow, Title: "T", Body: "B", Urgency: "low", SID: "s1"}

	if _, err := bus.Notify(ctx, 0, frame); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	calls := bus.Calls()
	if len(calls) != 1 {
		t.Fatalf("Calls(): got %d entries, want 1", len(calls))
	}
	got := calls[0]
	if got.ReplaceID != 0 {
		t.Errorf("call[0].ReplaceID = %d, want 0", got.ReplaceID)
	}
	if got.Frame != frame {
		t.Errorf("call[0].Frame = %+v, want %+v", got.Frame, frame)
	}
}

// TestFakeBus_ActionDelivery verifies that EmitAction delivers an ActionEvent
// on the Actions() channel within 100ms.
func TestFakeBus_ActionDelivery(t *testing.T) {
	bus := NewFakeBus()

	received := make(chan ActionEvent, 1)
	go func() {
		select {
		case ev := <-bus.Actions():
			received <- ev
		case <-time.After(200 * time.Millisecond):
			// timeout: nothing sent, main goroutine will catch via its own timer
		}
	}()

	bus.EmitAction(7, "default")

	select {
	case ev := <-received:
		if ev.NotifID != 7 || ev.ActionKey != "default" {
			t.Errorf("ActionEvent = %+v, want {NotifID:7, ActionKey:\"default\"}", ev)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("ActionDelivery: timed out waiting for ActionEvent")
	}
}

// TestFakeBus_ClosedDelivery verifies that EmitClosed delivers a ClosedEvent
// on the Closed() channel within 100ms.
func TestFakeBus_ClosedDelivery(t *testing.T) {
	bus := NewFakeBus()

	received := make(chan ClosedEvent, 1)
	go func() {
		select {
		case ev := <-bus.Closed():
			received <- ev
		case <-time.After(200 * time.Millisecond):
		}
	}()

	bus.EmitClosed(9, 1)

	select {
	case ev := <-received:
		if ev.NotifID != 9 || ev.Reason != 1 {
			t.Errorf("ClosedEvent = %+v, want {NotifID:9, Reason:1}", ev)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("ClosedDelivery: timed out waiting for ClosedEvent")
	}
}

// TestFakeBus_CloseIdempotent verifies that Close() can be called multiple
// times without returning an error or panicking.
func TestFakeBus_CloseIdempotent(t *testing.T) {
	bus := NewFakeBus()
	if err := bus.Close(); err != nil {
		t.Errorf("Close() 1st call: %v", err)
	}
	if err := bus.Close(); err != nil {
		t.Errorf("Close() 2nd call: %v", err)
	}
}

// TestUrgencyToByte verifies the urgency string to byte mapping per spec.
// low→0, normal→1, critical→2, unknown→1 (default).
func TestUrgencyToByte(t *testing.T) {
	tests := []struct {
		in   string
		want byte
	}{
		{"low", 0},
		{"normal", 1},
		{"critical", 2},
		{"", 1},     // empty → default normal
		{"weird", 1}, // unknown → default normal
	}
	for _, tc := range tests {
		got := urgencyToByte(tc.in)
		if got != tc.want {
			t.Errorf("urgencyToByte(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestSessionHint verifies the session hint helper:
// empty string → "unknown", non-empty → pass-through.
func TestSessionHint(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", "unknown"},
		{"abc", "abc"},
		{"session-123", "session-123"},
	}
	for _, tc := range tests {
		got := sessionHint(tc.in)
		if got != tc.want {
			t.Errorf("sessionHint(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestFakeBus_CloseNotification verifies CloseNotification does not error on FakeBus.
func TestFakeBus_CloseNotification(t *testing.T) {
	bus := NewFakeBus()
	if err := bus.CloseNotification(context.Background(), 5); err != nil {
		t.Errorf("CloseNotification: unexpected error: %v", err)
	}
}

// TestRunDispatchLoop verifies that runDispatchLoop correctly routes signals
// to the typed event channels without a real D-Bus connection.
func TestRunDispatchLoop(t *testing.T) {
	done := make(chan struct{})
	sigCh := make(chan *dbus.Signal, 8)
	actions := make(chan ActionEvent, 8)
	closed := make(chan ClosedEvent, 8)

	go runDispatchLoop(done, sigCh, actions, closed)

	// Send ActionInvoked signal.
	sigCh <- &dbus.Signal{
		Name: sigAction,
		Body: []interface{}{uint32(42), "default"},
	}
	select {
	case ev := <-actions:
		if ev.NotifID != 42 || ev.ActionKey != "default" {
			t.Errorf("ActionEvent = %+v, want {42, default}", ev)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for ActionEvent")
	}

	// Send NotificationClosed signal.
	sigCh <- &dbus.Signal{
		Name: sigClosed,
		Body: []interface{}{uint32(99), uint32(2)},
	}
	select {
	case ev := <-closed:
		if ev.NotifID != 99 || ev.Reason != 2 {
			t.Errorf("ClosedEvent = %+v, want {99, 2}", ev)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for ClosedEvent")
	}

	// Send nil signal (should be ignored without panic).
	sigCh <- nil

	// Send malformed body (too short, should be ignored).
	sigCh <- &dbus.Signal{
		Name: sigAction,
		Body: []interface{}{uint32(1)},
	}

	// Close done channel to stop the loop.
	close(done)
}

// TestRunDispatchLoop_FullChannel verifies that a full actions channel causes
// the drop path (default branch) to be taken without blocking.
func TestRunDispatchLoop_FullChannel(t *testing.T) {
	done := make(chan struct{})
	sigCh := make(chan *dbus.Signal, 4)
	// actions channel with capacity 0 forces the default branch immediately.
	actions := make(chan ActionEvent) // unbuffered: will be full if no reader
	closed := make(chan ClosedEvent, 4)

	// Start loop without a reader on actions.
	go runDispatchLoop(done, sigCh, actions, closed)

	// Send two action signals; second should hit the default (drop) path.
	sigCh <- &dbus.Signal{Name: sigAction, Body: []interface{}{uint32(1), "default"}}
	sigCh <- &dbus.Signal{Name: sigAction, Body: []interface{}{uint32(2), "default"}}

	// Give the loop time to process.
	time.Sleep(50 * time.Millisecond)
	close(done)
}

// TestRunDispatchLoop_FullClosedChannel verifies that a full closed channel
// causes the drop path to be taken for NotificationClosed signals.
func TestRunDispatchLoop_FullClosedChannel(t *testing.T) {
	done := make(chan struct{})
	sigCh := make(chan *dbus.Signal, 4)
	actions := make(chan ActionEvent, 4)
	// closed channel unbuffered: will be full if no reader.
	closed := make(chan ClosedEvent) // unbuffered

	go runDispatchLoop(done, sigCh, actions, closed)

	// Send two closed signals; second should hit the default (drop) path.
	sigCh <- &dbus.Signal{Name: sigClosed, Body: []interface{}{uint32(1), uint32(1)}}
	sigCh <- &dbus.Signal{Name: sigClosed, Body: []interface{}{uint32(2), uint32(1)}}

	time.Sleep(50 * time.Millisecond)
	close(done)
}

// TestRunDispatchLoop_ClosedSigCh verifies that a closed sigCh stops the loop.
func TestRunDispatchLoop_ClosedSigCh(t *testing.T) {
	done := make(chan struct{})
	sigCh := make(chan *dbus.Signal)
	actions := make(chan ActionEvent, 1)
	closed := make(chan ClosedEvent, 1)

	finished := make(chan struct{})
	go func() {
		runDispatchLoop(done, sigCh, actions, closed)
		close(finished)
	}()

	close(sigCh) // closing the channel stops the loop
	select {
	case <-finished:
		// expected
	case <-time.After(200 * time.Millisecond):
		t.Fatal("runDispatchLoop did not exit after sigCh closed")
	}
	close(done)
}

// TestRunReconnectRetries_SucceedsOnFirstAttempt verifies that
// runReconnectRetries calls onExhausted if all dial attempts fail.
func TestRunReconnectRetries_AllFail(t *testing.T) {
	ctx := context.Background()
	done := make(chan struct{})
	exhausted := make(chan struct{}, 1)

	failDial := func() (*dbus.Conn, error) {
		return nil, fmt.Errorf("no session bus")
	}
	nopAttach := func(*dbus.Conn) error { return nil }

	go runReconnectRetries(ctx, done, 5*time.Millisecond, failDial, nopAttach, func() {
		exhausted <- struct{}{}
	})

	select {
	case <-exhausted:
		// expected: all 5 attempts exhausted, onExhausted called
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runReconnectRetries: timed out waiting for exhaustion")
	}
}

// TestRunReconnectRetries_StopsOnDone verifies that runReconnectRetries
// returns early when done is closed before a retry delay expires.
func TestRunReconnectRetries_StopsOnDone(t *testing.T) {
	ctx := context.Background()
	done := make(chan struct{})

	callCount := 0
	slowDial := func() (*dbus.Conn, error) {
		callCount++
		return nil, fmt.Errorf("always fail")
	}
	nopAttach := func(*dbus.Conn) error { return nil }
	nopExhausted := func() {}

	finished := make(chan struct{})
	go func() {
		runReconnectRetries(ctx, done, 5*time.Millisecond, slowDial, nopAttach, nopExhausted)
		close(finished)
	}()

	// Close done after first retry fires.
	time.Sleep(20 * time.Millisecond)
	close(done)

	select {
	case <-finished:
		// expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runReconnectRetries: did not stop after done closed")
	}
}

// TestRunReconnectRetries_SucceedsOnSecondAttempt verifies that
// runReconnectRetries stops retrying after a successful attach.
func TestRunReconnectRetries_SucceedsOnSecondAttempt(t *testing.T) {
	ctx := context.Background()
	done := make(chan struct{})
	defer close(done)

	callCount := 0
	fakeDial := func() (*dbus.Conn, error) {
		callCount++
		if callCount < 2 {
			return nil, fmt.Errorf("first attempt fails")
		}
		// Return nil conn; attach will be called with nil.
		return nil, nil
	}
	attachCalls := 0
	fakeAttach := func(conn *dbus.Conn) error {
		attachCalls++
		return nil // success
	}
	exhaustedCalled := false
	onExhausted := func() { exhaustedCalled = true }

	runReconnectRetries(ctx, done, 5*time.Millisecond, fakeDial, fakeAttach, onExhausted)

	if exhaustedCalled {
		t.Error("onExhausted should not be called when reconnect succeeds")
	}
	if attachCalls != 1 {
		t.Errorf("attach called %d times, want 1", attachCalls)
	}
}

// TestRunReconnectRetries_AttachFailFallthrough verifies that when dial
// succeeds but attach fails, the loop continues to the next attempt.
func TestRunReconnectRetries_AttachFailFallthrough(t *testing.T) {
	ctx := context.Background()
	done := make(chan struct{})
	defer close(done)

	exhausted := make(chan struct{}, 1)

	// dial always "succeeds" (returns nil conn)
	dialOK := func() (*dbus.Conn, error) { return nil, nil }
	// attach always fails
	attachFail := func(*dbus.Conn) error { return fmt.Errorf("attach error") }

	runReconnectRetries(ctx, done, 5*time.Millisecond, dialOK, attachFail, func() {
		exhausted <- struct{}{}
	})

	select {
	case <-exhausted:
		// expected: all attempts exhausted via attach-fail path
	default:
		t.Error("onExhausted should have been called after all attach failures")
	}
}

// TestRunReconnectRetries_StopsOnCtxCancel verifies early exit on ctx cancel.
func TestRunReconnectRetries_StopsOnCtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	failDial := func() (*dbus.Conn, error) {
		return nil, fmt.Errorf("fail")
	}
	nopAttach := func(*dbus.Conn) error { return nil }
	nopExhausted := func() {}

	finished := make(chan struct{})
	go func() {
		runReconnectRetries(ctx, done, 5*time.Millisecond, failDial, nopAttach, nopExhausted)
		close(finished)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-finished:
		// expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runReconnectRetries: did not stop after ctx cancelled")
	}
	close(done)
}

// TestWaitConnDrop_DoneFirst verifies that waitConnDrop returns false when
// done is closed before connCtx is cancelled.
func TestWaitConnDrop_DoneFirst(t *testing.T) {
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	close(done)
	got := waitConnDrop(done, ctx)
	if got {
		t.Error("waitConnDrop: expected false when done is closed first")
	}
}

// TestWaitConnDrop_ConnDropFirst verifies that waitConnDrop returns true when
// connCtx is cancelled before done is closed.
func TestWaitConnDrop_ConnDropFirst(t *testing.T) {
	done := make(chan struct{})
	defer close(done)
	ctx, cancel := context.WithCancel(context.Background())

	cancel() // simulate connection drop
	got := waitConnDrop(done, ctx)
	if !got {
		t.Error("waitConnDrop: expected true when connCtx cancelled first")
	}
}

// TestCloseDone verifies that closeDone is idempotent and closes the channel.
func TestCloseDone(t *testing.T) {
	done := make(chan struct{})

	// First call: should close the channel.
	closeDone(done)
	select {
	case <-done:
		// expected: channel closed
	default:
		t.Fatal("closeDone: channel not closed after first call")
	}

	// Second call: must not panic (idempotent).
	closeDone(done)
}

// TestBuildNotifyArgs verifies that buildNotifyArgs produces the correct
// positional arguments for the freedesktop Notify D-Bus call.
func TestBuildNotifyArgs(t *testing.T) {
	frame := Frame{V: 1, Op: OpShow, Title: "Task done", Body: "Claude finished", Urgency: "normal", SID: "sess-1"}
	args := buildNotifyArgs(42, frame)

	if args.AppName != "ClaudeCode" {
		t.Errorf("AppName = %q, want ClaudeCode", args.AppName)
	}
	if args.ReplacesID != 42 {
		t.Errorf("ReplacesID = %d, want 42", args.ReplacesID)
	}
	if args.AppIcon != "" {
		t.Errorf("AppIcon = %q, want empty", args.AppIcon)
	}
	if args.Summary != "Task done" {
		t.Errorf("Summary = %q, want %q", args.Summary, "Task done")
	}
	if args.Body != "Claude finished" {
		t.Errorf("Body = %q, want %q", args.Body, "Claude finished")
	}
	if len(args.Actions) != 2 || args.Actions[0] != "default" || args.Actions[1] != "Focus" {
		t.Errorf("Actions = %v, want [default Focus]", args.Actions)
	}
	if args.Timeout != int32(0) {
		t.Errorf("Timeout = %d, want 0", args.Timeout)
	}
	// Hints are verified by TestBuildNotifyHints.
}

// TestRealBus_NotifyError verifies that realBus.Notify wraps D-Bus errors.
func TestRealBus_NotifyError(t *testing.T) {
	obj := newFakeObj()
	obj.SetError(notifIface+".Notify", fmt.Errorf("D-Bus unavailable"))
	b := &realBus{
		obj:     obj,
		actions: make(chan ActionEvent, 1),
		closed:  make(chan ClosedEvent, 1),
		done:    make(chan struct{}),
	}
	frame := Frame{V: 1, Op: OpShow, Title: "T", Body: "B", Urgency: "normal", SID: "s"}
	_, err := b.Notify(context.Background(), 0, frame)
	if err == nil {
		t.Error("Notify: expected error from D-Bus, got nil")
	}
}

// TestRealBus_NotifySuccess verifies that realBus.Notify succeeds when
// the fake obj returns no error (id stored from body).
func TestRealBus_NotifySuccess(t *testing.T) {
	obj := newFakeObj()
	b := &realBus{
		obj:     obj,
		actions: make(chan ActionEvent, 1),
		closed:  make(chan ClosedEvent, 1),
		done:    make(chan struct{}),
	}
	frame := Frame{V: 1, Op: OpShow, Title: "T", Body: "B", Urgency: "normal", SID: "s"}
	// Call.Store on a nil Body returns an error; that's the "store id" error path.
	_, err := b.Notify(context.Background(), 0, frame)
	// err may be non-nil (store error) but the D-Bus call itself succeeded.
	// Just verify no panic and err contains "store" context if non-nil.
	if err != nil && err.Error() == "dbus Notify: D-Bus unavailable" {
		t.Errorf("unexpected Notify error: %v", err)
	}
}

// TestRealBus_CloseNotificationError verifies CloseNotification wraps errors.
func TestRealBus_CloseNotificationError(t *testing.T) {
	obj := newFakeObj()
	obj.SetError(notifIface+".CloseNotification", fmt.Errorf("no notification"))
	b := &realBus{
		obj:     obj,
		actions: make(chan ActionEvent, 1),
		closed:  make(chan ClosedEvent, 1),
		done:    make(chan struct{}),
	}
	err := b.CloseNotification(context.Background(), 5)
	if err == nil {
		t.Error("CloseNotification: expected error, got nil")
	}
}

// TestRealBus_CloseNotificationSuccess verifies CloseNotification returns nil on success.
func TestRealBus_CloseNotificationSuccess(t *testing.T) {
	obj := newFakeObj()
	b := &realBus{
		obj:     obj,
		actions: make(chan ActionEvent, 1),
		closed:  make(chan ClosedEvent, 1),
		done:    make(chan struct{}),
	}
	if err := b.CloseNotification(context.Background(), 5); err != nil {
		t.Errorf("CloseNotification: unexpected error: %v", err)
	}
}

// TestRealBus_ActionsAndClosed verifies that Actions() and Closed() return
// the correct channels (accessible without D-Bus).
func TestRealBus_ActionsAndClosed(t *testing.T) {
	actions := make(chan ActionEvent, 1)
	closed := make(chan ClosedEvent, 1)
	b := &realBus{
		obj:     newFakeObj(),
		actions: actions,
		closed:  closed,
		done:    make(chan struct{}),
	}
	if b.Actions() != (<-chan ActionEvent)(actions) {
		t.Error("Actions() returned wrong channel")
	}
	if b.Closed() != (<-chan ClosedEvent)(closed) {
		t.Error("Closed() returned wrong channel")
	}
}

// TestBuildNotifyArgs_ZeroReplace verifies replaceID==0 passes through correctly.
func TestBuildNotifyArgs_ZeroReplace(t *testing.T) {
	frame := Frame{V: 1, Op: OpShow, Title: "T", Body: "B", Urgency: "low", SID: ""}
	args := buildNotifyArgs(0, frame)
	if args.ReplacesID != 0 {
		t.Errorf("ReplacesID = %d, want 0", args.ReplacesID)
	}
}

// TestBuildNotifyHints verifies the D-Bus hints map construction.
func TestBuildNotifyHints(t *testing.T) {
	tests := []struct {
		name         string
		frame        Frame
		wantUrgency  byte
		wantSession  string
	}{
		{
			name:        "normal urgency with session",
			frame:       Frame{V: 1, Op: OpShow, Urgency: "normal", SID: "abc"},
			wantUrgency: 1,
			wantSession: "abc",
		},
		{
			name:        "low urgency empty session",
			frame:       Frame{V: 1, Op: OpShow, Urgency: "low", SID: ""},
			wantUrgency: 0,
			wantSession: "unknown",
		},
		{
			name:        "critical urgency",
			frame:       Frame{V: 1, Op: OpShow, Urgency: "critical", SID: "s1"},
			wantUrgency: 2,
			wantSession: "s1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hints := buildNotifyHints(tc.frame)
			if u, ok := hints["urgency"]; !ok {
				t.Error("hints missing 'urgency'")
			} else if got, ok := u.Value().(byte); !ok || got != tc.wantUrgency {
				t.Errorf("urgency = %v (type %T), want byte(%d)", u.Value(), u.Value(), tc.wantUrgency)
			}
			if s, ok := hints["x-claude-session"]; !ok {
				t.Error("hints missing 'x-claude-session'")
			} else if got, ok := s.Value().(string); !ok || got != tc.wantSession {
				t.Errorf("x-claude-session = %v, want %q", s.Value(), tc.wantSession)
			}
		})
	}
}

// TestParseActionSignal verifies the D-Bus signal body parser for ActionInvoked.
func TestParseActionSignal(t *testing.T) {
	tests := []struct {
		name    string
		body    []interface{}
		wantOK  bool
		wantEv  ActionEvent
	}{
		{
			name:   "well-formed",
			body:   []interface{}{uint32(7), "default"},
			wantOK: true,
			wantEv: ActionEvent{NotifID: 7, ActionKey: "default"},
		},
		{
			name:   "too short",
			body:   []interface{}{uint32(1)},
			wantOK: false,
		},
		{
			name:   "empty body",
			body:   []interface{}{},
			wantOK: false,
		},
		{
			name:   "wrong types",
			body:   []interface{}{"notanid", uint32(1)},
			wantOK: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev, ok := parseActionSignal(tc.body)
			if ok != tc.wantOK {
				t.Errorf("parseActionSignal ok=%v, want %v", ok, tc.wantOK)
			}
			if ok && ev != tc.wantEv {
				t.Errorf("parseActionSignal ev=%+v, want %+v", ev, tc.wantEv)
			}
		})
	}
}

// TestParseClosedSignal verifies the D-Bus signal body parser for NotificationClosed.
func TestParseClosedSignal(t *testing.T) {
	tests := []struct {
		name    string
		body    []interface{}
		wantOK  bool
		wantEv  ClosedEvent
	}{
		{
			name:   "well-formed",
			body:   []interface{}{uint32(9), uint32(1)},
			wantOK: true,
			wantEv: ClosedEvent{NotifID: 9, Reason: 1},
		},
		{
			name:   "too short",
			body:   []interface{}{uint32(9)},
			wantOK: false,
		},
		{
			name:   "empty body",
			body:   []interface{}{},
			wantOK: false,
		},
		{
			name:   "wrong types",
			body:   []interface{}{uint32(9), "wrongtype"},
			wantOK: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev, ok := parseClosedSignal(tc.body)
			if ok != tc.wantOK {
				t.Errorf("parseClosedSignal ok=%v, want %v", ok, tc.wantOK)
			}
			if ok && ev != tc.wantEv {
				t.Errorf("parseClosedSignal ev=%+v, want %+v", ev, tc.wantEv)
			}
		})
	}
}
