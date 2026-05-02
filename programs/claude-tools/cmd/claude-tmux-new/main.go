// claude-tmux-new creates (or attaches to) a tmux session+window pair backed
// by a git worktree, optionally starting `claude` in the right pane.
//
// usage: claude-tmux-new <branch> [--from-root [<session-id>]] [--no-claude]
//
//	[--worktree-base <dir>] [--prompt <text>]
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"claude-tools/internal/gitwt"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
	"claude-tools/internal/tmux"
)

const progName = "claude-tmux-new"

var logger = obslog.New(progName)

type options struct {
	branch          string
	fromRoot        bool
	noClaude        bool
	explicitSession string
	worktreeBase    string
	initialPrompt   string
}

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-tmux-new:", err)
		os.Exit(1)
	}
	if err := run(context.Background(), proc.RealRunner{}, opts); err != nil {
		fmt.Fprintln(os.Stderr, "claude-tmux-new:", err)
		os.Exit(1)
	}
}

// parseArgs is the testable, side-effect-free arg parser.
func parseArgs(argv []string) (options, error) {
	var o options
	if len(argv) == 0 || strings.HasPrefix(argv[0], "-") {
		return options{}, fmt.Errorf("usage: claude-tmux-new <branch> [--from-root [<session-id>]] [--no-claude] [--worktree-base <dir>] [--prompt <text>]")
	}
	o.branch = argv[0]
	i := 1
	for i < len(argv) {
		switch argv[i] {
		case "--from-root":
			o.fromRoot = true
			i++
			if i < len(argv) && !strings.HasPrefix(argv[i], "-") {
				o.explicitSession = argv[i]
				i++
			}
		case "--no-claude":
			o.noClaude = true
			i++
		case "--worktree-base":
			i++
			if i >= len(argv) {
				return options{}, fmt.Errorf("--worktree-base requires a directory argument")
			}
			o.worktreeBase = argv[i]
			i++
		case "--prompt":
			i++
			if i >= len(argv) {
				return options{}, fmt.Errorf("--prompt requires a text argument")
			}
			o.initialPrompt = argv[i]
			i++
		case "-h", "--help":
			return options{}, fmt.Errorf("usage: claude-tmux-new <branch> [--from-root [<session-id>]] [--no-claude] [--worktree-base <dir>] [--prompt <text>]")
		default:
			return options{}, fmt.Errorf("unknown arg: %s", argv[i])
		}
	}
	if o.fromRoot && o.noClaude {
		return options{}, fmt.Errorf("--from-root and --no-claude are mutually exclusive")
	}
	if o.noClaude && o.initialPrompt != "" {
		return options{}, fmt.Errorf("--prompt is incompatible with --no-claude")
	}
	return o, nil
}

func run(ctx context.Context, r proc.Runner, opts options) error {
	gw := gitwt.New(r)
	tc := tmux.New(r)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	mainRepo, err := gw.MainRepo(ctx, cwd)
	if err != nil {
		return fmt.Errorf("not inside a git repo (cwd=%s): %w", cwd, err)
	}

	repoBasename := filepath.Base(mainRepo)
	session := tmux.Sanitize(repoBasename)
	if session == "" {
		return fmt.Errorf("failed to resolve repo basename")
	}
	safeBranch := tmux.Sanitize(opts.branch)
	windowName := safeBranch

	worktree, err := resolveWorktree(ctx, gw, opts, mainRepo, repoBasename, safeBranch)
	if err != nil {
		return err
	}

	sessionID := ""
	if opts.fromRoot {
		sessionID, err = pickRootSession(mainRepo, opts.explicitSession)
		if err != nil {
			return err
		}
		if sessionID == "" {
			return nil // user cancelled fzf
		}
	}

	worktreeHasHistory := claudeWorktreeHistoryExists(worktree)

	if !tc.HasSession(ctx, session) {
		if err := tc.NewSessionDetached(ctx, session, windowName, worktree); err != nil {
			return fmt.Errorf("failed to create session %s: %w", session, err)
		}
	}
	if err := tc.NewWindowSelectExisting(ctx, session, windowName, worktree); err != nil {
		return fmt.Errorf("failed to create or attach window %s:%s: %w", session, windowName, err)
	}

	target := session + ":" + windowName
	if err := tc.SetWindowOption(ctx, target, "@claude-managed", "yes"); err != nil {
		logger.Warn("set @claude-managed failed", "err", err)
	}
	_ = tc.SetWindowOption(ctx, target, "@claude-worktree", worktree)
	_ = tc.SetWindowOption(ctx, target, "@claude-main-repo", mainRepo)

	paneLines, err := tc.ListPanes(ctx, target, ".")
	if err != nil {
		return fmt.Errorf("list-panes failed: %w", err)
	}
	paneCount := len(paneLines)
	if paneCount <= 1 {
		setupNewWindow(ctx, tc, target, worktree, sessionID, worktreeHasHistory, opts)
	}

	// Switch / attach
	if os.Getenv("TMUX") != "" {
		return tc.SwitchClient(ctx, target)
	}
	return tc.AttachSessionExec(target)
}

func resolveWorktree(ctx context.Context, gw *gitwt.Client, opts options, mainRepo, repoBasename, safeBranch string) (string, error) {
	if existing, ok, err := gw.FindByBranch(ctx, mainRepo, opts.branch); err == nil && ok {
		return existing.Path, nil
	} else if err != nil {
		return "", fmt.Errorf("worktree list: %w", err)
	}

	worktree := mainRepo + "-" + safeBranch
	if opts.worktreeBase != "" {
		worktree = filepath.Join(opts.worktreeBase, repoBasename, safeBranch)
		if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
			return "", fmt.Errorf("mkdir worktree parent: %w", err)
		}
	}
	if !dirExists(worktree) {
		switch {
		case gw.HasLocalRef(ctx, mainRepo, opts.branch):
			if err := gw.AddExistingLocal(ctx, mainRepo, worktree, opts.branch); err != nil {
				return "", fmt.Errorf("worktree add (local): %w", err)
			}
		case gw.HasRemoteRef(ctx, mainRepo, opts.branch):
			if err := gw.AddTrackingRemote(ctx, mainRepo, worktree, opts.branch); err != nil {
				return "", fmt.Errorf("worktree add (origin): %w", err)
			}
		default:
			if err := gw.AddFromHead(ctx, mainRepo, worktree, opts.branch); err != nil {
				return "", fmt.Errorf("worktree add (HEAD): %w", err)
			}
		}
	}
	return worktree, nil
}

func setupNewWindow(ctx context.Context, tc *tmux.Client, target, worktree, sessionID string, hasHistory bool, opts options) {
	if opts.noClaude {
		tc.SelectPaneTitle(ctx, target+".0", "work")
		return
	}
	if err := tc.SplitWindowH(ctx, target, worktree); err != nil {
		logger.Error("split-window failed", "target", target, "err", err)
		return
	}
	tc.SelectPaneTitle(ctx, target+".0", "work")
	tc.SelectPaneTitle(ctx, target+".1", "claude")

	cmd := buildClaudeCommand(sessionID, hasHistory, opts.initialPrompt)
	if err := tc.SendKeys(ctx, target+".1", cmd, "Enter"); err != nil {
		logger.Error("send-keys claude failed", "target", target, "err", err)
	}
}

// buildClaudeCommand returns the shell command string to feed to send-keys.
// The initial prompt is shell-quoted (POSIX single-quoting).
func buildClaudeCommand(sessionID string, hasHistory bool, prompt string) string {
	var cmd string
	switch {
	case sessionID != "":
		cmd = "claude --resume " + sessionID + " --fork-session"
	case hasHistory:
		cmd = "claude --continue --fork-session"
	default:
		cmd = "claude"
	}
	if prompt != "" {
		cmd += " " + tmux.ShellQuote(prompt)
	}
	return cmd
}

// claudeWorktreeHistoryExists checks whether `~/.claude/projects/<encoded>/`
// has any *.jsonl files for the given worktree path.
func claudeWorktreeHistoryExists(worktree string) bool {
	encoded := strings.NewReplacer("/", "-", ".", "-").Replace(worktree)
	dir := filepath.Join(os.Getenv("HOME"), ".claude", "projects", encoded)
	matches, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	return len(matches) > 0
}

// pickRootSession resolves the root session id either from the explicit
// argument or via fzf.
func pickRootSession(mainRepo, explicitID string) (string, error) {
	encoded := strings.NewReplacer("/", "-", ".", "-").Replace(mainRepo)
	dir := filepath.Join(os.Getenv("HOME"), ".claude", "projects", encoded)
	if !dirExists(dir) {
		return "", fmt.Errorf("no claude sessions at %s", dir)
	}
	if explicitID != "" {
		if !fileExists(filepath.Join(dir, explicitID+".jsonl")) {
			return "", fmt.Errorf("session id not found: %s", explicitID)
		}
		return explicitID, nil
	}
	if _, err := exec.LookPath("fzf"); err != nil {
		return "", fmt.Errorf("fzf required for --from-root without an id")
	}
	pipe := fmt.Sprintf(`ls -t %q/*.jsonl 2>/dev/null | fzf --prompt='root session> ' --preview 'head -50 {}' --height=80%%`, dir)
	cmd := exec.Command("bash", "-c", pipe)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", nil // user cancelled fzf
	}
	pick := strings.TrimSpace(string(out))
	if pick == "" {
		return "", nil
	}
	return strings.TrimSuffix(filepath.Base(pick), ".jsonl"), nil
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
