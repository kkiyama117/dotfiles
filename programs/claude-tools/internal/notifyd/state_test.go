package notifyd

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"claude-tools/internal/notify"
)

// TestState_RegisterAndRecordShown_RoundTrip verifies:
//   - RegisterShow returns 0 when no previous id exists for the sid
//   - RecordShown persists the notif_id to disk
//   - LoadReplaceID reads back the persisted id
func TestState_RegisterAndRecordShown_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewState(dir)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}

	popup := notify.PopupContext{SessionID: "sid-A", TmuxPane: "%1", TmuxSession: "dev"}
	sid := "sid-A"

	// No prior state: prevID must be 0.
	prevID := s.RegisterShow(sid, popup)
	if prevID != 0 {
		t.Errorf("RegisterShow: prevID = %d, want 0 (no prior state)", prevID)
	}

	// Record the notif_id returned by D-Bus Notify.
	if err := s.RecordShown(sid, 42, popup); err != nil {
		t.Fatalf("RecordShown: %v", err)
	}

	// Disk must reflect the new id.
	got := notify.LoadReplaceID(dir, sid)
	if got != 42 {
		t.Errorf("LoadReplaceID after RecordShown = %d, want 42", got)
	}
}

// TestState_ConcurrentRecordShown spawns 100 goroutines each writing a
// distinct sid and asserts all 100 disk files exist with correct ids.
// The -race flag catches any data race.
func TestState_ConcurrentRecordShown(t *testing.T) {
	const n = 100
	dir := t.TempDir()
	s, err := NewState(dir)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			sid := fmt.Sprintf("sid%d", i)
			popup := notify.PopupContext{SessionID: sid}
			if err := s.RecordShown(sid, uint32(i+1), popup); err != nil {
				t.Errorf("RecordShown(%q, %d): %v", sid, i+1, err)
			}
		}()
	}
	wg.Wait()

	// Verify all 100 disk files exist with the correct ids.
	for i := 0; i < n; i++ {
		sid := fmt.Sprintf("sid%d", i)
		want := uint32(i + 1)
		got := notify.LoadReplaceID(dir, sid)
		if got != want {
			t.Errorf("sid=%q: LoadReplaceID = %d, want %d", sid, got, want)
		}
	}
}

// TestState_WarmStart pre-seeds 3 .id files then verifies that NewState
// loads them so RegisterShow returns the seeded id as prevID.
func TestState_WarmStart(t *testing.T) {
	dir := t.TempDir()
	seeds := map[string]uint32{
		"warm-A": 10,
		"warm-B": 20,
		"warm-C": 30,
	}
	for sid, id := range seeds {
		if err := notify.SaveReplaceID(dir, sid, id); err != nil {
			t.Fatalf("SaveReplaceID(%q, %d): %v", sid, id, err)
		}
	}

	s, err := NewState(dir)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}

	// For each seeded sid, RegisterShow must return the seeded id as prevID.
	for sid, want := range seeds {
		popup := notify.PopupContext{SessionID: sid}
		got := s.RegisterShow(sid, popup)
		if got != want {
			t.Errorf("RegisterShow(%q): prevID = %d, want %d (warm-start)", sid, got, want)
		}
	}
}

// TestState_InflightLifecycle verifies the full inflight lifecycle:
//  1. RegisterShow returns 0 for a new sid.
//  2. RecordShown makes LookupAction return the popup ctx.
//  3. Forget removes from inflight (LookupAction returns false).
//  4. replace-id persists on disk even after Forget.
func TestState_InflightLifecycle(t *testing.T) {
	dir := t.TempDir()
	s, err := NewState(dir)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}

	popup := notify.PopupContext{SessionID: "sA", TmuxPane: "%2", TmuxSession: "main"}
	sid := "sA"
	const notifID uint32 = 7

	// Step 1: no prior state.
	if prev := s.RegisterShow(sid, popup); prev != 0 {
		t.Errorf("RegisterShow: prevID = %d, want 0", prev)
	}

	// Step 2: record shown — inflight must be populated.
	if err := s.RecordShown(sid, notifID, popup); err != nil {
		t.Fatalf("RecordShown: %v", err)
	}
	got, ok := s.LookupAction(notifID)
	if !ok {
		t.Fatal("LookupAction: expected ok=true after RecordShown")
	}
	if got != popup {
		t.Errorf("LookupAction: got %+v, want %+v", got, popup)
	}

	// Step 3: Forget removes from inflight.
	s.Forget(notifID)
	_, ok = s.LookupAction(notifID)
	if ok {
		t.Error("LookupAction: expected ok=false after Forget")
	}

	// Step 4: replace-id on disk must still be 7.
	if diskID := notify.LoadReplaceID(dir, sid); diskID != notifID {
		t.Errorf("LoadReplaceID after Forget = %d, want %d (disk persists)", diskID, notifID)
	}
}

// TestNewState_ErrorOnUnreadableDir verifies that NewState propagates errors
// from LoadAllReplaceIDs when the state directory is not readable.
func TestNewState_ErrorOnUnreadableDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses file permission checks")
	}
	root := t.TempDir()
	dir := root + "/unreadable"
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write a file so the dir is non-empty, then remove read permission.
	if err := os.WriteFile(dir+"/x.id", []byte("1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := NewState(dir)
	if err == nil {
		t.Error("NewState: expected error for unreadable dir, got nil")
	}
}

// TestState_RecordShownZeroIDIsNoOp verifies that RecordShown with notifID==0
// is a no-op and does not write to disk.
func TestState_RecordShownZeroIDIsNoOp(t *testing.T) {
	dir := t.TempDir()
	s, err := NewState(dir)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}

	popup := notify.PopupContext{SessionID: "sB"}
	if err := s.RecordShown("sB", 0, popup); err != nil {
		t.Errorf("RecordShown(0): expected no error, got %v", err)
	}
	// No disk file should exist.
	if id := notify.LoadReplaceID(dir, "sB"); id != 0 {
		t.Errorf("LoadReplaceID after RecordShown(0) = %d, want 0", id)
	}
	// LookupAction should find nothing.
	if _, ok := s.LookupAction(0); ok {
		t.Error("LookupAction(0): expected false after zero-id RecordShown")
	}
}

// TestState_RecordShownPersistError verifies that RecordShown propagates
// disk-write errors from SaveReplaceID (e.g. read-only directory).
func TestState_RecordShownPersistError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses file permission checks")
	}
	dir := t.TempDir()
	s, err := NewState(dir)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}

	// Make directory read-only so SaveReplaceID cannot write.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	popup := notify.PopupContext{SessionID: "sErr"}
	err = s.RecordShown("sErr", 77, popup)
	if err == nil {
		t.Error("RecordShown: expected error for read-only dir, got nil")
	}
}

// TestState_LookupActionUnknown verifies that LookupAction for an unknown id
// returns zero value and false without panicking.
func TestState_LookupActionUnknown(t *testing.T) {
	dir := t.TempDir()
	s, err := NewState(dir)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}

	got, ok := s.LookupAction(999)
	if ok {
		t.Error("LookupAction(999): expected ok=false for unknown id")
	}
	if got != (notify.PopupContext{}) {
		t.Errorf("LookupAction(999): expected zero PopupContext, got %+v", got)
	}
}
