package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// fakeBackend builds a configurable lookPath / execFn pair that records
// invocations. lookPath returns the entry from `available` if present.
// execFn appends every (bin, argv) to `calls` and returns whatever the
// caller pre-loads in `execErrs` (popped from the front).
type fakeBackend struct {
	available map[string]string
	calls     []recordedCall
	execErrs  []error
}

type recordedCall struct {
	bin  string
	argv []string
}

func (f *fakeBackend) lookPath(name string) (string, error) {
	if path, ok := f.available[name]; ok {
		return path, nil
	}
	return "", errors.New("not found")
}

func (f *fakeBackend) exec(bin string, argv []string, _ []string) error {
	f.calls = append(f.calls, recordedCall{bin: bin, argv: argv})
	if len(f.execErrs) == 0 {
		return nil
	}
	err := f.execErrs[0]
	f.execErrs = f.execErrs[1:]
	return err
}

func makeSound(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sample.oga")
	if err := os.WriteFile(p, []byte{0x4f, 0x67, 0x67, 0x53}, 0644); err != nil {
		t.Fatalf("write sound file: %v", err)
	}
	return p
}

func TestPlay_PrefersPwPlay(t *testing.T) {
	sound := makeSound(t)
	fb := &fakeBackend{
		available: map[string]string{
			"pw-play": "/usr/bin/pw-play",
			"paplay":  "/usr/bin/paplay",
			"ffplay":  "/usr/bin/ffplay",
		},
	}
	play(sound, fb.lookPath, fb.exec)
	if len(fb.calls) != 1 {
		t.Fatalf("expected 1 exec call, got %d: %+v", len(fb.calls), fb.calls)
	}
	got := fb.calls[0]
	wantArgv := []string{"pw-play", "--volume=0.6", sound}
	if got.bin != "/usr/bin/pw-play" {
		t.Errorf("bin = %q, want /usr/bin/pw-play", got.bin)
	}
	if !reflect.DeepEqual(got.argv, wantArgv) {
		t.Errorf("argv = %v, want %v", got.argv, wantArgv)
	}
}

func TestPlay_FallsBackToPaplay(t *testing.T) {
	sound := makeSound(t)
	fb := &fakeBackend{
		available: map[string]string{
			"paplay": "/usr/bin/paplay",
			"ffplay": "/usr/bin/ffplay",
		},
	}
	play(sound, fb.lookPath, fb.exec)
	if len(fb.calls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(fb.calls))
	}
	wantArgv := []string{"paplay", "--volume=39322", sound}
	if !reflect.DeepEqual(fb.calls[0].argv, wantArgv) {
		t.Errorf("argv = %v, want %v", fb.calls[0].argv, wantArgv)
	}
}

func TestPlay_FallsBackToFfplay(t *testing.T) {
	sound := makeSound(t)
	fb := &fakeBackend{
		available: map[string]string{
			"ffplay": "/usr/bin/ffplay",
		},
	}
	play(sound, fb.lookPath, fb.exec)
	if len(fb.calls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(fb.calls))
	}
	wantArgv := []string{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", "-volume", "60", sound}
	if !reflect.DeepEqual(fb.calls[0].argv, wantArgv) {
		t.Errorf("argv = %v, want %v", fb.calls[0].argv, wantArgv)
	}
}

func TestPlay_NoBackendsAvailable(t *testing.T) {
	sound := makeSound(t)
	fb := &fakeBackend{available: map[string]string{}}
	play(sound, fb.lookPath, fb.exec)
	if len(fb.calls) != 0 {
		t.Errorf("expected no exec calls, got %d: %+v", len(fb.calls), fb.calls)
	}
}

func TestPlay_EmptySound(t *testing.T) {
	fb := &fakeBackend{
		available: map[string]string{"pw-play": "/usr/bin/pw-play"},
	}
	play("", fb.lookPath, fb.exec)
	if len(fb.calls) != 0 {
		t.Errorf("expected no exec call when sound is empty, got %+v", fb.calls)
	}
}

func TestPlay_NonexistentSound(t *testing.T) {
	fb := &fakeBackend{
		available: map[string]string{"pw-play": "/usr/bin/pw-play"},
	}
	play(filepath.Join(t.TempDir(), "ghost.oga"), fb.lookPath, fb.exec)
	if len(fb.calls) != 0 {
		t.Errorf("expected no exec call for missing sound, got %+v", fb.calls)
	}
}

func TestPlay_ExecErrorFallsThrough(t *testing.T) {
	// pw-play exec fails (rare: file not executable / ETXTBSY etc) → try paplay.
	sound := makeSound(t)
	fb := &fakeBackend{
		available: map[string]string{
			"pw-play": "/usr/bin/pw-play",
			"paplay":  "/usr/bin/paplay",
		},
		execErrs: []error{errors.New("exec format error")},
	}
	play(sound, fb.lookPath, fb.exec)
	if len(fb.calls) != 2 {
		t.Fatalf("expected 2 exec calls (pw-play fail → paplay), got %d: %+v", len(fb.calls), fb.calls)
	}
	if fb.calls[0].argv[0] != "pw-play" || fb.calls[1].argv[0] != "paplay" {
		t.Errorf("call order wrong: %v", fb.calls)
	}
}
