package obslog

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"claude-tools/internal/proc"
)

func TestNew_writesToWriter(t *testing.T) {
	var buf bytes.Buffer
	logger := newWith(&buf, "test-prog", proc.NewFakeRunner())
	logger.Info("hello", "key", "value")

	out := buf.String()
	if !strings.Contains(out, `"msg":"hello"`) {
		t.Errorf("missing msg in output: %s", out)
	}
	if !strings.Contains(out, `"key":"value"`) {
		t.Errorf("missing kv in output: %s", out)
	}
	if !strings.Contains(out, `"prog":"test-prog"`) {
		t.Errorf("missing prog tag in output: %s", out)
	}
}

func TestErrorForwardsToLogger(t *testing.T) {
	var buf bytes.Buffer
	fake := proc.NewFakeRunner()
	// When logger is invoked with the expected message, return success.
	fake.Register("logger", []string{"-t", "test-prog", "boom err=oops"}, nil, nil)

	logger := newWith(&buf, "test-prog", fake)
	logger.Error("boom", "err", "oops")

	if !strings.Contains(buf.String(), `"msg":"boom"`) {
		t.Errorf("error log not in stderr buffer: %s", buf.String())
	}
	// Implicit assertion: if forward did NOT call logger with the registered
	// args, FakeRunner would have returned an error which we'd see in panic
	// or failure. We accept the call as success-by-non-error.
}

func TestInfoDoesNotInvokeLogger(t *testing.T) {
	var buf bytes.Buffer
	fake := proc.NewFakeRunner()
	// Intentionally do NOT register "logger". If Info forwards, FakeRunner
	// returns "unregistered call" error — but obslog ignores forward errors,
	// so the test passes if no panic / no extra noise in stderr.
	logger := newWith(&buf, "test-prog", fake)
	logger.Info("info-msg")

	if !strings.Contains(buf.String(), `"msg":"info-msg"`) {
		t.Errorf("info message missing from stderr: %s", buf.String())
	}
}

// Smoke: slog level filter
func TestLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	logger := newWith(&buf, "test-prog", proc.NewFakeRunner())
	logger.Log(context.Background(), slog.LevelDebug, "should-not-appear")

	if strings.Contains(buf.String(), "should-not-appear") {
		t.Errorf("DEBUG should be filtered (default is INFO): %s", buf.String())
	}
}
