package notifyd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"claude-tools/internal/notify"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
)

var serverLogger = obslog.New("claude-notifyd")

// ServerOptions configures Server via a plain struct (all fields required).
type ServerOptions struct {
	Listener net.Listener
	Bus      Bus
	State    *State
	Runner   proc.Runner
	LookPath func(string) (string, error)
	GetEnv   func(string) string
}

// Server accepts Unix-socket connections from claude-notify-hook clients,
// dispatches OpShow frames to the notification bus, and reacts to bus action
// and closed events.
type Server struct {
	opts ServerOptions
	wg   sync.WaitGroup
}

// NewServer constructs a Server from opts. All fields in opts are required.
func NewServer(opts ServerOptions) *Server {
	return &Server{opts: opts}
}

// Serve runs the accept loop and the bus event dispatcher until ctx is
// cancelled or the listener fails. Returns nil on clean shutdown.
//
// Both ctx cancellation and listener.Close() terminate the accept loop cleanly.
// In-flight connection handlers are drained before Serve returns (up to 5s).
func (s *Server) Serve(ctx context.Context) error {
	// Close the listener when ctx is done so Accept unblocks.
	go func() {
		<-ctx.Done()
		_ = s.opts.Listener.Close()
	}()

	// Start the bus event dispatcher goroutine.
	dispatchDone := make(chan struct{})
	go func() {
		defer close(dispatchDone)
		s.runDispatcher(ctx)
	}()

	// Accept loop.
	for {
		conn, err := s.opts.Listener.Accept()
		if err != nil {
			// Distinguish normal shutdown from unexpected errors.
			select {
			case <-ctx.Done():
				// Normal shutdown triggered by context cancellation.
			default:
				if !isNetClosedErr(err) {
					serverLogger.Error("accept error", "err", err)
				}
			}
			break
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(ctx, conn)
		}()
	}

	// Drain in-flight handlers with a 5-second deadline.
	drainDone := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(drainDone)
	}()
	select {
	case <-drainDone:
	case <-time.After(5 * time.Second):
		serverLogger.Warn("server drain timeout: some connections may not have completed")
	}

	// Wait for the dispatcher goroutine to exit.
	<-dispatchDone
	return nil
}

// handleConn reads a single frame from conn and dispatches it. One-shot
// semantics: one frame per connection (spec §4.2).
func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			serverLogger.Error("panic in connection handler", "panic", fmt.Sprintf("%v", r))
		}
		_ = conn.Close()
	}()

	// Wrap in LimitReader so we never read beyond MaxFrameBytes+1 bytes.
	limited := io.LimitReader(conn, int64(MaxFrameBytes)+1)
	scanner := bufio.NewScanner(limited)
	buf := make([]byte, MaxFrameBytes+1)
	scanner.Buffer(buf, MaxFrameBytes+1)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			serverLogger.Warn("frame read error", "err", err)
		}
		return
	}

	line := scanner.Bytes()
	if len(line) > MaxFrameBytes {
		serverLogger.Warn("frame too large, dropping", "len", len(line))
		return
	}

	f, err := Unmarshal(line)
	if err != nil {
		serverLogger.Warn("frame unmarshal error", "err", err)
		return
	}

	switch f.Op {
	case OpShow:
		s.handleShow(ctx, f)
	default:
		serverLogger.Warn("unknown op in frame", "op", f.Op)
	}
}

// handleShow dispatches a single OpShow frame: registers the show, calls
// Bus.Notify, and records the returned notif_id.
func (s *Server) handleShow(ctx context.Context, f Frame) {
	popup := notify.PopupContext{
		SessionID:   f.SID,
		TmuxPane:    f.TmuxPane,
		TmuxSession: f.TmuxSession,
	}

	prev := s.opts.State.RegisterShow(f.SID, popup)

	id, err := s.opts.Bus.Notify(ctx, prev, f)
	if err != nil {
		serverLogger.Error("Bus.Notify failed", "sid", f.SID, "err", err)
		return
	}

	if err := s.opts.State.RecordShown(f.SID, id, popup); err != nil {
		serverLogger.Warn("RecordShown failed (best-effort)", "sid", f.SID, "err", err)
	}
}

// runDispatcher reads ActionEvent and ClosedEvent from the bus until both
// channels close or ctx is done.
func (s *Server) runDispatcher(ctx context.Context) {
	actions := s.opts.Bus.Actions()
	closed := s.opts.Bus.Closed()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-actions:
			if !ok {
				actions = nil
				if closed == nil {
					return
				}
				continue
			}
			s.handleAction(ctx, ev)
		case ev, ok := <-closed:
			if !ok {
				closed = nil
				if actions == nil {
					return
				}
				continue
			}
			s.opts.State.Forget(ev.NotifID)
		}
	}
}

// handleAction dispatches a single ActionEvent. Only the "default" key triggers
// focus helpers; all others are silently ignored (best-effort per spec §4.6).
func (s *Server) handleAction(ctx context.Context, ev ActionEvent) {
	if ev.ActionKey != "default" {
		return
	}
	popup, ok := s.opts.State.LookupAction(ev.NotifID)
	if !ok {
		return
	}
	go s.runAction(ctx, popup, ev.NotifID)
}

// runAction performs the focus sequence: FocusTmux → FocusWM → CloseNotification.
// Each step is best-effort (errors are logged and swallowed inside the helpers).
func (s *Server) runAction(ctx context.Context, popup notify.PopupContext, notifID uint32) {
	notify.FocusTmux(ctx, s.opts.Runner, popup)
	notify.FocusWM(ctx, s.opts.Runner, s.opts.LookPath, s.opts.GetEnv)
	notify.CloseNotification(ctx, s.opts.Runner, notifID)
}

// isNetClosedErr reports whether err is the "use of closed network connection"
// sentinel that net.Listener.Accept returns after Close().
func isNetClosedErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "use of closed network connection")
}
