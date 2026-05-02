package notifyd

import (
	"context"
	"fmt"
	"time"

	dbus "github.com/godbus/dbus/v5"

	"claude-tools/internal/obslog"
)

var dbusLogger = obslog.New("claude-notifyd")

// ActionEvent is delivered when wired-notify fires the ActionInvoked signal.
type ActionEvent struct {
	NotifID   uint32
	ActionKey string
}

// ClosedEvent is delivered on the NotificationClosed signal.
type ClosedEvent struct {
	NotifID uint32
	Reason  uint32
}

// Bus is the thin abstraction over the D-Bus session connection used by the
// daemon. Tests substitute FakeBus; production uses realBus (godbus).
type Bus interface {
	// Notify calls org.freedesktop.Notifications.Notify and returns the
	// assigned notif_id. replaceID==0 creates a new popup; non-zero updates
	// the existing popup in place.
	Notify(ctx context.Context, replaceID uint32, frame Frame) (uint32, error)

	// CloseNotification calls org.freedesktop.Notifications.CloseNotification.
	CloseNotification(ctx context.Context, id uint32) error

	// Actions returns a receive-only channel that yields ActionEvent on signal.
	Actions() <-chan ActionEvent

	// Closed returns a receive-only channel that yields ClosedEvent on signal.
	Closed() <-chan ClosedEvent

	// Close releases the connection. Idempotent.
	Close() error
}

// --- D-Bus argument helpers ---

// urgencyToByte maps the urgency string to the freedesktop notification
// urgency byte: low=0, normal=1, critical=2. Unknown values default to 1.
func urgencyToByte(urgency string) byte {
	switch urgency {
	case "low":
		return 0
	case "critical":
		return 2
	default:
		return 1 // "normal" and any unknown value
	}
}

// sessionHint returns sid if non-empty, otherwise "unknown".
// Mirrors the x-claude-session hint in cmd/claude-notify-dispatch.
func sessionHint(sid string) string {
	if sid == "" {
		return "unknown"
	}
	return sid
}

// --- realBus: production godbus implementation ---

const (
	notifDest    = "org.freedesktop.Notifications"
	notifPath    = "/org/freedesktop/Notifications"
	notifIface   = "org.freedesktop.Notifications"
	sigAction    = notifIface + ".ActionInvoked"
	sigClosed    = notifIface + ".NotificationClosed"
	maxReconnect = 5
)

// dbusObject is a minimal subset of dbus.BusObject used by realBus.
// Using our own interface allows tests to substitute a fake object.
type dbusObject interface {
	CallWithContext(ctx context.Context, method string, flags dbus.Flags, args ...interface{}) *dbus.Call
}

type realBus struct {
	conn    *dbus.Conn
	obj     dbusObject
	actions chan ActionEvent
	closed  chan ClosedEvent
	sigCh   chan *dbus.Signal
	done    chan struct{}
}

// DialSession connects to the D-Bus session bus, registers signals, and
// starts signal dispatch and reconnect goroutines.
func DialSession(ctx context.Context) (Bus, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("dbus dial session: %w", err)
	}

	b := &realBus{
		actions: make(chan ActionEvent, 16),
		closed:  make(chan ClosedEvent, 16),
		sigCh:   make(chan *dbus.Signal, 64),
		done:    make(chan struct{}),
	}
	if err := b.attach(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	go b.dispatch()
	go b.reconnectLoop(ctx)
	return b, nil
}

// attach wires conn into b and registers D-Bus signal match rules.
func (b *realBus) attach(conn *dbus.Conn) error {
	b.conn = conn
	b.obj = conn.Object(notifDest, dbus.ObjectPath(notifPath))
	conn.Signal(b.sigCh)

	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(dbus.ObjectPath(notifPath)),
		dbus.WithMatchInterface(notifIface),
		dbus.WithMatchMember("ActionInvoked"),
	); err != nil {
		return fmt.Errorf("dbus AddMatchSignal ActionInvoked: %w", err)
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(dbus.ObjectPath(notifPath)),
		dbus.WithMatchInterface(notifIface),
		dbus.WithMatchMember("NotificationClosed"),
	); err != nil {
		return fmt.Errorf("dbus AddMatchSignal NotificationClosed: %w", err)
	}
	return nil
}

// parseActionSignal extracts an ActionEvent from a raw D-Bus signal body.
// Returns (event, true) when the body is well-formed, (zero, false) otherwise.
func parseActionSignal(body []interface{}) (ActionEvent, bool) {
	if len(body) < 2 {
		return ActionEvent{}, false
	}
	id, ok1 := body[0].(uint32)
	key, ok2 := body[1].(string)
	if !ok1 || !ok2 {
		return ActionEvent{}, false
	}
	return ActionEvent{NotifID: id, ActionKey: key}, true
}

// parseClosedSignal extracts a ClosedEvent from a raw D-Bus signal body.
// Returns (event, true) when the body is well-formed, (zero, false) otherwise.
func parseClosedSignal(body []interface{}) (ClosedEvent, bool) {
	if len(body) < 2 {
		return ClosedEvent{}, false
	}
	id, ok1 := body[0].(uint32)
	reason, ok2 := body[1].(uint32)
	if !ok1 || !ok2 {
		return ClosedEvent{}, false
	}
	return ClosedEvent{NotifID: id, Reason: reason}, true
}

// runDispatchLoop reads raw D-Bus signals from sigCh and forwards them to the
// typed event channels until done is closed or sigCh is closed. Extracted as
// a package-level function so it can be unit-tested without a real D-Bus conn.
func runDispatchLoop(done <-chan struct{}, sigCh <-chan *dbus.Signal, actions chan<- ActionEvent, closed chan<- ClosedEvent) {
	for {
		select {
		case <-done:
			return
		case sig, ok := <-sigCh:
			if !ok {
				return
			}
			if sig == nil {
				continue
			}
			switch sig.Name {
			case sigAction:
				if ev, ok := parseActionSignal(sig.Body); ok {
					select {
					case actions <- ev:
					default:
						dbusLogger.Warn("dbus actions channel full, dropping ActionInvoked", "id", ev.NotifID)
					}
				}
			case sigClosed:
				if ev, ok := parseClosedSignal(sig.Body); ok {
					select {
					case closed <- ev:
					default:
						dbusLogger.Warn("dbus closed channel full, dropping NotificationClosed", "id", ev.NotifID)
					}
				}
			}
		}
	}
}

// dispatch starts the runDispatchLoop goroutine for this realBus.
func (b *realBus) dispatch() {
	runDispatchLoop(b.done, b.sigCh, b.actions, b.closed)
}

// dialFunc is the signature for a function that creates a new D-Bus connection.
// Abstracted to allow testing without a real session bus.
type dialFunc func() (*dbus.Conn, error)

// runReconnectRetries performs the exponential-backoff retry loop after a
// connection drop. It calls dial to obtain a new connection, then attaches.
// initialBackoff sets the starting delay; production callers pass 100ms,
// tests may pass a shorter value. This is a package-level function so tests
// can inject a fake dialFunc without a real D-Bus session bus.
func runReconnectRetries(ctx context.Context, done <-chan struct{}, initialBackoff time.Duration, dial dialFunc, attach func(*dbus.Conn) error, onExhausted func()) {
	backoff := initialBackoff
	for attempt := 1; attempt <= maxReconnect; attempt++ {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2

		conn, err := dial()
		if err != nil {
			dbusLogger.Warn("dbus reconnect failed", "attempt", attempt, "err", err)
			continue
		}
		if err := attach(conn); err != nil {
			if conn != nil {
				_ = conn.Close()
			}
			dbusLogger.Warn("dbus re-attach failed", "attempt", attempt, "err", err)
			continue
		}
		dbusLogger.Info("dbus reconnected", "attempt", attempt)
		return
	}

	dbusLogger.Error("dbus reconnect exhausted after max attempts, closing signal channels")
	onExhausted()
}

// waitConnDrop blocks until either the done channel is closed (returns false)
// or connCtx is cancelled (returns true — connection dropped, retry needed).
// Extracted for testability without a real D-Bus connection.
func waitConnDrop(done <-chan struct{}, connCtx context.Context) bool {
	select {
	case <-done:
		return false
	case <-connCtx.Done():
		return true
	}
}

// reconnectLoop monitors the connection and retries on disconnect with
// exponential backoff: 100ms, 200ms, 400ms, 800ms, 1600ms (5 attempts).
func (b *realBus) reconnectLoop(ctx context.Context) {
	if !waitConnDrop(b.done, b.conn.Context()) {
		return
	}

	runReconnectRetries(ctx, b.done, 100*time.Millisecond, func() (*dbus.Conn, error) { return dbus.ConnectSessionBus() }, b.attach, func() {
		close(b.actions)
		close(b.closed)
	})
	// If runReconnectRetries succeeded, restart the monitor.
	if b.conn != nil {
		go b.reconnectLoop(ctx)
	}
}

// buildNotifyHints constructs the D-Bus hints map for a Notify call.
// Extracted as a package-level function to allow unit testing without a bus.
func buildNotifyHints(frame Frame) map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"urgency":          dbus.MakeVariant(urgencyToByte(frame.Urgency)),
		"x-claude-session": dbus.MakeVariant(sessionHint(frame.SID)),
	}
}

// notifyArgs holds the positional arguments for the D-Bus Notify call in the
// order the freedesktop spec requires. Extracted for testability.
type notifyArgs struct {
	AppName    string
	ReplacesID uint32
	AppIcon    string
	Summary    string
	Body       string
	Actions    []string
	Hints      map[string]dbus.Variant
	Timeout    int32
}

// buildNotifyArgs constructs the full argument set for a Notify D-Bus call,
// matching what `notify-send --app-name=ClaudeCode` would send.
func buildNotifyArgs(replaceID uint32, frame Frame) notifyArgs {
	return notifyArgs{
		AppName:    "ClaudeCode",
		ReplacesID: replaceID,
		AppIcon:    "",
		Summary:    frame.Title,
		Body:       frame.Body,
		Actions:    []string{"default", "Focus"},
		Hints:      buildNotifyHints(frame),
		Timeout:    int32(0),
	}
}

// Notify calls org.freedesktop.Notifications.Notify with arguments matching
// what notify-send --app-name=ClaudeCode would issue per the freedesktop spec.
func (b *realBus) Notify(ctx context.Context, replaceID uint32, frame Frame) (uint32, error) {
	args := buildNotifyArgs(replaceID, frame)

	var id uint32
	call := b.obj.CallWithContext(ctx, notifIface+".Notify", 0,
		args.AppName,
		args.ReplacesID,
		args.AppIcon,
		args.Summary,
		args.Body,
		args.Actions,
		args.Hints,
		args.Timeout,
	)
	if call.Err != nil {
		return 0, fmt.Errorf("dbus Notify: %w", call.Err)
	}
	if err := call.Store(&id); err != nil {
		return 0, fmt.Errorf("dbus Notify store id: %w", err)
	}
	return id, nil
}

// CloseNotification calls org.freedesktop.Notifications.CloseNotification.
func (b *realBus) CloseNotification(ctx context.Context, id uint32) error {
	call := b.obj.CallWithContext(ctx, notifIface+".CloseNotification", 0, id)
	if call.Err != nil {
		return fmt.Errorf("dbus CloseNotification: %w", call.Err)
	}
	return nil
}

// Actions returns the channel receiving ActionInvoked events.
func (b *realBus) Actions() <-chan ActionEvent { return b.actions }

// Closed returns the channel receiving NotificationClosed events.
func (b *realBus) Closed() <-chan ClosedEvent { return b.closed }

// closeDone closes the done channel idempotently. Extracted for testability.
func closeDone(done chan struct{}) {
	select {
	case <-done:
		// already closed — idempotent
	default:
		close(done)
	}
}

// Close shuts down the dispatch goroutine and closes the D-Bus connection.
// Idempotent via done channel.
func (b *realBus) Close() error {
	closeDone(b.done)
	return b.conn.Close()
}
