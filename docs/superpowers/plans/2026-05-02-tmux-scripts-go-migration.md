# tmux Scripts → Go Migration (C Subsystem 前半) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Phase 1/2 で確立した `programs/claude-tools/` 上に、`dot_config/tmux/scripts/` 配下の対話 UI / git worktree 操作系 shell **5 本** を Go binary で 1:1 置換する。`tpm-bootstrap.sh` は D subsystem へ繰延 (本 plan の対象外)。

**Architecture:** 新規 `internal/tmux` (tmux command wrapper) と `internal/gitwt` (git worktree porcelain parser + ops) を追加し、5 cmd binary は両 internal package の thin entry point として実装。**PR 1 本につき shell 1 本 → Go 1 本 atomic swap**。tmux window options / cockpit cache パスは shell 時代と完全互換。

**Tech Stack:**
- Go 1.22+ (Phase 1/2 と同じ、mise 管理)
- `log/slog` + `logger -t` (`internal/obslog` 既存)
- `os/exec` + `syscall.Exec` for fzf interactive / tmux attach (Phase 1 switcher と同パターン)
- 新規外部依存なし

**Spec:** [`../specs/2026-05-02-tmux-scripts-go-migration-design.md`](../specs/2026-05-02-tmux-scripts-go-migration-design.md)

**Phase 1 / 2 plan (前提):**
- [`./2026-05-01-shell-to-go-migration.md`](./2026-05-01-shell-to-go-migration.md) (B subsystem)
- [`./2026-05-02-shell-to-go-migration-phase2.md`](./2026-05-02-shell-to-go-migration-phase2.md) (A subsystem)

---

## File Structure

### 新規作成 (本 plan 内で生成)

| パス | 役割 | PR |
|---|---|---|
| `programs/claude-tools/internal/gitwt/gitwt.go` | `Worktree` struct / `Client` (proc.Runner wrapper) / parsePorcelain / CurrentBranch / TopLevel / MainRepo / FindByBranch / LocalBranches / HasLocalRef / HasRemoteRef / Add{ExistingLocal,TrackingRemote,FromHead} / Remove / Prune | C-1 (skeleton + CurrentBranch) → C-3 / C-4 / C-5 で拡張 |
| `programs/claude-tools/internal/gitwt/gitwt_test.go` | parsePorcelain 9 case + CurrentBranch + 各 Add 系 argv 検証 | 同上 |
| `programs/claude-tools/internal/tmux/tmux.go` | `Client` (proc.Runner wrapper) / Display / HasSession / SetWindowOption / ShowWindowOption / ListPanes / DisplayMessageGet / NewSessionDetached / NewWindowSelectExisting / SplitWindowH / SelectPaneTitle / SendKeys / SwitchClient / AttachSessionExec / KillWindow / RespawnPaneKill / Sanitize / ShellQuote | C-2 (5 method + Sanitize/ShellQuote) → C-3 / C-4 で拡張 |
| `programs/claude-tools/internal/tmux/tmux_test.go` | 各 method の argv 期待値 + Sanitize 6 case + ShellQuote 7 case (round-trip 1 件含む) | 同上 |
| `programs/claude-tools/cmd/claude-branch/main.go` | `[<branch>] ` 出力、status-right safe (常に exit 0) | C-1 |
| `programs/claude-tools/cmd/claude-branch/main_test.go` | 引数なし / git 不在 / 通常 / panic recover の 4 case | C-1 |
| `programs/claude-tools/cmd/claude-respawn-pane/main.go` | claude pane 検出 → respawn-pane -k → send-keys claude --continue | C-2 |
| `programs/claude-tools/cmd/claude-respawn-pane/main_test.go` | claude pane 存在 / 不在 / send-keys argv | C-2 |
| `programs/claude-tools/cmd/claude-kill-session/main.go` | 安全チェック 3 段 + worktree remove + cache cleanup | C-3 |
| `programs/claude-tools/cmd/claude-kill-session/main_test.go` | 安全チェック matrix / fallback path / cache cleanup | C-3 |
| `programs/claude-tools/cmd/claude-tmux-new/main.go` | argparse + worktree resolve + session/window create + 2-pane split + claude起動 + switch/attach | C-4 |
| `programs/claude-tools/cmd/claude-tmux-new/main_test.go` | argparse + worktree 3 分岐 + send-keys ShellQuote 検証 | C-4 |
| `programs/claude-tools/cmd/claude-pick-branch/main.go` | fzf wrapper + buildExecArgs + syscall.Exec | C-5 |
| `programs/claude-tools/cmd/claude-pick-branch/main_test.go` | buildExecArgs argv / fzf 不在 / 候補 0 | C-5 |
| `docs/superpowers/smoke/2026-05-02-go-tmux-scripts-smoke.md` | smoke 結果記録 (5 領域 + 中間) | 各 PR |

### 変更 (本 plan 内、各 PR)

| パス | 変更内容 | PR |
|---|---|---|
| `programs/claude-tools/README.md` | "Phase 2 で `internal/notify` を追加予定" → 実装済み記述 + `internal/{tmux,gitwt}` 言及追加 | C-1 |
| `dot_config/tmux/conf/status.conf` L11 | `~/.config/tmux/scripts/claude-branch.sh` → `~/.local/bin/claude-branch` | C-1 |
| `dot_config/tmux/conf/bindings.conf` L71 | `~/.config/tmux/scripts/claude-respawn-pane.sh` → `~/.local/bin/claude-respawn-pane` | C-2 |
| `dot_config/tmux/conf/bindings.conf` L86 | `~/.config/tmux/scripts/claude-kill-session.sh` → `~/.local/bin/claude-kill-session` | C-3 |
| `dot_config/tmux/conf/bindings.conf` L61, L66 | `~/.config/tmux/scripts/claude-pick-branch.sh` → `~/.local/bin/claude-pick-branch` | C-5 |
| `docs/todos.md` G セクション | C-1〜C-5 を順次 `[x]` に。完走後 D subsystem follow-up を記載 | 各 PR |

### 削除 (本 plan 内、各 PR で `git rm`)

| パス | PR |
|---|---|
| `dot_config/tmux/scripts/executable_claude-branch.sh` | C-1 |
| `dot_config/tmux/scripts/executable_claude-respawn-pane.sh` | C-2 |
| `dot_config/tmux/scripts/executable_claude-kill-session.sh` | C-3 |
| `dot_config/tmux/scripts/executable_tmux-claude-new.sh` | C-4 |
| `dot_config/tmux/scripts/executable_claude-pick-branch.sh` | C-5 |

---

## Tasks

### Task 0: Prerequisites

- [ ] **Step 0.1: Working tree clean確認**

```bash
git status
```

Expected: `nothing to commit, working tree clean` (または spec/plan のみ uncommitted)。HEAD は develop の Phase 2 完了済 (`bb43617` 以降) 系列。

- [ ] **Step 0.2: Phase 2 baseline 緑確認**

```bash
cd programs/claude-tools && go test -race ./...
```

Expected: 全 PASS。失敗していれば本 plan に進まず Phase 2 の plan に戻る。

- [ ] **Step 0.3: 環境確認**

```bash
command -v fzf && tmux -V && command -v go
```

Expected: fzf path, tmux 3.6+, go 1.22+ が表示。

---

### Task 1: PR-C-1 — `claude-branch` + `internal/gitwt` 初期化

**Files:**
- Create: `programs/claude-tools/internal/gitwt/gitwt.go`
- Create: `programs/claude-tools/internal/gitwt/gitwt_test.go`
- Create: `programs/claude-tools/cmd/claude-branch/main.go`
- Create: `programs/claude-tools/cmd/claude-branch/main_test.go`
- Modify: `dot_config/tmux/conf/status.conf:11`
- Modify: `programs/claude-tools/README.md` (Phase 進捗)
- Delete: `dot_config/tmux/scripts/executable_claude-branch.sh`

#### TDD steps

- [ ] **Step 1.1: `internal/gitwt/gitwt.go` skeleton 作成 (CurrentBranch のみ)**

```go
// Package gitwt wraps git worktree / branch operations behind a proc.Runner
// for testability. Initial set: CurrentBranch only; later PRs add the
// ListPorcelain parser and worktree mutation methods.
package gitwt

import (
	"context"
	"strings"

	"claude-tools/internal/proc"
)

// Worktree represents one entry from `git worktree list --porcelain`.
type Worktree struct {
	Path   string
	Branch string // "refs/heads/<x>" の <x> 部分。detached は ""
	HEAD   string
}

// Client wraps proc.Runner for git invocations.
type Client struct{ runner proc.Runner }

// New returns a Client backed by the given runner.
func New(r proc.Runner) *Client { return &Client{runner: r} }

// CurrentBranch returns the current branch of the working tree at cwd.
// Returns ("", nil) if HEAD is detached or git fails (caller decides handling).
func (c *Client) CurrentBranch(ctx context.Context, cwd string) (string, error) {
	out, err := c.runner.Run(ctx, "git", "-C", cwd, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
```

- [ ] **Step 1.2: `internal/gitwt/gitwt_test.go` で CurrentBranch の RED テスト**

```go
package gitwt

import (
	"context"
	"testing"

	"claude-tools/internal/proc"
)

func TestCurrentBranch_returnsTrimmedBranch(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/tmp/repo", "branch", "--show-current"}, []byte("feat/x\n"), nil)
	got, err := New(r).CurrentBranch(context.Background(), "/tmp/repo")
	if err != nil || got != "feat/x" {
		t.Fatalf("got=%q err=%v, want feat/x nil", got, err)
	}
}

func TestCurrentBranch_returnsEmptyOnDetached(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/tmp/repo", "branch", "--show-current"}, []byte(""), nil)
	got, err := New(r).CurrentBranch(context.Background(), "/tmp/repo")
	if err != nil || got != "" {
		t.Fatalf("got=%q err=%v, want empty nil", got, err)
	}
}
```

- [ ] **Step 1.3: テスト pass 確認**

```bash
cd programs/claude-tools && go test -race ./internal/gitwt/...
```

Expected: 2 PASS。

- [ ] **Step 1.4: `cmd/claude-branch/main_test.go` で挙動 RED テスト**

```go
package main

import (
	"context"
	"testing"

	"claude-tools/internal/proc"
)

func TestFormatBranch_emptyCwd(t *testing.T) {
	got, _ := formatBranch(context.Background(), proc.NewFakeRunner(), "")
	if got != "" {
		t.Fatalf("empty cwd: got %q want empty", got)
	}
}

func TestFormatBranch_normalCase(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/tmp/repo", "branch", "--show-current"}, []byte("main\n"), nil)
	got, err := formatBranch(context.Background(), r, "/tmp/repo")
	if err != nil || got != "[main] " {
		t.Fatalf("got=%q err=%v, want '[main] '", got, err)
	}
}

func TestFormatBranch_gitFailure_returnsEmpty(t *testing.T) {
	r := proc.NewFakeRunner() // unregistered → returns error
	got, err := formatBranch(context.Background(), r, "/tmp/repo")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got != "" {
		t.Fatalf("got=%q want empty", got)
	}
}

func TestFormatBranch_detachedHEAD(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/tmp/repo", "branch", "--show-current"}, []byte(""), nil)
	got, err := formatBranch(context.Background(), r, "/tmp/repo")
	if err != nil || got != "" {
		t.Fatalf("got=%q err=%v, want empty nil", got, err)
	}
}
```

- [ ] **Step 1.5: `cmd/claude-branch/main.go` 実装**

```go
// claude-branch prints "[<branch>] " for use in tmux status-right.
// Always exit 0 to keep status-right rendering safe even on errors
// (status-right invokes this on every refresh; a non-zero exit would
// break the entire status line).
package main

import (
	"context"
	"fmt"
	"os"

	"claude-tools/internal/gitwt"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
)

const progName = "claude-branch"

var logger = obslog.New(progName)

func main() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic recovered", "panic", fmt.Sprint(r))
		}
		os.Exit(0)
	}()

	cwd := ""
	if len(os.Args) > 1 {
		cwd = os.Args[1]
	}
	out, err := formatBranch(context.Background(), proc.RealRunner{}, cwd)
	if err != nil {
		logger.Debug("branch lookup failed", "cwd", cwd, "err", err)
		return
	}
	fmt.Print(out)
}

// formatBranch is the testable core: returns the formatted status fragment
// or an empty string on any non-fatal condition.
func formatBranch(ctx context.Context, r proc.Runner, cwd string) (string, error) {
	if cwd == "" {
		return "", nil
	}
	branch, err := gitwt.New(r).CurrentBranch(ctx, cwd)
	if err != nil {
		return "", err
	}
	if branch == "" {
		return "", nil
	}
	return fmt.Sprintf("[%s] ", branch), nil
}
```

- [ ] **Step 1.6: テスト pass 確認**

```bash
cd programs/claude-tools && go test -race ./cmd/claude-branch/... ./internal/gitwt/...
```

Expected: 全 PASS (gitwt 2 + claude-branch 4)。

- [ ] **Step 1.7: `dot_config/tmux/conf/status.conf` L11 書き換え**

旧:
```
set -g status-right "#(~/.local/bin/claude-cockpit-summary)#(~/.config/tmux/scripts/claude-branch.sh '#{pane_current_path}')%H:%M "
```

新:
```
set -g status-right "#(~/.local/bin/claude-cockpit-summary)#(~/.local/bin/claude-branch '#{pane_current_path}')%H:%M "
```

- [ ] **Step 1.8: `programs/claude-tools/README.md` 進捗表記更新**

`internal/{cockpit,xdg,atomicfile,proc,obslog}` の行を以下で置換:

```
- `internal/{cockpit,xdg,atomicfile,proc,obslog,notify,notifyd,gitwt}` — 共有パッケージ
  (Phase 3 で `internal/tmux` を順次追加)
```

- [ ] **Step 1.9: 旧 shell 削除**

```bash
git rm dot_config/tmux/scripts/executable_claude-branch.sh
```

- [ ] **Step 1.10: 全体 build 確認**

```bash
cd programs/claude-tools && go test -race ./... && go build ./cmd/claude-branch
```

Expected: 全 PASS、binary が programs/claude-tools/ 直下に生成 (削除しても問題ない、build 確認のみ)。

- [ ] **Step 1.11: smoke 実機**

```bash
chezmoi diff   # ~/.local/bin/claude-branch 増 + status.conf 変更を確認
chezmoi apply
tmux source-file ~/.config/tmux/tmux.conf  # status-right 反映
```

git worktree 内 / 非 git ディレクトリ で tmux pane を切替えて status-right の `[<branch>] ` 部分が正しく出る/消えることを目視。

- [ ] **Step 1.12: `docs/superpowers/smoke/2026-05-02-go-tmux-scripts-smoke.md` 新設 + C-1 結果記録**

```markdown
# C subsystem (tmux scripts) Go migration smoke log

## C-1: claude-branch — 2026-05-02

- [x] git worktree 内: `[feat/c-subsystem-design] ` 表示 OK
- [x] 非 git cwd: 空文字列 OK
- [x] worktree 切替で追従 OK
```

- [ ] **Step 1.13: commit**

```bash
git add programs/claude-tools/internal/gitwt/ \
        programs/claude-tools/cmd/claude-branch/ \
        programs/claude-tools/README.md \
        dot_config/tmux/conf/status.conf \
        docs/superpowers/smoke/2026-05-02-go-tmux-scripts-smoke.md
git rm dot_config/tmux/scripts/executable_claude-branch.sh
git commit -m "$(cat <<'EOF'
feat(g2): C-1 replace claude-branch.sh with Go binary + internal/gitwt skeleton

- introduce internal/gitwt with CurrentBranch (rest of the package
  surface lands in C-3 / C-4)
- claude-branch always exits 0 (status-right safety) and prints
  "[<branch>] " when inside a git repo, empty otherwise
- tmux status.conf updated to call ~/.local/bin/claude-branch
EOF
)"
```

- [ ] **Step 1.14: `docs/todos.md` G セクションに C-1 完了の `[x]` 追加** (G-2 セクション新設、無ければ G-1 末尾に追記。フォーマットは既存セクション踏襲)。

---

### Task 2: PR-C-2 — `claude-respawn-pane` + `internal/tmux` 初期化

**Files:**
- Create: `programs/claude-tools/internal/tmux/tmux.go`
- Create: `programs/claude-tools/internal/tmux/tmux_test.go`
- Create: `programs/claude-tools/cmd/claude-respawn-pane/main.go`
- Create: `programs/claude-tools/cmd/claude-respawn-pane/main_test.go`
- Modify: `dot_config/tmux/conf/bindings.conf:71`
- Delete: `dot_config/tmux/scripts/executable_claude-respawn-pane.sh`

#### TDD steps

- [ ] **Step 2.1: `internal/tmux/tmux.go` skeleton (Display, ListPanes, DisplayMessageGet, RespawnPaneKill, SendKeys, Sanitize, ShellQuote)**

```go
// Package tmux wraps tmux command invocations behind a proc.Runner so cmds
// can be unit-tested by injecting FakeRunner. Sanitize / ShellQuote are
// package-level helpers shared across all cmds.
package tmux

import (
	"context"
	"regexp"
	"strings"

	"claude-tools/internal/proc"
)

// Client wraps proc.Runner for tmux invocations.
type Client struct{ runner proc.Runner }

// New returns a Client backed by the given runner.
func New(r proc.Runner) *Client { return &Client{runner: r} }

// Display posts a short message to the status line. Failures are swallowed.
func (c *Client) Display(ctx context.Context, msg string) {
	_, _ = c.runner.Run(ctx, "tmux", "display-message", msg)
}

// ListPanes runs `tmux list-panes -t <target> -F <format>` and returns
// the trimmed lines.
func (c *Client) ListPanes(ctx context.Context, target, format string) ([]string, error) {
	out, err := c.runner.Run(ctx, "tmux", "list-panes", "-t", target, "-F", format)
	if err != nil {
		return nil, err
	}
	s := strings.TrimRight(string(out), "\n")
	if s == "" {
		return nil, nil
	}
	return strings.Split(s, "\n"), nil
}

// DisplayMessageGet runs `tmux display-message -p [-t <target>] <format>`.
// If target is "", -t flag is omitted.
func (c *Client) DisplayMessageGet(ctx context.Context, target, format string) (string, error) {
	args := []string{"display-message", "-p"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, format)
	out, err := c.runner.Run(ctx, "tmux", args...)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// RespawnPaneKill runs `tmux respawn-pane -k -t <target>`.
func (c *Client) RespawnPaneKill(ctx context.Context, target string) error {
	_, err := c.runner.Run(ctx, "tmux", "respawn-pane", "-k", "-t", target)
	return err
}

// SendKeys runs `tmux send-keys -t <target> <keys...>`. The caller appends
// "Enter" / "C-m" etc. as a separate key argument when needed.
func (c *Client) SendKeys(ctx context.Context, target string, keys ...string) error {
	args := append([]string{"send-keys", "-t", target}, keys...)
	_, err := c.runner.Run(ctx, "tmux", args...)
	return err
}

// sanitizeRe matches characters NOT allowed in tmux session/window names.
var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// Sanitize replaces every character outside [a-zA-Z0-9._-] with '-'.
// Mirrors the shell `tr -c 'a-zA-Z0-9._-' '-'` behaviour.
func Sanitize(s string) string {
	return sanitizeRe.ReplaceAllString(s, "-")
}

// ShellQuote returns a POSIX single-quoted form of s. Empty string becomes
// '' and any internal ' becomes '\''. Output is byte-different from
// `printf %q` but is semantically equivalent: feeding it back through bash
// reproduces the original string.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
```

- [ ] **Step 2.2: `internal/tmux/tmux_test.go` で argv + Sanitize + ShellQuote の RED テスト**

```go
package tmux

import (
	"context"
	"os/exec"
	"testing"

	"claude-tools/internal/proc"
)

func TestDisplay_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"display-message", "hello"}, nil, nil)
	New(r).Display(context.Background(), "hello") // must not panic
}

func TestListPanes_splitsLines(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"list-panes", "-t", "S:W", "-F", "#{pane_id}"}, []byte("%1\n%2\n"), nil)
	got, err := New(r).ListPanes(context.Background(), "S:W", "#{pane_id}")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 || got[0] != "%1" || got[1] != "%2" {
		t.Fatalf("got %v", got)
	}
}

func TestListPanes_emptyOutput_returnsNil(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"list-panes", "-t", "S", "-F", "."}, []byte(""), nil)
	got, _ := New(r).ListPanes(context.Background(), "S", ".")
	if got != nil {
		t.Fatalf("got %v want nil", got)
	}
}

func TestDisplayMessageGet_withAndWithoutTarget(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"display-message", "-p", "-t", "S:W", "#S"}, []byte("S\n"), nil)
	r.Register("tmux", []string{"display-message", "-p", "#S"}, []byte("X\n"), nil)
	c := New(r)
	if got, _ := c.DisplayMessageGet(context.Background(), "S:W", "#S"); got != "S" {
		t.Fatalf("with target: got %q", got)
	}
	if got, _ := c.DisplayMessageGet(context.Background(), "", "#S"); got != "X" {
		t.Fatalf("without target: got %q", got)
	}
}

func TestRespawnPaneKill_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"respawn-pane", "-k", "-t", "%3"}, nil, nil)
	if err := New(r).RespawnPaneKill(context.Background(), "%3"); err != nil {
		t.Fatal(err)
	}
}

func TestSendKeys_appendsKeys(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"send-keys", "-t", "%3", "claude --continue", "Enter"}, nil, nil)
	if err := New(r).SendKeys(context.Background(), "%3", "claude --continue", "Enter"); err != nil {
		t.Fatal(err)
	}
}

func TestSanitize_table(t *testing.T) {
	cases := []struct{ in, want string }{
		{"feat/x", "feat-x"},
		{"abc.def_ghi-jkl", "abc.def_ghi-jkl"},
		{"hello world!", "hello-world-"},
		{"", ""},
		{"a//b", "a--b"},
	}
	for _, c := range cases {
		if got := Sanitize(c.in); got != c.want {
			t.Errorf("Sanitize(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestShellQuote_table(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "''"},
		{"plain", "'plain'"},
		{"with space", "'with space'"},
		{"it's", `'it'\''s'`},
		{"$HOME", "'$HOME'"},
		{`a\b`, `'a\b'`},
		{"line\nbreak", "'line\nbreak'"},
	}
	for _, c := range cases {
		if got := ShellQuote(c.in); got != c.want {
			t.Errorf("ShellQuote(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

// TestShellQuote_roundTrip feeds the quoted form through bash and verifies
// the original bytes round-trip. Skips if bash is unavailable.
func TestShellQuote_roundTrip(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	for _, in := range []string{"hi", "it's", "$X", `a\b`, "テスト 'quote'"} {
		quoted := ShellQuote(in)
		out, err := exec.Command("bash", "-c", "printf '%s' "+quoted).Output()
		if err != nil {
			t.Fatalf("bash failed for %q: %v", in, err)
		}
		if got := string(out); got != in {
			t.Errorf("roundtrip mismatch: in=%q quoted=%s out=%q", in, quoted, got)
		}
	}
}
```

- [ ] **Step 2.3: テスト pass 確認**

```bash
cd programs/claude-tools && go test -race ./internal/tmux/...
```

Expected: 全 PASS。

- [ ] **Step 2.4: `cmd/claude-respawn-pane/main_test.go` で挙動 RED テスト**

```go
package main

import (
	"context"
	"testing"

	"claude-tools/internal/proc"
)

func TestPickTargetPane_findsClaudePane(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"display-message", "-p", "#S"}, []byte("dev\n"), nil)
	r.Register("tmux", []string{"list-panes", "-t", "dev", "-F", "#{pane_id} #{pane_current_command}"},
		[]byte("%1 zsh\n%2 claude\n"), nil)
	got, err := pickTargetPane(context.Background(), r)
	if err != nil || got != "%2" {
		t.Fatalf("got=%q err=%v want %2", got, err)
	}
}

func TestPickTargetPane_fallsBackToCurrentPane(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"display-message", "-p", "#S"}, []byte("dev\n"), nil)
	r.Register("tmux", []string{"list-panes", "-t", "dev", "-F", "#{pane_id} #{pane_current_command}"},
		[]byte("%1 zsh\n%2 vim\n"), nil)
	r.Register("tmux", []string{"display-message", "-p", "#{pane_id}"}, []byte("%1\n"), nil)
	got, err := pickTargetPane(context.Background(), r)
	if err != nil || got != "%1" {
		t.Fatalf("got=%q err=%v want %1", got, err)
	}
}
```

- [ ] **Step 2.5: `cmd/claude-respawn-pane/main.go` 実装**

```go
// claude-respawn-pane restarts claude in the current session's claude pane.
// Strategy: find a pane in the current session whose pane_current_command
// is 'claude'; if found, respawn-pane -k and start a fresh
// `claude --continue`. If none found, do it in the current pane.
package main

import (
	"context"
	"os"
	"strings"

	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
	"claude-tools/internal/tmux"
)

const progName = "claude-respawn-pane"

var logger = obslog.New(progName)

func main() {
	ctx := context.Background()
	r := proc.RealRunner{}
	target, err := pickTargetPane(ctx, r)
	if err != nil {
		logger.Error("pick target pane failed", "err", err)
		os.Exit(1)
	}
	tc := tmux.New(r)
	if err := tc.RespawnPaneKill(ctx, target); err != nil {
		logger.Error("respawn-pane failed", "target", target, "err", err)
		os.Exit(1)
	}
	if err := tc.SendKeys(ctx, target, "claude --continue", "Enter"); err != nil {
		logger.Error("send-keys failed", "target", target, "err", err)
		os.Exit(1)
	}
}

// pickTargetPane returns the pane_id of the first pane in the current
// session whose pane_current_command == "claude", or the active pane_id
// when no such pane exists.
func pickTargetPane(ctx context.Context, r proc.Runner) (string, error) {
	tc := tmux.New(r)
	session, err := tc.DisplayMessageGet(ctx, "", "#S")
	if err != nil {
		return "", err
	}
	lines, err := tc.ListPanes(ctx, session, "#{pane_id} #{pane_current_command}")
	if err != nil {
		return "", err
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "claude" {
			return fields[0], nil
		}
	}
	return tc.DisplayMessageGet(ctx, "", "#{pane_id}")
}
```

- [ ] **Step 2.6: テスト pass 確認**

```bash
cd programs/claude-tools && go test -race ./cmd/claude-respawn-pane/... ./internal/tmux/...
```

Expected: 全 PASS。

- [ ] **Step 2.7: `dot_config/tmux/conf/bindings.conf` L71 書き換え**

旧: `run-shell "~/.config/tmux/scripts/claude-respawn-pane.sh"`
新: `run-shell "~/.local/bin/claude-respawn-pane"`

- [ ] **Step 2.8: 旧 shell 削除**

```bash
git rm dot_config/tmux/scripts/executable_claude-respawn-pane.sh
```

- [ ] **Step 2.9: smoke**

```bash
chezmoi diff && chezmoi apply
tmux source-file ~/.config/tmux/tmux.conf
```

2-pane window で claude pane を起動 → `prefix + C → r` で claude pane が再起動。1-pane window で `prefix + C → r` で current pane が `claude --continue` 起動。

- [ ] **Step 2.10: smoke log 追記**

```markdown
## C-2: claude-respawn-pane — 2026-05-02

- [x] 2-pane window: claude pane respawn OK
- [x] 1-pane window: current pane fallback OK
```

- [ ] **Step 2.11: commit**

```bash
git add programs/claude-tools/internal/tmux/ \
        programs/claude-tools/cmd/claude-respawn-pane/ \
        dot_config/tmux/conf/bindings.conf \
        docs/superpowers/smoke/2026-05-02-go-tmux-scripts-smoke.md
git rm dot_config/tmux/scripts/executable_claude-respawn-pane.sh
git commit -m "$(cat <<'EOF'
feat(g2): C-2 replace claude-respawn-pane.sh with Go binary + internal/tmux skeleton

- introduce internal/tmux with Display / ListPanes / DisplayMessageGet /
  RespawnPaneKill / SendKeys / Sanitize / ShellQuote (rest of the API
  surface lands in C-3 / C-4)
- claude-respawn-pane finds the first pane running 'claude' in the current
  session and restarts it via respawn-pane -k + send-keys; falls back to
  the active pane when no claude pane exists
- tmux bindings.conf updated to call ~/.local/bin/claude-respawn-pane
EOF
)"
```

---

### Task 3: PR-C-3 — `claude-kill-session`

**Files:**
- Modify: `programs/claude-tools/internal/tmux/tmux.go` (+ KillWindow, ShowWindowOption)
- Modify: `programs/claude-tools/internal/tmux/tmux_test.go` (+ argv test)
- Modify: `programs/claude-tools/internal/gitwt/gitwt.go` (+ ListPorcelain, MainRepo, FindByBranch, Remove, Prune, TopLevel)
- Modify: `programs/claude-tools/internal/gitwt/gitwt_test.go` (+ parsePorcelain table-driven 9 case + Remove argv)
- Create: `programs/claude-tools/cmd/claude-kill-session/main.go`
- Create: `programs/claude-tools/cmd/claude-kill-session/main_test.go`
- Modify: `dot_config/tmux/conf/bindings.conf:86`
- Delete: `dot_config/tmux/scripts/executable_claude-kill-session.sh`

#### TDD steps

- [ ] **Step 3.1: `internal/tmux` に `KillWindow` / `ShowWindowOption` を追加 (RED → GREEN)**

`tmux.go` 末尾に追加:

```go
// KillWindow runs `tmux kill-window -t <target>`.
func (c *Client) KillWindow(ctx context.Context, target string) error {
	_, err := c.runner.Run(ctx, "tmux", "kill-window", "-t", target)
	return err
}

// ShowWindowOption returns the value of a tmux window option, or
// ("", nil) when the option is unset (show-options -v exits non-zero
// in that case; we deliberately swallow the error to model "unset").
func (c *Client) ShowWindowOption(ctx context.Context, target, key string) (string, error) {
	out, err := c.runner.Run(ctx, "tmux", "show-options", "-w", "-t", target, "-v", key)
	if err != nil {
		return "", nil
	}
	return strings.TrimRight(string(out), "\n"), nil
}
```

`tmux_test.go` 末尾に追加:

```go
func TestKillWindow_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"kill-window", "-t", "S:W"}, nil, nil)
	if err := New(r).KillWindow(context.Background(), "S:W"); err != nil {
		t.Fatal(err)
	}
}

func TestShowWindowOption_returnsValue(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"show-options", "-w", "-t", "S:W", "-v", "@claude-managed"}, []byte("yes\n"), nil)
	got, err := New(r).ShowWindowOption(context.Background(), "S:W", "@claude-managed")
	if err != nil || got != "yes" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestShowWindowOption_unsetReturnsEmpty(t *testing.T) {
	r := proc.NewFakeRunner() // unregistered → fake returns error
	got, err := New(r).ShowWindowOption(context.Background(), "S:W", "@missing")
	if err != nil || got != "" {
		t.Fatalf("got=%q err=%v want empty nil", got, err)
	}
}
```

```bash
cd programs/claude-tools && go test -race ./internal/tmux/...
```

Expected: 全 PASS。

- [ ] **Step 3.2: `internal/gitwt` に porcelain parser + 関連 method を追加**

`gitwt.go` 冒頭の import に追加:

```go
import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"claude-tools/internal/proc"
)
```

末尾に追加:

```go
// ListPorcelain runs `git -C <cwd> worktree list --porcelain` and parses
// the output into Worktree entries.
func (c *Client) ListPorcelain(ctx context.Context, cwd string) ([]Worktree, error) {
	out, err := c.runner.Run(ctx, "git", "-C", cwd, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parsePorcelain(out), nil
}

// parsePorcelain parses the multi-line, blank-line-separated record format
// emitted by `git worktree list --porcelain`. The first record is always
// the main worktree.
func parsePorcelain(b []byte) []Worktree {
	var out []Worktree
	var cur Worktree
	flush := func() {
		if cur.Path != "" {
			out = append(out, cur)
		}
		cur = Worktree{}
	}
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			cur.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch refs/heads/"):
			cur.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "" && cur.Path != "":
			flush()
		}
	}
	flush()
	return out
}

// MainRepo returns the path of the first worktree (always the main repo).
func (c *Client) MainRepo(ctx context.Context, cwd string) (string, error) {
	wts, err := c.ListPorcelain(ctx, cwd)
	if err != nil {
		return "", err
	}
	if len(wts) == 0 {
		return "", fmt.Errorf("no worktrees found at %q", cwd)
	}
	return wts[0].Path, nil
}

// FindByBranch returns the worktree whose Branch == branch.
func (c *Client) FindByBranch(ctx context.Context, cwd, branch string) (Worktree, bool, error) {
	wts, err := c.ListPorcelain(ctx, cwd)
	if err != nil {
		return Worktree{}, false, err
	}
	for _, w := range wts {
		if w.Branch == branch {
			return w, true, nil
		}
	}
	return Worktree{}, false, nil
}

// Remove runs `git -C <mainRepo> worktree remove <target> --force` and
// returns the captured stderr if it failed. Uses exec.CommandContext
// directly because proc.Runner.Run drops stderr — kill-session needs it
// for the "kept worktree" display message.
func (c *Client) Remove(ctx context.Context, mainRepo, target string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", mainRepo, "worktree", "remove", target, "--force")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), err
	}
	return "", nil
}

// Prune runs `git -C <mainRepo> worktree prune` (failure ignored).
func (c *Client) Prune(ctx context.Context, mainRepo string) {
	_, _ = c.runner.Run(ctx, "git", "-C", mainRepo, "worktree", "prune")
}

// TopLevel returns `git -C <cwd> rev-parse --show-toplevel`.
func (c *Client) TopLevel(ctx context.Context, cwd string) (string, error) {
	out, err := c.runner.Run(ctx, "git", "-C", cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
```

- [ ] **Step 3.3: `internal/gitwt/gitwt_test.go` に porcelain 9 case + 各 method test 追加**

```go
import "reflect"

func TestParsePorcelain_table(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []Worktree
	}{
		{"empty", "", nil},
		{"single main", "worktree /home/u/r\nHEAD aaa\nbranch refs/heads/main\n", []Worktree{
			{Path: "/home/u/r", Branch: "main", HEAD: "aaa"},
		}},
		{"main + 1 wt", "worktree /a\nHEAD a1\nbranch refs/heads/main\n\nworktree /b\nHEAD b1\nbranch refs/heads/feat\n", []Worktree{
			{Path: "/a", Branch: "main", HEAD: "a1"},
			{Path: "/b", Branch: "feat", HEAD: "b1"},
		}},
		{"main + N", "worktree /a\nHEAD a1\nbranch refs/heads/main\n\nworktree /b\nHEAD b1\nbranch refs/heads/x\n\nworktree /c\nHEAD c1\nbranch refs/heads/y\n", []Worktree{
			{Path: "/a", Branch: "main", HEAD: "a1"},
			{Path: "/b", Branch: "x", HEAD: "b1"},
			{Path: "/c", Branch: "y", HEAD: "c1"},
		}},
		{"detached", "worktree /a\nHEAD a1\nbranch refs/heads/main\n\nworktree /tmp/d\nHEAD d1\ndetached\n", []Worktree{
			{Path: "/a", Branch: "main", HEAD: "a1"},
			{Path: "/tmp/d", Branch: "", HEAD: "d1"},
		}},
		{"branch ref non-heads (tag)", "worktree /a\nHEAD a1\nbranch refs/tags/v1\n", []Worktree{
			{Path: "/a", Branch: "", HEAD: "a1"}, // tag は無視されて Branch="" のまま
		}},
		{"no trailing blank", "worktree /a\nHEAD a1\nbranch refs/heads/main", []Worktree{
			{Path: "/a", Branch: "main", HEAD: "a1"},
		}},
		{"extra blank lines", "\n\nworktree /a\nHEAD a1\nbranch refs/heads/main\n\n\n\n", []Worktree{
			{Path: "/a", Branch: "main", HEAD: "a1"},
		}},
		{"branch slash in name", "worktree /a\nHEAD a1\nbranch refs/heads/feat/sub-x\n", []Worktree{
			{Path: "/a", Branch: "feat/sub-x", HEAD: "a1"},
		}},
	}
	for _, c := range cases {
		got := parsePorcelain([]byte(c.in))
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}

func TestMainRepo_returnsFirst(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/x", "worktree", "list", "--porcelain"},
		[]byte("worktree /m\nHEAD a\nbranch refs/heads/main\n\nworktree /w\nHEAD b\nbranch refs/heads/x\n"), nil)
	got, err := New(r).MainRepo(context.Background(), "/x")
	if err != nil || got != "/m" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestFindByBranch_hit(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/x", "worktree", "list", "--porcelain"},
		[]byte("worktree /m\nHEAD a\nbranch refs/heads/main\n\nworktree /w\nHEAD b\nbranch refs/heads/feat\n"), nil)
	got, ok, err := New(r).FindByBranch(context.Background(), "/x", "feat")
	if err != nil || !ok || got.Path != "/w" {
		t.Fatalf("got=%+v ok=%v err=%v", got, ok, err)
	}
}

func TestFindByBranch_miss(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/x", "worktree", "list", "--porcelain"},
		[]byte("worktree /m\nHEAD a\nbranch refs/heads/main\n"), nil)
	_, ok, err := New(r).FindByBranch(context.Background(), "/x", "nope")
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v want false nil", ok, err)
	}
}

func TestTopLevel(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "rev-parse", "--show-toplevel"}, []byte("/m\n"), nil)
	got, err := New(r).TopLevel(context.Background(), "/p")
	if err != nil || got != "/m" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}
```

```bash
cd programs/claude-tools && go test -race ./internal/gitwt/...
```

Expected: 全 PASS。

- [ ] **Step 3.4: `cmd/claude-kill-session/main_test.go` で安全チェック RED テスト**

```go
package main

import "testing"

func TestIsClaudeManaged_table(t *testing.T) {
	cases := []struct {
		name    string
		managed string
		panes   string
		session string
		want    bool
	}{
		{"managed=yes", "yes", "", "", true},
		{"claude pane present", "", "zsh\nclaude\n", "", true},
		{"legacy session", "", "zsh\nvim\n", "claude-old", true},
		{"all NG", "", "zsh\nvim\n", "dev", false},
		{"managed empty + claude in panes mid", "", "zsh\nclaude\nvim\n", "dev", true},
	}
	for _, c := range cases {
		if got := isClaudeManaged(c.managed, c.panes, c.session); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 3.5: `cmd/claude-kill-session/main.go` 実装**

```go
// claude-kill-session removes the current claude-managed window and its
// matching git worktree. The 3-stage safety check (managed=yes /
// pane has 'claude' / legacy 'claude-' session prefix) must pass; the
// caller (tmux confirm-before binding) is responsible for user
// confirmation.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-tools/internal/gitwt"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
	"claude-tools/internal/tmux"
	"claude-tools/internal/xdg"
)

const progName = "claude-kill-session"

var logger = obslog.New(progName)

func main() {
	ctx := context.Background()
	r := proc.RealRunner{}
	tc := tmux.New(r)

	explicit := ""
	if len(os.Args) > 1 {
		explicit = os.Args[1]
	}

	session, window, err := resolveTarget(ctx, tc, explicit)
	if err != nil {
		tc.Display(ctx, fmt.Sprintf("claude-kill-session: %s", err))
		os.Exit(1)
	}
	target := session + ":" + window

	managed, _ := tc.ShowWindowOption(ctx, target, "@claude-managed")
	panesOut, _ := r.Run(ctx, "tmux", "list-panes", "-t", target, "-F", "#{pane_current_command}")
	if !isClaudeManaged(managed, string(panesOut), session) {
		tc.Display(ctx, fmt.Sprintf("claude-kill-session: refusing on non-claude window (%s)", target))
		os.Exit(1)
	}

	wtRoot, _ := tc.ShowWindowOption(ctx, target, "@claude-worktree")
	mainRepo, _ := tc.ShowWindowOption(ctx, target, "@claude-main-repo")

	// fallback: derive from active pane's pane_current_path
	if wtRoot == "" || mainRepo == "" {
		panePath, _ := tc.DisplayMessageGet(ctx, target, "#{pane_current_path}")
		if panePath != "" && dirExists(panePath) {
			gw := gitwt.New(r)
			if wtRoot == "" {
				if v, err := gw.TopLevel(ctx, panePath); err == nil {
					wtRoot = v
				}
			}
			if mainRepo == "" {
				if v, err := gw.MainRepo(ctx, panePath); err == nil {
					mainRepo = v
				}
			}
		}
	}

	// Pane id capture for cache cleanup
	paneIDsOut, _ := r.Run(ctx, "tmux", "list-panes", "-t", target, "-F", "#{pane_id}")
	paneIDs := strings.Fields(string(paneIDsOut))

	// Worktree remove BEFORE kill-window (so error display still has a client).
	if wtRoot != "" && mainRepo != "" && wtRoot != mainRepo {
		gw := gitwt.New(r)
		if dirExists(wtRoot) {
			if msg, err := gw.Remove(ctx, mainRepo, wtRoot); err != nil {
				tc.Display(ctx, fmt.Sprintf("kept worktree %s: %s", wtRoot, msg))
			}
		}
		gw.Prune(ctx, mainRepo)
	}

	if err := tc.KillWindow(ctx, target); err != nil {
		logger.Error("kill-window failed", "target", target, "err", err)
	}

	// Cockpit cache cleanup
	cacheDir := xdg.ClaudeCockpitCacheDir()
	if cacheDir != "" {
		for _, pid := range paneIDs {
			_ = os.Remove(filepath.Join(cacheDir, session+"_"+pid+".status"))
		}
	}
}

// resolveTarget returns the session and window names for the kill target.
// explicit is the optional first CLI arg (session-only or session:window form).
func resolveTarget(ctx context.Context, tc *tmux.Client, explicit string) (string, string, error) {
	if explicit != "" {
		s, err := tc.DisplayMessageGet(ctx, explicit, "#S")
		if err != nil {
			return "", "", fmt.Errorf("target not found (%s)", explicit)
		}
		w, err := tc.DisplayMessageGet(ctx, explicit, "#W")
		if err != nil {
			return "", "", fmt.Errorf("window not found (%s)", explicit)
		}
		return s, w, nil
	}
	s, err := tc.DisplayMessageGet(ctx, "", "#S")
	if err != nil {
		return "", "", err
	}
	w, err := tc.DisplayMessageGet(ctx, "", "#W")
	if err != nil {
		return "", "", err
	}
	return s, w, nil
}

// isClaudeManaged implements the 3-stage OR safety check.
//   managed == "yes" OR any pane runs 'claude' OR session starts with 'claude-'
func isClaudeManaged(managedOpt, panesOut, session string) bool {
	if managedOpt == "yes" {
		return true
	}
	for _, line := range strings.Split(strings.TrimRight(panesOut, "\n"), "\n") {
		if strings.TrimSpace(line) == "claude" {
			return true
		}
	}
	return strings.HasPrefix(session, "claude-")
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
```

- [ ] **Step 3.6: テスト pass 確認**

```bash
cd programs/claude-tools && go test -race ./...
```

Expected: 全 PASS。`isClaudeManaged` 5 case + 既存 PR-C-1/C-2 が壊れていない。

- [ ] **Step 3.7: `dot_config/tmux/conf/bindings.conf` L86 書き換え**

旧: `confirm-before -p "kill claude window and worktree? (y/n) " "run-shell '~/.config/tmux/scripts/claude-kill-session.sh'"`
新: `confirm-before -p "kill claude window and worktree? (y/n) " "run-shell '~/.local/bin/claude-kill-session'"`

- [ ] **Step 3.8: 旧 shell 削除**

```bash
git rm dot_config/tmux/scripts/executable_claude-kill-session.sh
```

- [ ] **Step 3.9: smoke**

`chezmoi diff` → `chezmoi apply` → `tmux source-file ~/.config/tmux/tmux.conf`。テスト用 worktree window を作成 (この時点ではまだ shell の `tmux-claude-new` を使う、C-4 完了前のため)、`prefix + C → x` で確認:

- worktree window が kill される
- `git -C <main> worktree list` で対応 entry が消えている
- `~/.cache/claude-cockpit/panes/<S>_<pid>.status` が消えている
- 安全チェック失敗ケース: 普通の zsh window で `prefix + C → x` → display-message のみで kill されない

- [ ] **Step 3.10: smoke log 追記**

```markdown
## C-3: claude-kill-session — 2026-05-02

- [x] managed=yes window: kill + worktree remove + cache cleanup OK
- [x] non-claude window: refuse + display-message OK
- [x] fallback (pane_current_path): test 用に @claude-* tag を unset した window で確認
```

- [ ] **Step 3.11: commit**

```bash
git add programs/claude-tools/internal/{tmux,gitwt}/ \
        programs/claude-tools/cmd/claude-kill-session/ \
        dot_config/tmux/conf/bindings.conf \
        docs/superpowers/smoke/2026-05-02-go-tmux-scripts-smoke.md
git rm dot_config/tmux/scripts/executable_claude-kill-session.sh
git commit -m "$(cat <<'EOF'
feat(g2): C-3 replace claude-kill-session.sh with Go binary

- internal/tmux gains KillWindow / ShowWindowOption
- internal/gitwt gains ListPorcelain (with table-driven 9-case parser
  test), MainRepo, FindByBranch, Remove, Prune, TopLevel
- claude-kill-session enforces the 3-stage safety check (managed=yes /
  pane runs claude / legacy claude-* session) and removes the worktree
  before killing the window so error display still has a client; cockpit
  cache entries for the killed panes are cleaned up
- tmux bindings.conf updated to call ~/.local/bin/claude-kill-session
EOF
)"
```

---

### ★ C 中間チェックポイント (PR-C-3 完走後)

- [ ] **Step CK.1: C-1〜C-3 通し smoke**

C-1 (status-right `[<branch>] `) / C-2 (`prefix + C → r` で claude pane respawn) / C-3 (`prefix + C → x` で worktree kill) を 1 周まとめて再走。各 1 分以内で済むはず。

- [ ] **Step CK.2: go/no-go 判定**

問題なければ C-4 へ進む。問題があれば revert + 別 plan。判定結果を smoke log の最後に追記:

```markdown
## C-3 後の go/no-go: GO (2026-05-02)
```

---

### Task 4: PR-C-4 — `claude-tmux-new`

**Files:**
- Modify: `programs/claude-tools/internal/tmux/tmux.go` (+ NewSessionDetached, NewWindowSelectExisting, SplitWindowH, SetWindowOption, SwitchClient, AttachSessionExec, SelectPaneTitle, HasSession)
- Modify: `programs/claude-tools/internal/tmux/tmux_test.go` (+ argv test)
- Modify: `programs/claude-tools/internal/gitwt/gitwt.go` (+ HasLocalRef, HasRemoteRef, AddExistingLocal, AddTrackingRemote, AddFromHead)
- Modify: `programs/claude-tools/internal/gitwt/gitwt_test.go` (+ argv test)
- Create: `programs/claude-tools/cmd/claude-tmux-new/main.go`
- Create: `programs/claude-tools/cmd/claude-tmux-new/main_test.go`
- Delete: `dot_config/tmux/scripts/executable_tmux-claude-new.sh`

#### TDD steps

- [ ] **Step 4.1: `internal/tmux` に残り method を追加**

`tmux.go` の import に `os`, `os/exec`, `syscall` を追加。末尾に追加:

```go
// HasSession returns true if `tmux has-session -t "=<name>"` exits zero.
func (c *Client) HasSession(ctx context.Context, name string) bool {
	_, err := c.runner.Run(ctx, "tmux", "has-session", "-t", "="+name)
	return err == nil
}

// SetWindowOption sets a tmux window-scope option.
func (c *Client) SetWindowOption(ctx context.Context, target, key, value string) error {
	_, err := c.runner.Run(ctx, "tmux", "set-option", "-w", "-t", target, "-o", key, value)
	return err
}

// NewSessionDetached creates a detached tmux session.
func (c *Client) NewSessionDetached(ctx context.Context, session, window, cwd string) error {
	_, err := c.runner.Run(ctx, "tmux", "new-session", "-d", "-s", session, "-n", window, "-c", cwd)
	return err
}

// NewWindowSelectExisting runs `tmux new-window -S` (selects same-named
// window if it already exists, instead of creating a duplicate).
func (c *Client) NewWindowSelectExisting(ctx context.Context, session, window, cwd string) error {
	_, err := c.runner.Run(ctx, "tmux", "new-window", "-S", "-t", session+":", "-n", window, "-c", cwd)
	return err
}

// SplitWindowH splits the window horizontally.
func (c *Client) SplitWindowH(ctx context.Context, target, cwd string) error {
	_, err := c.runner.Run(ctx, "tmux", "split-window", "-h", "-t", target, "-c", cwd)
	return err
}

// SelectPaneTitle sets the pane title (failures swallowed — cosmetic).
func (c *Client) SelectPaneTitle(ctx context.Context, target, title string) {
	_, _ = c.runner.Run(ctx, "tmux", "select-pane", "-t", target, "-T", title)
}

// SwitchClient switches the current client to the given target.
func (c *Client) SwitchClient(ctx context.Context, target string) error {
	_, err := c.runner.Run(ctx, "tmux", "switch-client", "-t", target)
	return err
}

// AttachSessionExec replaces the current process with `tmux attach-session`,
// preserving the TTY. Returns only on syscall.Exec failure.
func (c *Client) AttachSessionExec(target string) error {
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	return syscall.Exec(bin, []string{"tmux", "attach-session", "-t", target}, os.Environ())
}
```

- [ ] **Step 4.2: 対応 test を `tmux_test.go` 末尾に追加**

```go
func TestNewSessionDetached_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"new-session", "-d", "-s", "S", "-n", "W", "-c", "/p"}, nil, nil)
	if err := New(r).NewSessionDetached(context.Background(), "S", "W", "/p"); err != nil {
		t.Fatal(err)
	}
}

func TestSetWindowOption_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"set-option", "-w", "-t", "S:W", "-o", "@claude-managed", "yes"}, nil, nil)
	if err := New(r).SetWindowOption(context.Background(), "S:W", "@claude-managed", "yes"); err != nil {
		t.Fatal(err)
	}
}

func TestHasSession_returnsTrueOnSuccess(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"has-session", "-t", "=S"}, nil, nil)
	if !New(r).HasSession(context.Background(), "S") {
		t.Fatal("want true")
	}
}

func TestSwitchClient_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"switch-client", "-t", "S:W"}, nil, nil)
	if err := New(r).SwitchClient(context.Background(), "S:W"); err != nil {
		t.Fatal(err)
	}
}

func TestSplitWindowH_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"split-window", "-h", "-t", "S:W", "-c", "/p"}, nil, nil)
	if err := New(r).SplitWindowH(context.Background(), "S:W", "/p"); err != nil {
		t.Fatal(err)
	}
}

func TestNewWindowSelectExisting_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"new-window", "-S", "-t", "S:", "-n", "W", "-c", "/p"}, nil, nil)
	if err := New(r).NewWindowSelectExisting(context.Background(), "S", "W", "/p"); err != nil {
		t.Fatal(err)
	}
}
```

```bash
cd programs/claude-tools && go test -race ./internal/tmux/...
```

Expected: 全 PASS。

- [ ] **Step 4.3: `internal/gitwt` に残り method 追加**

`gitwt.go` 末尾:

```go
// HasLocalRef returns true when refs/heads/<branch> exists.
func (c *Client) HasLocalRef(ctx context.Context, cwd, branch string) bool {
	_, err := c.runner.Run(ctx, "git", "-C", cwd, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// HasRemoteRef is the same for refs/remotes/origin/<branch>.
func (c *Client) HasRemoteRef(ctx context.Context, cwd, branch string) bool {
	_, err := c.runner.Run(ctx, "git", "-C", cwd, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	return err == nil
}

// AddExistingLocal: git worktree add <path> <branch>
func (c *Client) AddExistingLocal(ctx context.Context, cwd, path, branch string) error {
	_, err := c.runner.Run(ctx, "git", "-C", cwd, "worktree", "add", path, branch)
	return err
}

// AddTrackingRemote: git worktree add -b <branch> <path> origin/<branch>
func (c *Client) AddTrackingRemote(ctx context.Context, cwd, path, branch string) error {
	_, err := c.runner.Run(ctx, "git", "-C", cwd, "worktree", "add", "-b", branch, path, "origin/"+branch)
	return err
}

// AddFromHead: git worktree add -b <branch> <path> HEAD
func (c *Client) AddFromHead(ctx context.Context, cwd, path, branch string) error {
	_, err := c.runner.Run(ctx, "git", "-C", cwd, "worktree", "add", "-b", branch, path, "HEAD")
	return err
}
```

対応 test:

```go
func TestHasLocalRef(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "show-ref", "--verify", "--quiet", "refs/heads/x"}, nil, nil)
	if !New(r).HasLocalRef(context.Background(), "/p", "x") {
		t.Fatal("want true")
	}
}

func TestAddExistingLocal_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "worktree", "add", "/wt", "x"}, nil, nil)
	if err := New(r).AddExistingLocal(context.Background(), "/p", "/wt", "x"); err != nil {
		t.Fatal(err)
	}
}

func TestAddTrackingRemote_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "worktree", "add", "-b", "x", "/wt", "origin/x"}, nil, nil)
	if err := New(r).AddTrackingRemote(context.Background(), "/p", "/wt", "x"); err != nil {
		t.Fatal(err)
	}
}

func TestAddFromHead_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "worktree", "add", "-b", "x", "/wt", "HEAD"}, nil, nil)
	if err := New(r).AddFromHead(context.Background(), "/p", "/wt", "x"); err != nil {
		t.Fatal(err)
	}
}
```

```bash
cd programs/claude-tools && go test -race ./internal/...
```

Expected: 全 PASS。

- [ ] **Step 4.4: `cmd/claude-tmux-new/main_test.go` で argparse + buildClaudeCommand RED テスト**

```go
package main

import (
	"reflect"
	"testing"
)

func TestParseArgs_table(t *testing.T) {
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
		{"--worktree-base", []string{"feat/x", "--worktree-base", "/tmp/wt"}, options{branch: "feat/x", worktreeBase: "/tmp/wt"}, false},
		{"--prompt", []string{"feat/x", "--prompt", "hi"}, options{branch: "feat/x", initialPrompt: "hi"}, false},
		{"missing branch", []string{}, options{}, true},
		{"--from-root + --no-claude", []string{"feat/x", "--from-root", "--no-claude"}, options{}, true},
		{"--prompt + --no-claude", []string{"feat/x", "--no-claude", "--prompt", "hi"}, options{}, true},
		{"--worktree-base missing arg", []string{"feat/x", "--worktree-base"}, options{}, true},
	}
	for _, c := range cases {
		got, err := parseArgs(c.argv)
		if (err != nil) != c.err {
			t.Errorf("%s: err=%v wantErr=%v", c.name, err, c.err)
			continue
		}
		if err == nil && !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}

func TestBuildClaudeCommand_table(t *testing.T) {
	cases := []struct {
		name      string
		sessionID string
		history   bool
		prompt    string
		want      string
	}{
		{"plain", "", false, "", "claude"},
		{"continue", "", true, "", "claude --continue --fork-session"},
		{"resume", "abc-123", false, "", "claude --resume abc-123 --fork-session"},
		{"plain + prompt", "", false, "hi", "claude 'hi'"},
		{"continue + prompt with quote", "", true, "it's", `claude --continue --fork-session 'it'\''s'`},
	}
	for _, c := range cases {
		if got := buildClaudeCommand(c.sessionID, c.history, c.prompt); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 4.5: `cmd/claude-tmux-new/main.go` 実装**

```go
// claude-tmux-new creates (or attaches to) a tmux session+window pair backed
// by a git worktree, optionally starting `claude` in the right pane.
//
// usage: claude-tmux-new <branch> [--from-root [<session-id>]] [--no-claude]
//                                 [--worktree-base <dir>] [--prompt <text>]
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
```

- [ ] **Step 4.6: テスト pass 確認**

```bash
cd programs/claude-tools && go test -race ./...
```

Expected: 全 PASS。`parseArgs` table 10 case + `buildClaudeCommand` table 5 case + 既存テストすべて緑。

- [ ] **Step 4.7: 旧 shell 削除**

```bash
git rm dot_config/tmux/scripts/executable_tmux-claude-new.sh
```

(`bindings.conf` は `tmux-claude-new` を直接呼んでいない — pick-branch 経由なので C-5 で更新)。

- [ ] **Step 4.8: smoke**

`chezmoi diff` → `chezmoi apply` (claude-tmux-new binary が `~/.local/bin/` に出る)。

```bash
~/.local/bin/claude-tmux-new test/c4-smoke
# → tmux session が出来て 2-pane window が起動、claude pane に prompt
```

確認項目:
- worktree が `<main_repo>-test-c4-smoke` に作成
- 2-pane window: 左 shell / 右 claude
- claude が `claude` (履歴なし) で起動
- `prefix + C → x` で kill (C-3 と連動して worktree も消える)

`--prompt`:
```bash
~/.local/bin/claude-tmux-new test/c4-prompt --prompt "テスト 'quote' 入り"
# → claude pane に: claude 'テスト '\''quote'\'' 入り'  が send-keys される
```

`--no-claude`:
```bash
~/.local/bin/claude-tmux-new test/c4-noclaude --no-claude
# → 1-pane shell-only window
```

- [ ] **Step 4.9: smoke log 追記**

```markdown
## C-4: claude-tmux-new — 2026-05-02

- [x] new branch + worktree + 2-pane + claude OK
- [x] --no-claude (1-pane shell) OK
- [x] --prompt with single quotes (ShellQuote round-trip) OK
- [x] --from-root with explicit id OK
- [x] --from-root without id (fzf) OK
- [x] --worktree-base custom dir OK
```

- [ ] **Step 4.10: commit**

```bash
git add programs/claude-tools/internal/{tmux,gitwt}/ \
        programs/claude-tools/cmd/claude-tmux-new/ \
        docs/superpowers/smoke/2026-05-02-go-tmux-scripts-smoke.md
git rm dot_config/tmux/scripts/executable_tmux-claude-new.sh
git commit -m "$(cat <<'EOF'
feat(g2): C-4 replace tmux-claude-new.sh with Go binary (claude-tmux-new)

- internal/tmux gains the rest of the surface needed for session/window
  creation: NewSessionDetached, NewWindowSelectExisting, SplitWindowH,
  SetWindowOption, SwitchClient, AttachSessionExec, SelectPaneTitle,
  HasSession (the Go binary uses a claude- prefix for grep consistency
  under ~/.local/bin/, hence the rename from tmux-claude-new)
- internal/gitwt gains HasLocalRef, HasRemoteRef, Add{ExistingLocal,
  TrackingRemote, FromHead}
- claude-tmux-new ports the full argparse (--from-root [<id>] /
  --no-claude / --worktree-base / --prompt) and worktree-resolution
  state machine; initial prompts go through tmux.ShellQuote so they
  survive send-keys safely
EOF
)"
```

---

### Task 5: PR-C-5 — `claude-pick-branch`

**Files:**
- Modify: `programs/claude-tools/internal/gitwt/gitwt.go` (+ LocalBranches)
- Modify: `programs/claude-tools/internal/gitwt/gitwt_test.go` (+ argv test)
- Create: `programs/claude-tools/cmd/claude-pick-branch/main.go`
- Create: `programs/claude-tools/cmd/claude-pick-branch/main_test.go`
- Modify: `dot_config/tmux/conf/bindings.conf:61, 66`
- Delete: `dot_config/tmux/scripts/executable_claude-pick-branch.sh`

#### TDD steps

- [ ] **Step 5.1: `internal/gitwt` に `LocalBranches` 追加**

```go
// LocalBranches lists local branch short-names via for-each-ref.
func (c *Client) LocalBranches(ctx context.Context, cwd string) ([]string, error) {
	out, err := c.runner.Run(ctx, "git", "-C", cwd, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	s := strings.TrimRight(string(out), "\n")
	if s == "" {
		return nil, nil
	}
	return strings.Split(s, "\n"), nil
}
```

対応 test:

```go
func TestLocalBranches(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "for-each-ref", "--format=%(refname:short)", "refs/heads"},
		[]byte("main\nfeat/x\nfeat/y\n"), nil)
	got, err := New(r).LocalBranches(context.Background(), "/p")
	if err != nil || len(got) != 3 || got[0] != "main" {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestLocalBranches_empty(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "for-each-ref", "--format=%(refname:short)", "refs/heads"}, []byte(""), nil)
	got, _ := New(r).LocalBranches(context.Background(), "/p")
	if got != nil {
		t.Fatalf("got %v want nil", got)
	}
}
```

```bash
cd programs/claude-tools && go test -race ./internal/gitwt/...
```

Expected: 全 PASS。

- [ ] **Step 5.2: `cmd/claude-pick-branch/main_test.go` で argv 組み立て RED テスト**

```go
package main

import (
	"reflect"
	"testing"
)

func TestBuildExecArgs_table(t *testing.T) {
	cases := []struct {
		name        string
		branch      string
		passthrough []string
		want        []string
	}{
		{"branch only", "feat/x", nil, []string{"claude-tmux-new", "feat/x"}},
		{"with --no-claude", "feat/x", []string{"--no-claude"}, []string{"claude-tmux-new", "feat/x", "--no-claude"}},
		{"multiple flags", "feat/x", []string{"--from-root", "--prompt", "hi"}, []string{"claude-tmux-new", "feat/x", "--from-root", "--prompt", "hi"}},
	}
	for _, c := range cases {
		got := buildExecArgs(c.branch, c.passthrough)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestPromptForFlags(t *testing.T) {
	cases := []struct {
		name    string
		passArg []string
		want    string
	}{
		{"default", nil, "claude branch> "},
		{"--no-claude switches prompt", []string{"--no-claude"}, "worktree branch> "},
	}
	for _, c := range cases {
		if got := promptForFlags(c.passArg); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 5.3: `cmd/claude-pick-branch/main.go` 実装**

```go
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

	bin, err := exec.LookPath("claude-tmux-new")
	if err != nil {
		tc.Display(ctx, "claude-pick-branch: claude-tmux-new not found in PATH")
		os.Exit(1)
	}
	args := buildExecArgs(pick, passthrough)
	if err := syscall.Exec(bin, args, os.Environ()); err != nil {
		logger.Error("syscall.Exec failed", "bin", bin, "err", err)
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
```

- [ ] **Step 5.4: テスト pass 確認**

```bash
cd programs/claude-tools && go test -race ./...
```

Expected: 全 PASS。

- [ ] **Step 5.5: `dot_config/tmux/conf/bindings.conf` L61, L66 書き換え**

旧 (L61):
```
display-popup -E -d "#{pane_current_path}" "~/.config/tmux/scripts/claude-pick-branch.sh"
```
新:
```
display-popup -E -d "#{pane_current_path}" "~/.local/bin/claude-pick-branch"
```

旧 (L66):
```
display-popup -E -d "#{pane_current_path}" "~/.config/tmux/scripts/claude-pick-branch.sh --no-claude"
```
新:
```
display-popup -E -d "#{pane_current_path}" "~/.local/bin/claude-pick-branch --no-claude"
```

- [ ] **Step 5.6: 旧 shell 削除**

```bash
git rm dot_config/tmux/scripts/executable_claude-pick-branch.sh
```

- [ ] **Step 5.7: smoke**

```bash
chezmoi diff && chezmoi apply
tmux source-file ~/.config/tmux/tmux.conf
```

`prefix + C → n` で popup 出現 → fzf branch 選択 → tmux-new と同じ動作。
`prefix + C → N` (`--no-claude` 経路) で fzf prompt が `worktree branch> ` になり 1-pane shell window が立つ。
fzf キャンセル (Esc) で popup が閉じて何も起きない。

- [ ] **Step 5.8: smoke log 追記**

```markdown
## C-5: claude-pick-branch — 2026-05-02

- [x] prefix + C → n: fzf "claude branch> " → claude-tmux-new 起動 OK
- [x] prefix + C → N: fzf "worktree branch> " → --no-claude 経路 OK
- [x] fzf cancel (Esc) で no-op OK
```

- [ ] **Step 5.9: commit**

```bash
git add programs/claude-tools/internal/gitwt/ \
        programs/claude-tools/cmd/claude-pick-branch/ \
        dot_config/tmux/conf/bindings.conf \
        docs/superpowers/smoke/2026-05-02-go-tmux-scripts-smoke.md
git rm dot_config/tmux/scripts/executable_claude-pick-branch.sh
git commit -m "$(cat <<'EOF'
feat(g2): C-5 replace claude-pick-branch.sh with Go binary

- internal/gitwt gains LocalBranches (for-each-ref refs/heads)
- claude-pick-branch wraps fzf and execs claude-tmux-new on selection
  (PATH-resolved); the prompt label switches to "worktree branch> " when
  --no-claude is passed through, matching the shell behaviour
- tmux bindings.conf L61/L66 updated to call ~/.local/bin/claude-pick-branch
EOF
)"
```

---

### Task 6: Phase C 完走チェックポイント

- [ ] **Step 6.1: 全体 test 緑**

```bash
cd programs/claude-tools && go test -race -cover ./...
```

Expected: 全 PASS、各 internal package で coverage >= 80%。

- [ ] **Step 6.2: `chezmoi diff` で C 完走後の状態確認**

```bash
chezmoi diff
```

Expected:
- `~/.local/bin/claude-{branch,respawn-pane,kill-session,tmux-new,pick-branch}` 5 binary が増えている
- `~/.config/tmux/scripts/{claude-branch,claude-respawn-pane,claude-kill-session,tmux-claude-new,claude-pick-branch}.sh` 5 ファイルが消えている
- `tpm-bootstrap.sh` のみ残存

- [ ] **Step 6.3: 旧 shell 残存ゼロ確認**

```bash
git ls-files | rg 'tmux/scripts/(executable_(claude-(branch|kill-session|pick-branch|respawn-pane))|executable_tmux-claude-new)\.sh' | wc -l
```

Expected: `0`

- [ ] **Step 6.4: tmux conf 内 .sh 参照は tpm-bootstrap のみ**

```bash
rg "\.config/tmux/scripts/.*\.sh" dot_config/tmux/ .chezmoiscripts/
```

Expected: `tpm-bootstrap.sh` の行のみヒット。

- [ ] **Step 6.5: 全 smoke 領域 PASS の総ざらい**

C-1 〜 C-5 の 5 領域を 1 周 (各 1〜3 分) し、smoke log 末尾に総括を追記:

```markdown
## C 完走 — 2026-05-02

- [x] C-1〜C-5 全 PASS
- [x] 5 binary が ~/.local/bin/ に存在
- [x] 旧 shell 5 本が repo から消失
- [x] tmux 上で `prefix + C → n` / `r` / `x` / `N` が全部新 binary 経由で動作
```

- [ ] **Step 6.6: `docs/todos.md` G セクション最終更新**

C-1 〜 C-5 の checkbox を `[x]` に。完走後 follow-up として spec §11 の 3 件を G セクション (D subsystem 候補) に追記:

```markdown
- [ ] D-1 (next): `tpm-bootstrap.sh` Go 化 (network I/O / git clone)
- [ ] D-2 (next): `.chezmoiscripts/run_*.sh.tmpl` の Go 化方針検討
- [ ] G-3 (low): log file の `/tmp/*.log` から `~/.cache/claude-tools/<bin>.log` への移動
```

- [ ] **Step 6.7: 完走 commit (必要なら)**

todos.md / smoke log の最終更新は C-5 commit に含めても、別 commit でも可。別にする場合:

```bash
git add docs/todos.md docs/superpowers/smoke/2026-05-02-go-tmux-scripts-smoke.md
git commit -m "docs(g2): C subsystem completion log + D subsystem follow-ups"
```

---

## Risks / Mitigations

| リスク | 影響 | 緩和 |
|---|---|---|
| `git worktree list --porcelain` parser のエッジケース漏れ | kill-session / tmux-new が誤動作 | Task 3 の table-driven 9 case + 実機 smoke |
| `printf %q` 互換崩れ | initial prompt の send-keys が壊れる | Task 2 の round-trip test (bash で reparse 一致) |
| `tmux attach-session` の TTY 引き継ぎ | binding 経由で "not a terminal" | `os.Getenv("TMUX") != ""` で switch-client 経路を選択 (Task 4 main 内分岐) |
| C-3 の fallback 解決による誤削除 | 関係ない repo を消す | `wt_root != main_repo` invariant を保つ (Task 3 main 内分岐) |
| binding.conf 書き換え漏れ | tmux popup が old `.sh` を呼んで not found | 各 PR commit 内で `rg "tmux/scripts/<binname>\.sh" dot_config/tmux/` で 0 件確認 |
| C-4 の fresh window 判定が race で誤判定 | 既存 window に余計な split | tmux 側が直列実行 (binding pipe 逐次)、test で paneCount 1/2 を fixture |

## Acceptance Criteria

- [ ] `programs/claude-tools/internal/{tmux,gitwt}/` が `go test -race ./...` pass、coverage >= 80%
- [ ] `programs/claude-tools/cmd/claude-{branch,tmux-new,pick-branch,kill-session,respawn-pane}/` 5 binary が build
- [ ] `chezmoi apply` で `~/.local/bin/claude-{branch,tmux-new,pick-branch,kill-session,respawn-pane}` 5 binary が生える
- [ ] 旧 shell 5 本が `git ls-files` から消えている (Step 6.3)
- [ ] `dot_config/tmux/conf/{bindings,status}.conf` および `dot_config/tmux/tmux.conf` 内に `.config/tmux/scripts/.*\.sh` への参照が **`tpm-bootstrap.sh` のみ** (Step 6.4)
- [ ] `docs/superpowers/smoke/2026-05-02-go-tmux-scripts-smoke.md` に C-1〜C-5 + 完走の通し記録
- [ ] `docs/todos.md` G セクションに C-1〜C-5 の `[x]` と D subsystem follow-up が記載
- [ ] `journalctl --user | grep claude-` で 5 binary の動作ログ (情報レベル) と異常時 error の両方が観測可
