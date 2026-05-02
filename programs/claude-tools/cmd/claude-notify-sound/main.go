// claude-notify-sound plays an audio file via the first available
// backend (pipewire → pulseaudio → ffmpeg) and replaces the current
// process with that player on success. Mirrors the shell parity of the
// previous claude-notify-sound.sh.
//
// Argument $1: absolute path to the sound file.
// No-ops (silent exit 0) when:
//   - $1 is empty
//   - $1 does not exist or is not readable
//   - none of the backends is in $PATH
package main

import (
	"os"
	"os/exec"
	"syscall"
)

const progName = "claude-notify-sound"

// backend describes one audio player binary plus its argv (excluding
// the trailing sound-file argument).
type backend struct {
	name string
	args []string
}

// backends are tried in shell-parity priority order: pipewire → pulseaudio → ffmpeg.
//
// Volume normalisation (≈ 60% across backends):
//   - pw-play --volume=0.6   linear scale 0.0..1.0
//   - paplay  --volume=39322 0..65536 (≈ 0.6 * 65536)
//   - ffplay  -volume 60     0..100
var backends = []backend{
	{"pw-play", []string{"--volume=0.6"}},
	{"paplay", []string{"--volume=39322"}},
	{"ffplay", []string{"-nodisp", "-autoexit", "-loglevel", "quiet", "-volume", "60"}},
}

func main() {
	sound := ""
	if len(os.Args) > 1 {
		sound = os.Args[1]
	}
	play(sound, exec.LookPath, syscall.Exec)
}

// play picks the first available backend and exec's it with sound as
// the final argument. The execFn signature matches syscall.Exec; in
// production a successful exec replaces the process and never returns.
// For tests, fakeExec returning nil signals "exec'd successfully — do
// not try more backends" so that the priority order can be asserted.
//
// Hook contract: silent exit 0 on every failure mode (caller fire-and-forgets).
func play(
	sound string,
	lookPath func(string) (string, error),
	execFn func(argv0 string, argv []string, envv []string) error,
) {
	if sound == "" {
		return
	}
	if _, err := os.Stat(sound); err != nil {
		return
	}
	for _, b := range backends {
		bin, err := lookPath(b.name)
		if err != nil {
			continue
		}
		argv := make([]string, 0, len(b.args)+2)
		argv = append(argv, b.name)
		argv = append(argv, b.args...)
		argv = append(argv, sound)
		// syscall.Exec replaces the process on success and returns only on error.
		// fakeExec in tests returns nil to mean "treated as success".
		if err := execFn(bin, argv, os.Environ()); err == nil {
			return
		}
	}
}
