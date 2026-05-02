// Package notifyd defines the wire protocol for the claude-notifyd daemon.
// Frames are newline-delimited JSON (NDJSON) with a version field for
// forward-compatibility. MaxFrameBytes caps the per-line read to prevent DoS.
package notifyd

import (
	"encoding/json"
	"errors"
	"fmt"
)

// MaxFrameBytes is the maximum allowed byte length of a single wire frame
// (JSON line, no trailing newline). Per spec: 8192 bytes.
const MaxFrameBytes = 8192

// Op constants identify the frame operation.
const (
	OpShow = "show"
)

// Frame is the v=1 wire JSON struct.
type Frame struct {
	V           uint8  `json:"v"`
	Op          string `json:"op"`
	SID         string `json:"sid,omitempty"`
	Title       string `json:"title,omitempty"`
	Body        string `json:"body,omitempty"`
	Urgency     string `json:"urgency,omitempty"`
	TmuxPane    string `json:"tmux_pane,omitempty"`
	TmuxSession string `json:"tmux_session,omitempty"`
}

// Sentinel errors — use errors.Is for inspection.
var (
	ErrUnsupportedVersion = errors.New("notifyd: unsupported protocol version")
	ErrUnknownOp          = errors.New("notifyd: unknown op")
	ErrFieldTooLong       = errors.New("notifyd: field too long")
	ErrFrameTooLarge      = errors.New("notifyd: frame exceeds MaxFrameBytes")
)

// Marshal returns the JSON encoding of f. No trailing newline is added;
// the caller is responsible for appending '\n' when writing to the wire.
func Marshal(f Frame) ([]byte, error) {
	return json.Marshal(f)
}

// Unmarshal parses a single JSON line (no trailing newline required) into
// a Frame. Validation:
//   - len(line) must not exceed MaxFrameBytes
//   - v must equal 1
//   - op must be a known constant (currently only OpShow)
//   - SID length <= 256; Title and Body length <= 4096 (DoS guard)
func Unmarshal(line []byte) (Frame, error) {
	if len(line) > MaxFrameBytes {
		return Frame{}, fmt.Errorf("frame is %d bytes: %w", len(line), ErrFrameTooLarge)
	}

	var f Frame
	if err := json.Unmarshal(line, &f); err != nil {
		return Frame{}, err
	}

	if f.V != 1 {
		return Frame{}, fmt.Errorf("v=%d: %w", f.V, ErrUnsupportedVersion)
	}

	switch f.Op {
	case OpShow:
		// valid
	default:
		return Frame{}, fmt.Errorf("op=%q: %w", f.Op, ErrUnknownOp)
	}

	if len(f.SID) > 256 {
		return Frame{}, fmt.Errorf("sid length %d > 256: %w", len(f.SID), ErrFieldTooLong)
	}
	if len(f.Title) > 4096 {
		return Frame{}, fmt.Errorf("title length %d > 4096: %w", len(f.Title), ErrFieldTooLong)
	}
	if len(f.Body) > 4096 {
		return Frame{}, fmt.Errorf("body length %d > 4096: %w", len(f.Body), ErrFieldTooLong)
	}

	return f, nil
}
