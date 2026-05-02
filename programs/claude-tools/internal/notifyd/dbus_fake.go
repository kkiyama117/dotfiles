package notifyd

// FakeBus is a test-only in-memory implementation of Bus. It is defined in
// a non-_test.go file so that PR-D3's server_test.go can reuse it without
// depending on the real D-Bus session bus.
//
// Do NOT use FakeBus in production code.

import (
	"context"
	"sync"
	"sync/atomic"

	dbus "github.com/godbus/dbus/v5"
)

// fakeObj implements dbusObject for testing realBus.Notify and
// realBus.CloseNotification without a real D-Bus connection.
//
// Do NOT use in production code.
type fakeObj struct {
	mu    sync.Mutex
	calls []fakeObjCall
	// errForMethod maps method name to the error to return.
	errForMethod map[string]error
}

type fakeObjCall struct {
	Method string
	Args   []interface{}
}

// newFakeObj returns a fakeObj that succeeds for all methods by default.
func newFakeObj() *fakeObj {
	return &fakeObj{errForMethod: make(map[string]error)}
}

// SetError registers an error to return when method is called.
func (o *fakeObj) SetError(method string, err error) {
	o.mu.Lock()
	o.errForMethod[method] = err
	o.mu.Unlock()
}

// CallWithContext records the call and returns a *dbus.Call with the
// configured error (nil by default).
func (o *fakeObj) CallWithContext(_ context.Context, method string, _ dbus.Flags, args ...interface{}) *dbus.Call {
	o.mu.Lock()
	o.calls = append(o.calls, fakeObjCall{Method: method, Args: args})
	err := o.errForMethod[method]
	o.mu.Unlock()
	return &dbus.Call{Err: err}
}

// NotifyCall records the arguments of a single Notify invocation.
type NotifyCall struct {
	ReplaceID uint32
	Frame     Frame
}

// FakeBus implements Bus entirely in memory. Thread-safe.
type FakeBus struct {
	nextID    atomic.Uint32
	mu        sync.Mutex
	calls     []NotifyCall
	notifyErr error // if non-nil, Notify returns this error
	actions   chan ActionEvent
	closed    chan ClosedEvent
}

// NewFakeBus constructs a FakeBus ready for use in tests.
func NewFakeBus() *FakeBus {
	return &FakeBus{
		actions: make(chan ActionEvent, 32),
		closed:  make(chan ClosedEvent, 32),
	}
}

// SetNotifyErr configures Notify to return err for all subsequent calls.
// Pass nil to clear the error (test helper).
func (f *FakeBus) SetNotifyErr(err error) {
	f.mu.Lock()
	f.notifyErr = err
	f.mu.Unlock()
}

// Notify records the call and returns the assigned notification id.
// If replaceID > 0 the same id is echoed back (in-place update path).
// Otherwise a new auto-incrementing id (starting at 1) is assigned.
// If SetNotifyErr was called with a non-nil error, that error is returned.
func (f *FakeBus) Notify(_ context.Context, replaceID uint32, frame Frame) (uint32, error) {
	f.mu.Lock()
	f.calls = append(f.calls, NotifyCall{ReplaceID: replaceID, Frame: frame})
	err := f.notifyErr
	f.mu.Unlock()

	if err != nil {
		return 0, err
	}
	if replaceID > 0 {
		return replaceID, nil
	}
	return f.nextID.Add(1), nil
}

// CloseBusChannels closes the Actions and Closed channels to simulate bus
// shutdown (test helper). Call only once.
func (f *FakeBus) CloseBusChannels() {
	close(f.actions)
	close(f.closed)
}

// CloseNotification is a no-op on FakeBus.
func (f *FakeBus) CloseNotification(_ context.Context, _ uint32) error {
	return nil
}

// Actions returns the channel that EmitAction pushes to.
func (f *FakeBus) Actions() <-chan ActionEvent { return f.actions }

// Closed returns the channel that EmitClosed pushes to.
func (f *FakeBus) Closed() <-chan ClosedEvent { return f.closed }

// Close is a no-op on FakeBus (idempotent).
func (f *FakeBus) Close() error { return nil }

// Calls returns a snapshot of all recorded Notify invocations.
func (f *FakeBus) Calls() []NotifyCall {
	f.mu.Lock()
	out := make([]NotifyCall, len(f.calls))
	copy(out, f.calls)
	f.mu.Unlock()
	return out
}

// EmitAction pushes an ActionEvent onto the Actions channel (test helper).
func (f *FakeBus) EmitAction(notifID uint32, actionKey string) {
	f.actions <- ActionEvent{NotifID: notifID, ActionKey: actionKey}
}

// EmitClosed pushes a ClosedEvent onto the Closed channel (test helper).
func (f *FakeBus) EmitClosed(notifID uint32, reason uint32) {
	f.closed <- ClosedEvent{NotifID: notifID, Reason: reason}
}
