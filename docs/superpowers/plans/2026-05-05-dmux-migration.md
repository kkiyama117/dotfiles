# dmux Migration (Worktree Layout Alignment) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align the self-built worktree spawner stack (`claude-tmux-new` / `claude-kill-session` / `claude-branch-merge`) and the 3 slash commands (`/branch-out` / `/branch-finish` / `/branch-merge`) with dmux's directory conventions: worktrees move from `~/.local/share/worktrees/<repo>/<branch>/` to `<repo>/.dmux/worktrees/<flat-slug>/`, and `internal/gitwt.SanitizeSlug` is a bit-exact Go port of dmux's `sanitizeWorktreeSlugFromBranch()`.

**Architecture:** Surgical update (Approach 1 from the spec). Cockpit, notify, and other ancillary binaries are untouched (B-2 from spec). dmux TUI is installed via mise global npm but not depended on at runtime — the rewritten `claude-tmux-new` operates in standalone direct mode (Choice A from spec). `.gitignore` gets `.dmux/` appended idempotently on first `/branch-out` invocation per repo.

**Tech Stack:** Go 1.22+ (claude-tools package), table-driven tests via `testing` stdlib, `proc.Runner` interface for git invocations, chezmoi templates for distribution, mise for npm-based dmux install.

---

## Reference Documents

- **Spec**: `docs/superpowers/specs/2026-05-05-dmux-migration-design.md`
- **Parent spec**: `docs/superpowers/specs/2026-05-02-tmux-scripts-go-migration-design.md`
- **Upstream port source**: `https://github.com/standardagents/dmux/blob/main/src/utils/paneNaming.ts` (`sanitizeWorktreeSlugFromBranch` lines 33-44)
- **Smoke checklist**: produced in Task 11

## File Structure

### Create (new files)

| Path | Responsibility |
|---|---|
| `docs/superpowers/smoke/2026-05-05-dmux-migration-smoke.md` | Manual smoke checklist (6 items) |

### Modify (existing files)

| Path | Lines / scope | Responsibility |
|---|---|---|
| `programs/claude-tools/internal/gitwt/gitwt.go` | append after L234 | Add `SanitizeSlug`, `DmuxWorktreeRoot`, `EnsureGitignoreEntry` |
| `programs/claude-tools/internal/gitwt/gitwt_test.go` | append after L203 | Add table tests for the 3 new functions |
| `programs/claude-tools/cmd/claude-tmux-new/main.go` | L1-7 doc, L33 usage, L36-43 options, L84-90 parseArgs, L113-183 run, L185-216 resolveWorktree | Deprecate `--worktree-base`, switch base path to `.dmux/worktrees/<flat-slug>`, ensure `.gitignore` entry |
| `programs/claude-tools/cmd/claude-tmux-new/main_test.go` | L20, L25 cases | Update `--worktree-base` test cases to deprecated-warn variant |
| `programs/claude-tools/cmd/claude-kill-session/main.go` | insert before L75 | Add sanity check for `<main-repo>/.dmux/worktrees/` prefix |
| `programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-out.md` | L48-52, L67-71 | Remove `--worktree-base` flag; update worktree path notes; document `.gitignore` auto-append |
| `programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-finish.md` | tail notes | Update worktree path example (verify only — file may already be path-free) |
| `programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-merge.md` | tail notes | Update worktree path example (verify only — file may already be path-free) |
| `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` | tail (after TPM bootstrap, after L96) | Add `mise use -g npm:dmux@latest` |
| `.chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl` | tail | Informational dmux presence check |
| `programs/claude-tools/README.md` | claude-tmux-new note row + new compatibility section | Refresh worktree path description |
| `docs/todos.md` | F-7 entry sub-bullets | Add "dmux 互換化済み (2026-05-05)" follow-up |
| `docs/superpowers/specs/2026-05-05-dmux-migration-design.md` | L3 Status field | Mark `Implemented (2026-05-05)` after final verification |

### Out of scope (explicitly untouched)

`programs/claude-tools/cmd/{claude-cockpit-*,claude-notify-*,claude-notifyd,claude-respawn-pane,claude-branch,claude-pick-branch}/` and all `internal/{cockpit,notify,notifyd,obslog,proc,xdg,atomicfile,tmux}/` packages.

---

## Task 1: Add `SanitizeSlug` to `internal/gitwt`

**Goal:** Bit-exact Go port of dmux's `sanitizeWorktreeSlugFromBranch()`.

**Files:**
- Modify: `programs/claude-tools/internal/gitwt/gitwt.go` (append after L234, add `regexp` import)
- Test: `programs/claude-tools/internal/gitwt/gitwt_test.go` (append after L203)

- [ ] **Step 1: Write the failing tests**

Append this to `programs/claude-tools/internal/gitwt/gitwt_test.go`:

```go
func TestSanitizeSlug(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"normal type/kebab", "feat/dmux-migration", "feat-dmux-migration"},
		{"uppercase + space", "Feat/Dmux Migration", "feat-dmux-migration"},
		{"dot preserved", "chore/v1.2.3", "chore-v1.2.3"},
		{"double slash + double dash collapsed", "feat//double--dash", "feat-double-dash"},
		{"strip leading/trailing dash", "-feat/leading-dash-", "feat-leading-dash"},
		{"strip leading dot", "..weird", "weird"},
		{"empty residue falls back to pane", "///", "pane"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SanitizeSlug(c.in); got != c.want {
				t.Errorf("SanitizeSlug(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd programs/claude-tools && go test ./internal/gitwt/ -run TestSanitizeSlug -v`
Expected: `undefined: SanitizeSlug` compile error.

- [ ] **Step 3: Implement `SanitizeSlug`**

Update the import block at the top of `programs/claude-tools/internal/gitwt/gitwt.go` (replace L7-14) to include `regexp`:

```go
import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"claude-tools/internal/proc"
)
```

Append this block at the end of `gitwt.go` (after L234):

```go
// SanitizeSlug converts a branch name into a worktree directory slug,
// matching dmux's src/utils/paneNaming.ts sanitizeWorktreeSlugFromBranch().
//
// Algorithm:
//  1. trim whitespace
//  2. lowercase
//  3. replace runs of `\` and `/` with single `-`
//  4. replace runs of `[^a-z0-9._-]` with single `-`
//  5. collapse runs of `-` to single `-`
//  6. strip leading and trailing `-`
//  7. strip leading and trailing `.`
//  8. fall back to "pane" if the result is empty
func SanitizeSlug(branchName string) string {
	s := strings.ToLower(strings.TrimSpace(branchName))
	s = slashRun.ReplaceAllString(s, "-")
	s = nonSlugRun.ReplaceAllString(s, "-")
	s = dashRun.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	s = strings.Trim(s, ".")
	if s == "" {
		return "pane"
	}
	return s
}

var (
	slashRun   = regexp.MustCompile(`[\\/]+`)
	nonSlugRun = regexp.MustCompile(`[^a-z0-9._-]+`)
	dashRun    = regexp.MustCompile(`-+`)
)
```

- [ ] **Step 4: Run tests to verify pass**

Run: `cd programs/claude-tools && go test ./internal/gitwt/ -run TestSanitizeSlug -v`
Expected: 7 sub-tests PASS.

- [ ] **Step 5: Run race + full package tests**

Run: `cd programs/claude-tools && go test -race ./internal/gitwt/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add programs/claude-tools/internal/gitwt/gitwt.go programs/claude-tools/internal/gitwt/gitwt_test.go
git commit -m "feat(gitwt): add SanitizeSlug — port of dmux sanitizeWorktreeSlugFromBranch"
```

---

## Task 2: Add `DmuxWorktreeRoot` to `internal/gitwt`

**Goal:** A pure helper that returns `<repoRoot>/.dmux/worktrees`.

**Files:**
- Modify: `programs/claude-tools/internal/gitwt/gitwt.go` (append after `SanitizeSlug` block, add `path/filepath` import)
- Test: `programs/claude-tools/internal/gitwt/gitwt_test.go` (append after `TestSanitizeSlug`)

- [ ] **Step 1: Write the failing test**

Append to `gitwt_test.go`:

```go
func TestDmuxWorktreeRoot(t *testing.T) {
	got := DmuxWorktreeRoot("/home/kiyama/.local/share/chezmoi")
	want := "/home/kiyama/.local/share/chezmoi/.dmux/worktrees"
	if got != want {
		t.Errorf("DmuxWorktreeRoot = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `cd programs/claude-tools && go test ./internal/gitwt/ -run TestDmuxWorktreeRoot -v`
Expected: `undefined: DmuxWorktreeRoot`.

- [ ] **Step 3: Implement `DmuxWorktreeRoot`**

Update the import block at the top of `gitwt.go` to include `path/filepath`:

```go
import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"claude-tools/internal/proc"
)
```

Append to `gitwt.go` (after the `SanitizeSlug` block but before the `var (...)` regex block):

```go
// DmuxWorktreeRoot returns <repoRoot>/.dmux/worktrees as a clean path.
// The caller is responsible for ensuring repoRoot is the main worktree's
// toplevel (use Client.MainRepo or Client.TopLevel).
func DmuxWorktreeRoot(repoRoot string) string {
	return filepath.Join(repoRoot, ".dmux", "worktrees")
}
```

- [ ] **Step 4: Run test to verify pass**

Run: `cd programs/claude-tools && go test ./internal/gitwt/ -run TestDmuxWorktreeRoot -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add programs/claude-tools/internal/gitwt/gitwt.go programs/claude-tools/internal/gitwt/gitwt_test.go
git commit -m "feat(gitwt): add DmuxWorktreeRoot helper"
```

---

## Task 3: Add `EnsureGitignoreEntry` to `internal/gitwt`

**Goal:** Idempotently append a line to `<repoRoot>/.gitignore`.

**Files:**
- Modify: `programs/claude-tools/internal/gitwt/gitwt.go` (append after `DmuxWorktreeRoot`, add `os` import)
- Test: `programs/claude-tools/internal/gitwt/gitwt_test.go` (append after `TestDmuxWorktreeRoot`, ensure `os` and `path/filepath` are imported)

- [ ] **Step 1: Write the failing tests**

First ensure the imports in `gitwt_test.go` cover `os` and `path/filepath`. If they are not already present, add them. Then append:

```go
func TestEnsureGitignoreEntry(t *testing.T) {
	t.Run("creates new file when absent", func(t *testing.T) {
		dir := t.TempDir()
		changed, err := EnsureGitignoreEntry(dir, ".dmux/")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if !changed {
			t.Errorf("changed=false, want true")
		}
		got, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(got) != ".dmux/\n" {
			t.Errorf("file body = %q, want %q", got, ".dmux/\n")
		}
	})

	t.Run("noop when entry already present", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".gitignore")
		if err := os.WriteFile(path, []byte("node_modules/\n.dmux/\nbuild/\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		changed, err := EnsureGitignoreEntry(dir, ".dmux/")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if changed {
			t.Errorf("changed=true, want false")
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "node_modules/\n.dmux/\nbuild/\n" {
			t.Errorf("file body modified: %q", got)
		}
	})

	t.Run("appends newline when file lacks trailing newline", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".gitignore")
		if err := os.WriteFile(path, []byte("node_modules/"), 0o644); err != nil {
			t.Fatal(err)
		}
		changed, err := EnsureGitignoreEntry(dir, ".dmux/")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if !changed {
			t.Errorf("changed=false, want true")
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "node_modules/\n.dmux/\n" {
			t.Errorf("file body = %q, want %q", got, "node_modules/\n.dmux/\n")
		}
	})

	t.Run("returns error when parent dir is read-only", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".gitignore")
		if err := os.WriteFile(path, []byte("foo\n"), 0o444); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

		_, err := EnsureGitignoreEntry(dir, ".dmux/")
		if err == nil {
			t.Errorf("err=nil, want non-nil")
		}
	})
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd programs/claude-tools && go test ./internal/gitwt/ -run TestEnsureGitignoreEntry -v`
Expected: `undefined: EnsureGitignoreEntry`.

- [ ] **Step 3: Implement `EnsureGitignoreEntry`**

Update the import block at the top of `gitwt.go` to include `os`:

```go
import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"claude-tools/internal/proc"
)
```

Append to `gitwt.go` (after `DmuxWorktreeRoot`, before the `var (...)` regex block):

```go
// EnsureGitignoreEntry idempotently appends `line` to <repoRoot>/.gitignore.
// Returns changed=true if the file was modified.
//
// The match is exact on a single line (leading/trailing whitespace ignored).
// On any I/O error returns (changed=false, err).
//
// If the file does not exist it is created with mode 0644. If the file exists
// without a trailing newline, a newline is added before the appended line.
func EnsureGitignoreEntry(repoRoot, line string) (changed bool, err error) {
	path := filepath.Join(repoRoot, ".gitignore")
	existing, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return false, fmt.Errorf("read .gitignore: %w", readErr)
	}
	for _, l := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(l) == line {
			return false, nil
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, fmt.Errorf("open .gitignore: %w", err)
	}
	defer f.Close()
	var prefix string
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		prefix = "\n"
	}
	if _, err := f.WriteString(prefix + line + "\n"); err != nil {
		return false, fmt.Errorf("write .gitignore: %w", err)
	}
	return true, nil
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `cd programs/claude-tools && go test ./internal/gitwt/ -run TestEnsureGitignoreEntry -v`
Expected: 4 sub-tests PASS.

- [ ] **Step 5: Run full gitwt package + race + coverage**

Run: `cd programs/claude-tools && go test -race ./internal/gitwt/ -cover`
Expected: PASS, coverage ≥ 80% (was 74.6%).

- [ ] **Step 6: Commit**

```bash
git add programs/claude-tools/internal/gitwt/gitwt.go programs/claude-tools/internal/gitwt/gitwt_test.go
git commit -m "feat(gitwt): add EnsureGitignoreEntry for idempotent .dmux/ append"
```

---

## Task 4: Switch `claude-tmux-new` to dmux-compatible worktree base

**Goal:** Replace `~/.local/share/worktrees/<repo>/<branch>` with `<main-repo>/.dmux/worktrees/<flat-slug>`. Deprecate `--worktree-base` flag (warn-only). Auto-append `.dmux/` to main repo `.gitignore`.

**Files:**
- Modify: `programs/claude-tools/cmd/claude-tmux-new/main.go`
- Modify: `programs/claude-tools/cmd/claude-tmux-new/main_test.go`

- [ ] **Step 1: Update tests for the new behavior**

Replace the `TestParseArgs_table` `cases` slice in `programs/claude-tools/cmd/claude-tmux-new/main_test.go` (L9-26 of the existing file) with:

```go
	cases := []struct {
		name string
		argv []string
		want options
		err  bool
	}{
		{"branch only", []string{"feat/x"}, options{branch: "feat/x"}, false},
		{"--no-claude", []string{"feat/x", "--no-claude"}, options{branch: "feat/x", noClaude: true}, false},
		{"--from-root no id", []string{"feat/x", "--from-root"}, options{branch: "feat/x", fromRoot: true}, false},
		{"--from-root with id", []string{"feat/x", "--from-root", "abc-123"}, options{branch: "feat/x", fromRoot: true, explicitSession: "abc-123"}, false},
		{"--worktree-base deprecated (value ignored)", []string{"feat/x", "--worktree-base", "/tmp/wt"}, options{branch: "feat/x", worktreeBaseDeprecated: true}, false},
		{"--prompt", []string{"feat/x", "--prompt", "hi"}, options{branch: "feat/x", initialPrompt: "hi"}, false},
		{"missing branch", []string{}, options{}, true},
		{"--from-root + --no-claude", []string{"feat/x", "--from-root", "--no-claude"}, options{}, true},
		{"--prompt + --no-claude", []string{"feat/x", "--no-claude", "--prompt", "hi"}, options{}, true},
		{"--worktree-base missing arg", []string{"feat/x", "--worktree-base"}, options{}, true},
	}
```

- [ ] **Step 2: Run test to verify failure**

Run: `cd programs/claude-tools && go test ./cmd/claude-tmux-new/ -run TestParseArgs_table -v`
Expected: FAIL — struct has `worktreeBase string`, not `worktreeBaseDeprecated bool`.

- [ ] **Step 3: Update the `options` struct**

In `programs/claude-tools/cmd/claude-tmux-new/main.go`, replace the `options` struct (L36-43) with:

```go
type options struct {
	branch                 string
	fromRoot               bool
	noClaude               bool
	explicitSession        string
	worktreeBaseDeprecated bool // set when --worktree-base was passed; value is ignored with a warn
	initialPrompt          string
}
```

- [ ] **Step 4: Update `parseArgs` to handle the deprecated flag**

In `programs/claude-tools/cmd/claude-tmux-new/main.go`, replace the `--worktree-base` case in `parseArgs` (L84-90 in the existing file) with:

```go
		case "--worktree-base":
			i++
			if i >= len(argv) {
				return options{}, fmt.Errorf("--worktree-base requires a directory argument")
			}
			// Deprecated: value is ignored. New layout is <main-repo>/.dmux/worktrees/<flat-slug>.
			o.worktreeBaseDeprecated = true
			i++
```

- [ ] **Step 5: Run parseArgs test to verify pass**

Run: `cd programs/claude-tools && go test ./cmd/claude-tmux-new/ -run TestParseArgs_table -v`
Expected: PASS (10 cases).

- [ ] **Step 6: Update `resolveWorktree` to use new base path**

Replace `resolveWorktree` (L185-216) entirely with:

```go
func resolveWorktree(ctx context.Context, gw *gitwt.Client, opts options, mainRepo string) (string, error) {
	if existing, ok, err := gw.FindByBranch(ctx, mainRepo, opts.branch); err == nil && ok {
		return existing.Path, nil
	} else if err != nil {
		return "", fmt.Errorf("worktree list: %w", err)
	}

	worktree := filepath.Join(gitwt.DmuxWorktreeRoot(mainRepo), gitwt.SanitizeSlug(opts.branch))
	if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
		return "", fmt.Errorf("mkdir worktree parent: %w", err)
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
```

Update the caller at L135 (currently `resolveWorktree(ctx, gw, opts, mainRepo, repoBasename, safeBranch)`) to:

```go
	worktree, err := resolveWorktree(ctx, gw, opts, mainRepo)
```

The local `repoBasename` and `safeBranch` variables defined at L127 and L132 remain — `session` and `windowName` still depend on them.

- [ ] **Step 7: Wire `.gitignore` ensure + deprecation warn into `run()`**

In `run()`, immediately after `mainRepo` is resolved (insert after L125 of the existing file, between the `mainRepo` block and the `repoBasename` line), insert:

```go
	if opts.worktreeBaseDeprecated {
		fmt.Fprintln(os.Stderr, "claude-tmux-new: --worktree-base is deprecated; using <repo>/.dmux/worktrees/<slug> instead")
	}
	if changed, err := gitwt.EnsureGitignoreEntry(mainRepo, ".dmux/"); err != nil {
		logger.Warn("ensure .gitignore entry failed", "err", err)
	} else if changed {
		fmt.Fprintln(os.Stderr, `claude-tmux-new: appended ".dmux/" to .gitignore`)
	}
```

- [ ] **Step 8: Update package doc comment + `usageString`**

Replace the package doc comment at L1-7 of `main.go`:

```go
// claude-tmux-new creates (or attaches to) a tmux session+window pair backed
// by a git worktree at <main-repo>/.dmux/worktrees/<flat-slug>/, optionally
// starting `claude` in the right pane. Slug formatting matches dmux's
// sanitizeWorktreeSlugFromBranch().
//
// usage: claude-tmux-new <branch> [--from-root [<session-id>]] [--no-claude]
//                                 [--prompt <text>]
//                                 [--worktree-base <dir>] (deprecated, ignored)
package main
```

Replace `usageString()` (L32-34) body:

```go
func usageString() string {
	return "usage: claude-tmux-new <branch> [--from-root [<session-id>]] [--no-claude] [--prompt <text>] [--worktree-base <dir> (deprecated, ignored)]"
}
```

- [ ] **Step 9: Run all claude-tmux-new tests**

Run: `cd programs/claude-tools && go test -race ./cmd/claude-tmux-new/`
Expected: PASS.

- [ ] **Step 10: Run full Go test suite + race**

Run: `cd programs/claude-tools && go test -race ./...`
Expected: ALL PASS across all 24+ packages.

- [ ] **Step 11: `go vet`**

Run: `cd programs/claude-tools && go vet ./...`
Expected: no warnings.

- [ ] **Step 12: Commit**

```bash
git add programs/claude-tools/cmd/claude-tmux-new/main.go programs/claude-tools/cmd/claude-tmux-new/main_test.go
git commit -m "feat(claude-tmux-new): switch worktree base to <repo>/.dmux/worktrees/<slug>"
```

---

## Task 5: Add sanity check in `claude-kill-session`

**Goal:** Warn (but don't block) when worktree path is outside `<main-repo>/.dmux/worktrees/`.

**Files:**
- Modify: `programs/claude-tools/cmd/claude-kill-session/main.go` (insert before L75)

- [ ] **Step 1: Add the sanity check**

In `programs/claude-tools/cmd/claude-kill-session/main.go`, insert this block immediately before the existing `if wtRoot != "" && mainRepo != "" && wtRoot != mainRepo {` block (currently at L75):

```go
	// Sanity check: warn if worktree is outside <main-repo>/.dmux/worktrees/.
	// Do not block — legacy paths or hand-managed worktrees still need to be
	// cleanable.
	if wtRoot != "" && mainRepo != "" {
		dmuxRoot := gitwt.DmuxWorktreeRoot(mainRepo) + string(os.PathSeparator)
		if !strings.HasPrefix(wtRoot, dmuxRoot) {
			logger.Warn("worktree path is outside <main-repo>/.dmux/worktrees/, proceeding anyway",
				"wtRoot", wtRoot, "expected_prefix", dmuxRoot)
		}
	}
```

The imports at L8-20 already include `os`, `strings`, `claude-tools/internal/gitwt`. No import additions needed.

- [ ] **Step 2: Run claude-kill-session tests**

Run: `cd programs/claude-tools && go test -race ./cmd/claude-kill-session/`
Expected: PASS — the existing 5 cases of `TestIsClaudeManaged_table` are unaffected because they only exercise `isClaudeManaged`, not the new sanity-check block.

- [ ] **Step 3: Run full suite**

Run: `cd programs/claude-tools && go test -race ./...`
Expected: ALL PASS.

- [ ] **Step 4: Commit**

```bash
git add programs/claude-tools/cmd/claude-kill-session/main.go
git commit -m "feat(claude-kill-session): warn on worktree outside .dmux/worktrees/"
```

---

## Task 6: Update `/branch-out` slash command

**Goal:** Remove `--worktree-base` argument; refresh worktree path notes; document `.gitignore` auto-append.

**Files:**
- Modify: `programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-out.md`

- [ ] **Step 1: Update the `claude-tmux-new` invocation block**

In `programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-out.md`, replace the bash block at L48-52:

Old:
````
```bash
~/.local/bin/claude-tmux-new '<BRANCH>' \
  --worktree-base "${XDG_DATA_HOME:-$HOME/.local/share}/worktrees" \
  --prompt '<MESSAGE>'
```
````

New:
````
```bash
~/.local/bin/claude-tmux-new '<BRANCH>' \
  --prompt '<MESSAGE>'
```
````

- [ ] **Step 2: Update the 補足 section**

Replace the bullets at L67-71 (the entire 補足 list) with:

```markdown
- worktree 配置: `<main-repo-toplevel>/.dmux/worktrees/<sanitized-slug>` (dmux 互換、project-relative。slash はハイフンに sanitize される。アルゴリズムは dmux の `sanitizeWorktreeSlugFromBranch` と bit-exact)。
- 初回 `/branch-out` 実行時、main repo の `.gitignore` に `.dmux/` 行が無ければ自動追記される (1 回のみ、コミットはユーザに委ねる)。
- 同名ブランチが既に worktree として存在する場合は `claude-tmux-new` が既存 path を再利用 (新規作成はしない)。
- 新セッションは `claude '<MESSAGE>'` で起動するため、最初のメッセージとして依頼内容が pre-fill される。
- `--no-claude` 相当 (claude 抜きの worktree のみ作成) は本コマンドではサポートしない。必要なら直接 `claude-tmux-new <branch> --no-claude` を叩く。
```

- [ ] **Step 3: Verify the slash command file no longer references the deprecated flag**

Run: `grep -n -- '--worktree-base\|XDG_DATA_HOME.*worktrees' programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-out.md`
Expected: no hits.

- [ ] **Step 4: Commit**

```bash
git add programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-out.md
git commit -m "docs(branch-out): align slash command with dmux worktree layout"
```

---

## Task 7: Update `/branch-finish` and `/branch-merge` slash commands (verify only)

**Goal:** Refresh worktree path examples in the 補足 sections of both files if they reference the old `~/.local/share/worktrees/` path.

**Files:**
- Possibly modify: `programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-finish.md`
- Possibly modify: `programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-merge.md`

- [ ] **Step 1: Scan both files for outdated path references**

Run:
```bash
grep -n 'worktrees\|\.local/share\|\.dmux' \
  programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-finish.md \
  programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-merge.md
```

If no hits, skip Steps 2 and 3.

- [ ] **Step 2: Inline-replace any outdated paths**

For each hit pointing at `~/.local/share/worktrees/<repo>/<branch>`, replace with `<main-repo-toplevel>/.dmux/worktrees/<sanitized-slug>` keeping surrounding prose.

- [ ] **Step 3: Commit (skip if no diff)**

```bash
git diff programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-finish.md \
        programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-merge.md
```

If empty: skip. Otherwise:

```bash
git add programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-finish.md \
        programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-merge.md
git commit -m "docs(branch-finish,branch-merge): refresh worktree path notes"
```

---

## Task 8: Add dmux installation to `run_once_all_os.sh.cmd.tmpl`

**Goal:** Install `dmux` via mise global npm tool on Manjaro fresh-bootstrap.

**Files:**
- Modify: `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` (append after the TPM bootstrap block at L96)

- [ ] **Step 1: Append dmux install block**

Append to the end of `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` (after the existing TPM bootstrap block):

```sh

# dmux: tmux pane manager for AI agents (mise-managed npm global).
# Skips silently when mise is not on PATH (mise install runs earlier in this
# same script when missing — but a partial bootstrap is still tolerated here).
if ! command -v dmux >/dev/null 2>&1; then
  if command -v mise >/dev/null 2>&1; then
    mise use -g npm:dmux@latest
  else
    echo "[run_once_all_os] mise not found; skipping dmux install" >&2
  fi
fi
```

- [ ] **Step 2: Validate the template renders**

Run: `chezmoi execute-template < .chezmoiscripts/run_once_all_os.sh.cmd.tmpl | tail -20`
Expected: the new block renders without `{{` `}}` artifacts and the trailing TPM-bootstrap snippet is followed by the new dmux block.

- [ ] **Step 3: Verify `chezmoi diff` shows only the intended addition**

Run: `chezmoi diff .chezmoiscripts/run_once_all_os.sh.cmd.tmpl 2>&1 | head -30`
Expected: a +N-line diff at the tail of the file (chezmoi treats `.cmd.tmpl` as run-once, so `apply` will not re-execute on machines where it has already run).

- [ ] **Step 4: Commit**

```bash
git add .chezmoiscripts/run_once_all_os.sh.cmd.tmpl
git commit -m "chore(chezmoi): install dmux via mise global npm on fresh bootstrap"
```

---

## Task 9: Add dmux presence check in `run_onchange_after_build-claude-tools.sh.tmpl`

**Goal:** Informational-only check that dmux is reachable; never errors.

**Files:**
- Modify: `.chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl` (append at tail)

- [ ] **Step 1: Inspect current tail to find safe append point**

Run: `tail -20 .chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl`
Expected: identify any final `exit 0` or last command. Append just before it (or at the end if none).

- [ ] **Step 2: Append the check**

Append (or insert before the final `exit 0` if present):

```sh

# Informational: warn if dmux is not yet installed. /branch-out works
# without it (standalone direct mode); the warning just hints users to
# install dmux for the optional UI.
if ! command -v dmux >/dev/null 2>&1; then
  echo "[chezmoi] dmux not installed; /branch-out works in standalone mode (run 'mise use -g npm:dmux@latest' to install)" >&2
fi
```

- [ ] **Step 3: Validate template renders**

Run: `chezmoi execute-template < .chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl | tail -10`
Expected: the snippet renders cleanly.

- [ ] **Step 4: Commit**

```bash
git add .chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl
git commit -m "chore(chezmoi): informational dmux presence check after Go build"
```

---

## Task 10: Update `programs/claude-tools/README.md`

**Goal:** Refresh the `claude-tmux-new` row note and add a brief "dmux Compatibility" section.

**Files:**
- Modify: `programs/claude-tools/README.md`

- [ ] **Step 1: Edit the README row**

In `programs/claude-tools/README.md`, find the `claude-tmux-new` row of the binary table near the top:

Old:
```
| `claude-tmux-new` | `/branch-out` slash command, `prefix + C` ⇒ `n` / `o` | spawn worktree + window + child claude |
```

New:
```
| `claude-tmux-new` | `/branch-out` slash command, `prefix + C` ⇒ `n` / `o` | spawn worktree at `<repo>/.dmux/worktrees/<slug>/` + window + child claude (dmux-compatible layout) |
```

- [ ] **Step 2: Append a "dmux Compatibility" note section**

Append to `programs/claude-tools/README.md` at the end:

```markdown

## dmux Compatibility (2026-05-05 onward)

`claude-tmux-new` writes worktrees to `<main-repo>/.dmux/worktrees/<slug>/` so
that the [dmux](https://github.com/standardagents/dmux) TUI can manage them
when invoked. Branch naming (`<type>/<kebab>`) is preserved and the directory
slug is computed via `gitwt.SanitizeSlug`, a bit-exact Go port of dmux's
`sanitizeWorktreeSlugFromBranch()` (`src/utils/paneNaming.ts`). Standalone
operation does not require dmux to be installed; mise-managed install is
configured via `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl`.

Design spec: `docs/superpowers/specs/2026-05-05-dmux-migration-design.md`.
Plan: `docs/superpowers/plans/2026-05-05-dmux-migration.md`.
```

- [ ] **Step 3: Commit**

```bash
git add programs/claude-tools/README.md
git commit -m "docs(claude-tools): note dmux-compatible worktree layout"
```

---

## Task 11: Create smoke test checklist

**Goal:** A manual checklist for the 6 smoke items in spec §9.4.

**Files:**
- Create: `docs/superpowers/smoke/2026-05-05-dmux-migration-smoke.md`

- [ ] **Step 1: Write the smoke checklist**

Create `docs/superpowers/smoke/2026-05-05-dmux-migration-smoke.md` with this content:

````markdown
# dmux Migration — Manual Smoke Checklist (2026-05-05)

Spec: `docs/superpowers/specs/2026-05-05-dmux-migration-design.md` §9.4
Plan: `docs/superpowers/plans/2026-05-05-dmux-migration.md`

## Preconditions

- Active session inside tmux (any session)
- Working tree of chezmoi repo at `~/.local/share/chezmoi/` is clean (or only
  this plan's commits applied)
- `claude` CLI is on PATH
- `dmux` is optional — items 1-5 must work without it

## Items

### 1. `/branch-out` creates worktree at the dmux-compatible path

```
/branch-out "test dmux migration smoke"
```

Expected:
- A new tmux window is spawned and focused
- `git worktree list` (run in the new window's left pane) shows
  `~/.local/share/chezmoi/.dmux/worktrees/feat-test-dmux-migration-smoke`
  as a worktree
- The new pane runs `claude` and the message `test dmux migration smoke` is
  visible in the prompt area (pre-fill)

### 2. `.gitignore` has `.dmux/` appended automatically

```
grep -nE '^\.dmux/?$' ~/.local/share/chezmoi/.gitignore
```

Expected: 1 match. The change is uncommitted (user will commit when ready).

### 3. Pre-fill prompt is exact

In the new pane (claude), the first message displayed must be exactly
`test dmux migration smoke` — not a quoted variant, not truncated.

### 4. `/branch-merge main fetch` integrates back

In the spawned pane:
```
/branch-merge main fetch
```

Expected:
- `claude-branch-merge` outputs `merged feat/test-dmux-migration-smoke into main at /home/kiyama/.local/share/chezmoi`
- `git -C ~/.local/share/chezmoi log --oneline -1` shows the merged commit on `main`

### 5. `/branch-finish` removes worktree + window

In the spawned pane:
```
/branch-finish
```

Expected:
- The tmux window disappears (focus returns to the previous window)
- `git worktree list` no longer lists the spawned worktree
- `~/.local/share/chezmoi/.dmux/worktrees/feat-test-dmux-migration-smoke/` no
  longer exists

### 6. `dmux` UI sees existing worktrees (failure tolerated)

```
cd ~/.local/share/chezmoi
/branch-out "test dmux ui pickup"
# In a separate terminal:
cd ~/.local/share/chezmoi
dmux
```

Expected:
- The dmux TUI lists `feat-test-dmux-ui-pickup` as a manageable pane
- If it does not, file a follow-up under spec §11 D-1 / D-6 — but this is not
  a blocker for accepting the migration

After verifying, run `/branch-finish` from the spawned pane.

## Sign-off

When items 1-5 PASS, append to this file:

```
<!-- Smoke verified by kiyama on YYYY-MM-DD; item 6 result: PASS / FAIL / N/A -->
```
````

- [ ] **Step 2: Commit**

```bash
git add docs/superpowers/smoke/2026-05-05-dmux-migration-smoke.md
git commit -m "docs(smoke): add dmux migration manual smoke checklist"
```

---

## Task 12: Run smoke test items 1–5 and update todos.md

**Goal:** Execute the smoke test, then mark F-7 in `todos.md` as dmux-aligned.

**Files:**
- Modify: `docs/todos.md` (F-7 entry, around L155)
- Modify: `docs/superpowers/smoke/2026-05-05-dmux-migration-smoke.md` (sign-off line)

- [ ] **Step 1: Apply chezmoi to install new binaries**

Run: `chezmoi diff` (review carefully) → `chezmoi apply`.

Expected: Go binaries are rebuilt by `run_onchange_after_build-claude-tools.sh.tmpl`, slash command markdown is synced to the plugin cache, and (on a fresh Manjaro machine) dmux is installed by `run_once_all_os.sh.cmd.tmpl`. On the existing dev machine where `run_once` already ran, install dmux manually:

```bash
mise use -g npm:dmux@latest || true
```

- [ ] **Step 2: Walk through smoke items 1-5**

Follow `docs/superpowers/smoke/2026-05-05-dmux-migration-smoke.md` items 1 through 5 in order. For each, paste the actual tmux/git outputs into a scratch buffer for the commit message body.

If any item fails, STOP and triage. Do not proceed.

- [ ] **Step 3: Attempt smoke item 6 (dmux UI pickup)**

Run item 6. If dmux is installed and recognizes the worktree → mark PASS. If not → mark FAIL.

- [ ] **Step 4: Add sign-off comment to smoke doc**

Append to `docs/superpowers/smoke/2026-05-05-dmux-migration-smoke.md`:

```markdown
<!-- Smoke verified by kiyama on 2026-05-05; item 6 result: PASS / FAIL / N/A — annotate one -->
```

Replace `PASS / FAIL / N/A — annotate one` with the actual result of step 3 (a single token).

- [ ] **Step 5: Update `docs/todos.md` F-7 entry**

Find the F-7 entry (around L155, heading `### F-7. /branch-out worktree spawner`) and append a sub-bullet at the end of its bullet list:

```markdown
  - [x] **dmux 互換化** (2026-05-05): worktree base を `<repo>/.dmux/worktrees/<sanitized-slug>` に切替、`internal/gitwt.SanitizeSlug` を上流 dmux の `sanitizeWorktreeSlugFromBranch` の bit-exact Go 移植として追加、`/branch-out` から `--worktree-base` を削除、`.gitignore` に `.dmux/` 自動冪等追記、Manjaro bootstrap で `mise use -g npm:dmux@latest`。spec: `docs/superpowers/specs/2026-05-05-dmux-migration-design.md` / plan: `docs/superpowers/plans/2026-05-05-dmux-migration.md` / smoke: `docs/superpowers/smoke/2026-05-05-dmux-migration-smoke.md`
```

- [ ] **Step 6: Commit**

```bash
git add docs/todos.md docs/superpowers/smoke/2026-05-05-dmux-migration-smoke.md
git commit -m "docs(todos): mark F-7 as dmux-aligned; smoke verified"
```

---

## Final verification

- [ ] **Step 1: Re-run the entire Go test suite with race + coverage**

Run: `cd programs/claude-tools && go test -race -cover ./...`
Expected: ALL packages PASS. Coverage: `internal/gitwt` ≥ 80% (was 74.6%).

- [ ] **Step 2: Run go vet**

Run: `cd programs/claude-tools && go vet ./...`
Expected: no warnings.

- [ ] **Step 3: chezmoi-side sanity**

Run: `chezmoi verify`
Expected: clean, no template errors.

- [ ] **Step 4: Mark spec as Implemented**

In `docs/superpowers/specs/2026-05-05-dmux-migration-design.md` L3, replace:

Old:
```
- **Status**: Draft (brainstorming complete, awaiting user review)
```

New:
```
- **Status**: Implemented (2026-05-05)
```

- [ ] **Step 5: Final commit**

```bash
git add docs/superpowers/specs/2026-05-05-dmux-migration-design.md
git commit -m "docs(spec): mark dmux migration design as implemented"
```

---

## Self-Review Checklist (post-implementation)

- [ ] All 11 acceptance criteria in spec §10 pass
- [ ] `go test -race ./...` is green
- [ ] `chezmoi diff` is empty after `chezmoi apply`
- [ ] Smoke test items 1-5 PASS (item 6 documented as PASS / FAIL / N/A)
- [ ] `docs/todos.md` F-7 entry has the dmux-互換化 follow-up
