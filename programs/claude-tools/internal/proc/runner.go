// Package proc abstracts external command execution behind a Runner
// interface so that production code uses os/exec while tests inject
// a FakeRunner with pre-registered argv → response pairs.
package proc

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Runner runs external commands.
type Runner interface {
	// Run executes name+args and returns stdout (no stderr).
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// RealRunner is the production implementation backed by os/exec.
type RealRunner struct{}

// Run executes the command and returns stdout (stderr is dropped to /dev/null).
// If you need stderr, run os/exec directly with cmd.CombinedOutput().
func (RealRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// FakeRunner is a test double. Register expected argv tuples; calls that
// don't match return an error (so tests fail loudly on unexpected usage).
type FakeRunner struct {
	mu      sync.Mutex
	expects map[string]fakeResponse
}

type fakeResponse struct {
	out []byte
	err error
}

// NewFakeRunner returns an empty FakeRunner.
func NewFakeRunner() *FakeRunner {
	return &FakeRunner{expects: make(map[string]fakeResponse)}
}

// Register declares what to return for a specific name + args invocation.
// Multiple registrations for the same key overwrite the previous one.
func (f *FakeRunner) Register(name string, args []string, out []byte, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.expects[fakeKey(name, args)] = fakeResponse{out: out, err: err}
}

// Run looks up the registered response or returns an error.
func (f *FakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.expects[fakeKey(name, args)]
	if !ok {
		return nil, fmt.Errorf("FakeRunner: unregistered call %s %v", name, args)
	}
	return r.out, r.err
}

func fakeKey(name string, args []string) string {
	return name + "\x00" + strings.Join(args, "\x00")
}
