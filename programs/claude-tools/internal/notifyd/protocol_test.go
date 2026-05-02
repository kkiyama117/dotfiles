package notifyd

import (
	"errors"
	"strings"
	"testing"
)

// TestMarshalUnmarshal_RoundTrip builds a populated v=1 Show frame,
// Marshal then Unmarshal, expects deep equal.
func TestMarshalUnmarshal_RoundTrip(t *testing.T) {
	orig := Frame{
		V:           1,
		Op:          OpShow,
		SID:         "session-abc-123",
		Title:       "Claude Code",
		Body:        "Task completed",
		Urgency:     "normal",
		TmuxPane:    "%5",
		TmuxSession: "dev",
	}

	data, err := Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Marshal: returned empty bytes")
	}

	got, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: unexpected error: %v", err)
	}

	if got != orig {
		t.Errorf("round-trip mismatch:\n got  %+v\n want %+v", got, orig)
	}
}

// TestUnmarshal_UnsupportedVersion verifies that v=2 returns ErrUnsupportedVersion.
func TestUnmarshal_UnsupportedVersion(t *testing.T) {
	line := []byte(`{"v":2,"op":"show","sid":"x"}`)
	_, err := Unmarshal(line)
	if err == nil {
		t.Fatal("Unmarshal: expected error for v=2, got nil")
	}
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Errorf("Unmarshal: error = %v, want wrapping ErrUnsupportedVersion", err)
	}
}

// TestUnmarshal_UnknownOp verifies that an unrecognized op returns ErrUnknownOp.
func TestUnmarshal_UnknownOp(t *testing.T) {
	line := []byte(`{"v":1,"op":"bogus","sid":"x"}`)
	_, err := Unmarshal(line)
	if err == nil {
		t.Fatal("Unmarshal: expected error for unknown op, got nil")
	}
	if !errors.Is(err, ErrUnknownOp) {
		t.Errorf("Unmarshal: error = %v, want wrapping ErrUnknownOp", err)
	}
}

// TestUnmarshal_MalformedJSON verifies that truncated/invalid JSON returns a non-nil error.
func TestUnmarshal_MalformedJSON(t *testing.T) {
	line := []byte(`{"v":1,"op":"show",`)
	_, err := Unmarshal(line)
	if err == nil {
		t.Fatal("Unmarshal: expected error for malformed JSON, got nil")
	}
}

// TestUnmarshal_FieldTooLong verifies that SID exceeding 256 chars returns ErrFieldTooLong.
func TestUnmarshal_FieldTooLong(t *testing.T) {
	longSID := strings.Repeat("a", 257)
	line := []byte(`{"v":1,"op":"show","sid":"` + longSID + `"}`)
	_, err := Unmarshal(line)
	if err == nil {
		t.Fatal("Unmarshal: expected error for SID > 256 chars, got nil")
	}
	if !errors.Is(err, ErrFieldTooLong) {
		t.Errorf("Unmarshal: error = %v, want wrapping ErrFieldTooLong", err)
	}
}

// TestUnmarshal_AllOptionalEmpty verifies that a minimal Show frame with no
// optional fields is valid and all string fields are empty.
func TestUnmarshal_AllOptionalEmpty(t *testing.T) {
	line := []byte(`{"v":1,"op":"show"}`)
	got, err := Unmarshal(line)
	if err != nil {
		t.Fatalf("Unmarshal: unexpected error: %v", err)
	}
	if got.V != 1 {
		t.Errorf("V = %d, want 1", got.V)
	}
	if got.Op != OpShow {
		t.Errorf("Op = %q, want %q", got.Op, OpShow)
	}
	if got.SID != "" {
		t.Errorf("SID = %q, want empty", got.SID)
	}
	if got.Title != "" {
		t.Errorf("Title = %q, want empty", got.Title)
	}
	if got.Body != "" {
		t.Errorf("Body = %q, want empty", got.Body)
	}
	if got.Urgency != "" {
		t.Errorf("Urgency = %q, want empty", got.Urgency)
	}
	if got.TmuxPane != "" {
		t.Errorf("TmuxPane = %q, want empty", got.TmuxPane)
	}
	if got.TmuxSession != "" {
		t.Errorf("TmuxSession = %q, want empty", got.TmuxSession)
	}
}

// TestMaxFrameBytesConstant is a load-bearing assertion per spec.
func TestMaxFrameBytesConstant(t *testing.T) {
	if MaxFrameBytes != 8192 {
		t.Errorf("MaxFrameBytes = %d, want 8192", MaxFrameBytes)
	}
}
