package notifyd

import (
	"fmt"
	"sync"

	"claude-tools/internal/notify"
	"claude-tools/internal/obslog"
)

var stateLogger = obslog.New("claude-notifyd")

// State holds the in-memory popup state for the daemon.
//
//   - replace: sid -> last notif_id (mirrors disk; populated on warm-start)
//   - inflight: notif_id -> PopupContext (volatile; lost on restart, re-populated
//     by incoming Show frames)
//
// All methods are safe for concurrent use.
type State struct {
	mu       sync.RWMutex
	dir      string
	replace  map[string]uint32             // sid -> last notif_id (mirror of disk)
	inflight map[uint32]notify.PopupContext // notif_id -> popup ctx (volatile)
}

// NewState constructs a State, performing a warm-start by loading all
// existing replace-ids from stateDir. A missing directory is treated as
// "no prior state" and is not an error.
func NewState(stateDir string) (*State, error) {
	seed, err := notify.LoadAllReplaceIDs(stateDir)
	if err != nil {
		return nil, fmt.Errorf("notifyd state warm-start: %w", err)
	}
	stateLogger.Info("state warm-start", "dir", stateDir, "loaded", len(seed))
	return &State{
		dir:      stateDir,
		replace:  seed,
		inflight: make(map[uint32]notify.PopupContext),
	}, nil
}

// RegisterShow returns the previous replace-id for sid (0 if none). It does
// NOT mutate state — the notif_id is unknown until D-Bus Notify returns.
// Call RecordShown after Notify succeeds.
func (s *State) RegisterShow(sid string, _ notify.PopupContext) (prevID uint32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.replace[sid]
}

// RecordShown is called after org.freedesktop.Notifications.Notify returns
// notifID. It:
//  1. Updates the in-memory replace table.
//  2. Registers the popup ctx in the inflight map for action lookup.
//  3. Persists sid->notifID to disk via notify.SaveReplaceID.
//
// A notifID of 0 is a no-op (D-Bus error path).
func (s *State) RecordShown(sid string, notifID uint32, popup notify.PopupContext) error {
	if notifID == 0 {
		return nil
	}
	s.mu.Lock()
	s.replace[sid] = notifID
	s.inflight[notifID] = popup
	s.mu.Unlock()

	if err := notify.SaveReplaceID(s.dir, sid, notifID); err != nil {
		return fmt.Errorf("RecordShown persist: %w", err)
	}
	return nil
}

// LookupAction returns the PopupContext for notifID if it is currently
// inflight (i.e. the popup is visible and has not been forgotten yet).
// Returns (zero, false) for unknown ids.
func (s *State) LookupAction(notifID uint32) (notify.PopupContext, bool) {
	s.mu.RLock()
	p, ok := s.inflight[notifID]
	s.mu.RUnlock()
	return p, ok
}

// Forget removes notifID from the inflight map. The replace-id on disk is
// NOT removed — it is kept for the next Show on the same sid (replace
// semantics). Safe to call with an unknown id.
func (s *State) Forget(notifID uint32) {
	s.mu.Lock()
	delete(s.inflight, notifID)
	s.mu.Unlock()
}
