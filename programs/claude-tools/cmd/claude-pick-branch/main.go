// claude-pick-branch is a tmux popup wrapper: pick a local branch via fzf,
// then exec claude-tmux-new with the chosen branch and any passthrough
// flags. Exits 0 silently on user cancel.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"claude-tools/internal/gitwt"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
	"claude-tools/internal/tmux"
)

const progName = "claude-pick-branch"

var logger = obslog.New(progName)

func main() {
	ctx := context.Background()
	r := proc.RealRunner{}
	tc := tmux.New(r)

	if _, err := exec.LookPath("fzf"); err != nil {
		tc.Display(ctx, "fzf is required (install via paru -S fzf)")
		fmt.Fprintln(os.Stderr, "fzf is required (install via paru -S fzf)")
		os.Exit(1)
	}

	claudeTmuxNewBin, err := exec.LookPath("claude-tmux-new")
	if err != nil {
		tc.Display(ctx, "claude-pick-branch: claude-tmux-new not found in PATH")
		fmt.Fprintln(os.Stderr, "claude-pick-branch: claude-tmux-new not found in PATH")
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		tc.Display(ctx, "claude-pick-branch: getwd failed")
		os.Exit(1)
	}

	gw := gitwt.New(r)
	branches, err := gw.LocalBranches(ctx, cwd)
	if err != nil {
		tc.Display(ctx, fmt.Sprintf("claude-pick-branch: git for-each-ref failed (cwd=%s)", cwd))
		os.Exit(1)
	}
	if len(branches) == 0 {
		tc.Display(ctx, "claude-pick-branch: no local branches")
		os.Exit(0)
	}

	passthrough := os.Args[1:]
	prompt := promptForFlags(passthrough)

	pick, err := runFzf(strings.Join(branches, "\n"), prompt)
	if err != nil || pick == "" {
		os.Exit(0) // user cancel
	}

	args := buildExecArgs(pick, passthrough)
	if err := syscall.Exec(claudeTmuxNewBin, args, os.Environ()); err != nil {
		logger.Error("syscall.Exec failed", "bin", claudeTmuxNewBin, "err", err)
		os.Exit(1)
	}
}

// buildExecArgs builds the argv vector handed to syscall.Exec.
// argv[0] is the binary name (per syscall convention).
func buildExecArgs(branch string, passthrough []string) []string {
	out := []string{"claude-tmux-new", branch}
	out = append(out, passthrough...)
	return out
}

// promptForFlags chooses the fzf prompt string based on passthrough flags.
// Only the bare token "--no-claude" is recognized; the "=" form
// (--no-claude=true) is forwarded verbatim to claude-tmux-new but does not
// switch the prompt label here. Acceptable because tmux bindings.conf
// always passes the bare flag.
func promptForFlags(passthrough []string) string {
	for _, a := range passthrough {
		if a == "--no-claude" {
			return "worktree branch> "
		}
	}
	return "claude branch> "
}

func runFzf(stdin, prompt string) (string, error) {
	cmd := exec.Command("fzf", "--prompt="+prompt, "--height=100%")
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
