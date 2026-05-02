package proc

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFakeRunner_registered(t *testing.T) {
	f := NewFakeRunner()
	f.Register("tmux", []string{"display-message", "-p", "#{session_name}"},
		[]byte("mysession\n"), nil)

	got, err := f.Run(context.Background(), "tmux", "display-message", "-p", "#{session_name}")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if string(got) != "mysession\n" {
		t.Errorf("output = %q, want %q", got, "mysession\n")
	}
}

func TestFakeRunner_unregistered(t *testing.T) {
	f := NewFakeRunner()
	_, err := f.Run(context.Background(), "tmux", "list-sessions")
	if err == nil {
		t.Fatal("Run should fail for unregistered command")
	}
	if !strings.Contains(err.Error(), "unregistered") {
		t.Errorf("error = %v, expected 'unregistered'", err)
	}
}

func TestFakeRunner_returnsError(t *testing.T) {
	f := NewFakeRunner()
	want := errors.New("boom")
	f.Register("tmux", []string{"x"}, nil, want)

	_, err := f.Run(context.Background(), "tmux", "x")
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

func TestRealRunner_echo(t *testing.T) {
	r := RealRunner{}
	got, err := r.Run(context.Background(), "echo", "-n", "hi")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if string(got) != "hi" {
		t.Errorf("output = %q, want %q", got, "hi")
	}
}
