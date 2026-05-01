// Package obslog wraps log/slog with a stderr JSON handler that also
// forwards ERROR level records to `logger -t <progname>` for syslog.
//
// Shell parity: shell 時代の
//
//	command -v logger >/dev/null 2>&1 && logger -t <prog> "<msg>"
//
// と等価の syslog 転送を ERROR 記録時に自動で行う。`logger` コマンド
// 不在時は静かにスキップする。
package obslog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"claude-tools/internal/proc"
)

// New returns a logger that writes JSON to stderr and forwards ERROR
// records to `logger -t progname`.
func New(progname string) *slog.Logger {
	return newWith(os.Stderr, progname, proc.RealRunner{})
}

func newWith(w io.Writer, progname string, runner proc.Runner) *slog.Logger {
	base := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	h := &forwardHandler{
		next:   base,
		prog:   progname,
		runner: runner,
	}
	return slog.New(h).With("prog", progname)
}

type forwardHandler struct {
	next   slog.Handler
	prog   string
	runner proc.Runner
}

func (h *forwardHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.next.Enabled(ctx, lvl)
}

func (h *forwardHandler) Handle(ctx context.Context, r slog.Record) error {
	if err := h.next.Handle(ctx, r); err != nil {
		return err
	}
	if r.Level >= slog.LevelError {
		h.forwardToSyslog(ctx, r)
	}
	return nil
}

func (h *forwardHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &forwardHandler{next: h.next.WithAttrs(attrs), prog: h.prog, runner: h.runner}
}

func (h *forwardHandler) WithGroup(name string) slog.Handler {
	return &forwardHandler{next: h.next.WithGroup(name), prog: h.prog, runner: h.runner}
}

func (h *forwardHandler) forwardToSyslog(ctx context.Context, r slog.Record) {
	var sb strings.Builder
	sb.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&sb, " %s=%v", a.Key, a.Value.Any())
		return true
	})
	// Best-effort: ignore errors. logger absence is silent skip.
	_, _ = h.runner.Run(ctx, "logger", "-t", h.prog, sb.String())
}
