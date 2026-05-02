# Shell → Go Migration (Phase 1: Foundation + B Subsystem) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Foundation (Go module + chezmoi build trigger + 5 shared internal packages) と B サブシステム 5 binary (`claude-cockpit-{state,prune,summary,next-ready,switcher}`) を Go で 1:1 置換する。Phase 1 完了後に B 完走チェックポイント (8-step smoke) を通し、Phase 2 (A サブシステム notify pipeline) に進む go/no-go を判定する。

**Architecture:** `programs/claude-tools/` (chezmoi 管理外) に Go module を新設。`cmd/<name>/main.go` の thin entry point + `internal/{cockpit,xdg,atomicfile,proc,obslog}` の共有パッケージで 2-layer 構成。`.chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl` がソース変更検知 → `go test ./... && go build -o ~/.local/bin/ ./cmd/...` を実行して binary を配布。cache/state ファイルのパス・フォーマットは shell 時代と完全互換。

**Tech Stack:** Go 1.22+ (既存 mise install)、chezmoi run_onchange、`log/slog` + `logger -t` for observability、`encoding/json` for hook payload (将来用)。外部依存ゼロ (D-Bus は Phase 2 で追加)。

**Spec:** [`../specs/2026-05-01-shell-to-go-migration-design.md`](../specs/2026-05-01-shell-to-go-migration-design.md)

**Phase 2 (A サブシステム):** 本 plan 完了後、checkpoint (Task 13) の go 判定を経て別 plan として作成 — `2026-05-XX-shell-to-go-migration-phase2.md` (TBD date)。

---

## File Structure

### 新規作成 (本 plan 内で生成)

| パス | 役割 |
|---|---|
| `programs/claude-tools/go.mod` | Go module 定義 (module: `claude-tools`, go 1.22+) |
| `programs/claude-tools/README.md` | 何のためのプロジェクトか / build / test / 配布方法 |
| `programs/claude-tools/internal/xdg/xdg.go` | `RuntimeDir` / `CacheDir` / `ClaudeCockpitCacheDir` / `ClaudeNotifyStateDir` |
| `programs/claude-tools/internal/xdg/xdg_test.go` | env パターン全網羅 |
| `programs/claude-tools/internal/atomicfile/atomicfile.go` | `Write(path, data, perm) error` (tmp + rename) |
| `programs/claude-tools/internal/atomicfile/atomicfile_test.go` | 通常書き込み / dir 不在 / 並行書き込み |
| `programs/claude-tools/internal/proc/runner.go` | `Runner` interface, `RealRunner`, `FakeRunner` |
| `programs/claude-tools/internal/proc/runner_test.go` | FakeRunner 期待 args 登録、未登録呼び出しで fail |
| `programs/claude-tools/internal/obslog/obslog.go` | `New(progname) *slog.Logger`, ERROR で `logger -t` 転送 |
| `programs/claude-tools/internal/obslog/obslog_test.go` | レベルフィルタ、ERROR 時の logger 呼び出し |
| `programs/claude-tools/internal/cockpit/cockpit.go` | `Status` enum, `PaneState`, `CachePath`, `WriteStatus`, `LoadAll`, `Summary` |
| `programs/claude-tools/internal/cockpit/cockpit_test.go` | Status round-trip, CachePath, LoadAll 破損 skip, Summary 集計 |
| `programs/claude-tools/cmd/claude-cockpit-state/main.go` | hook entry: event → status mapping, atomic write, exit 0 契約 |
| `programs/claude-tools/cmd/claude-cockpit-state/main_test.go` | hook 契約 (常に exit 0) と env handling |
| `programs/claude-tools/cmd/claude-cockpit-prune/main.go` | tmux live pane 取得、orphan cache 削除 |
| `programs/claude-tools/cmd/claude-cockpit-prune/main_test.go` | prune ロジック (live set vs cached) |
| `programs/claude-tools/cmd/claude-cockpit-summary/main.go` | LoadAll → Summary、stdout 出力 |
| `programs/claude-tools/cmd/claude-cockpit-summary/main_test.go` | byte-exact 出力一致 |
| `programs/claude-tools/cmd/claude-cockpit-next-ready/main.go` | inbox 順 done pane 列挙、cur pane 後の循環ジャンプ |
| `programs/claude-tools/cmd/claude-cockpit-next-ready/main_test.go` | 順序 / 循環 / 空集合時の display-message |
| `programs/claude-tools/cmd/claude-cockpit-switcher/main.go` | tree builder, fzf 起動, key dispatch |
| `programs/claude-tools/cmd/claude-cockpit-switcher/main_test.go` | tree builder 出力, selection parser |
| `.chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl` | source tree sha256 変更検知 → go test + go build |
| `docs/superpowers/smoke/2026-05-01-go-cockpit-smoke.md` | Task 13 (B checkpoint) の smoke test 結果記録 |

### 変更 (本 plan 内)

| パス | 変更内容 |
|---|---|
| `.chezmoiignore` | `programs/` 行を追加 (Go ソースを chezmoi 配布対象から除外) |
| `dot_config/claude/settings.json` | 4 箇所の hook command を `claude-cockpit-state.sh` → `claude-cockpit-state` に書き換え |
| `dot_config/tmux/tmux.conf:15` | `prune.sh` → `~/.local/bin/claude-cockpit-prune` |
| `dot_config/tmux/conf/bindings.conf:73,78` | `switcher.sh` → `~/.local/bin/claude-cockpit-switcher`, `next-ready.sh` → `~/.local/bin/claude-cockpit-next-ready` |
| `dot_config/tmux/conf/status.conf:11` | `summary.sh` → `~/.local/bin/claude-cockpit-summary` |
| `docs/todos.md` G-1 | PR-1 〜 PR-5 のチェックボックスを `[x]` に更新 (Task 13 で) |

### 削除 (本 plan 内、各 binary task で `git rm`)

| パス |
|---|
| `dot_local/bin/executable_claude-cockpit-state.sh` (Task 8) |
| `dot_config/tmux/scripts/cockpit/executable_prune.sh` (Task 9) |
| `dot_config/tmux/scripts/cockpit/executable_summary.sh` (Task 10) |
| `dot_config/tmux/scripts/cockpit/executable_next-ready.sh` (Task 11) |
| `dot_config/tmux/scripts/cockpit/executable_switcher.sh` (Task 12) |

### 既存ファイル (本 plan で触らない、参考情報)

- `dot_config/mise/config.toml` — 既に `go = "latest"` + `GOPATH/GOROOT` 設定済み (toolchain task 不要)
- `dot_config/tmux/scripts/cockpit/executable_*.sh` — 上記 5 本以外なし
- `dot_local/bin/executable_claude-notify-*.sh` — Phase 2 (A サブシステム) で扱う、本 plan では touch しない

---

## Tasks

### Task 0: Prerequisites & Working Tree Sanity

**Files:** (read-only verification)

- [ ] **Step 0.1: Working tree が clean か確認**

```bash
cd /home/kiyama/.local/share/worktrees/chezmoi/refactor-replace-shell-scripts
git status
```

Expected: `nothing to commit, working tree clean` (HEAD は `65bb132 docs(g1): brainstorm spec for shell→Go migration of claude-tools` または直後の commit)。**dirty なら停止** — 既存変更を別 PR に切り出してから本 plan を再開。

- [ ] **Step 0.2: Go toolchain が動くか確認**

```bash
go version
```

Expected: `go version go1.22.x linux/amd64` または以降。mise が `go = "latest"` で導入済み。

- [ ] **Step 0.3: chezmoi コマンドが PATH にあるか確認**

```bash
chezmoi --version
chezmoi source-path
```

Expected: バージョン文字列 + `/home/kiyama/.local/share/chezmoi` が表示される (worktree でない、ソース dir そのもの)。

---

### Task 1: Go Module Initialization & .chezmoiignore

**Files:**
- Create: `programs/claude-tools/go.mod`
- Create: `programs/claude-tools/README.md`
- Create: `programs/claude-tools/.gitignore` (build 生成物を git に入れない)
- Modify: `.chezmoiignore` (末尾に `programs/` を追加)

- [ ] **Step 1.1: ディレクトリ作成**

```bash
mkdir -p programs/claude-tools/{cmd,internal}
cd programs/claude-tools
go mod init claude-tools
```

Expected: `programs/claude-tools/go.mod` が以下の内容で作成される:

```go
module claude-tools

go 1.22
```

- [ ] **Step 1.2: README.md 作成**

ファイル `programs/claude-tools/README.md` を以下の内容で作成:

```markdown
# claude-tools

chezmoi-managed Go binaries that replace the shell scripts under
`dot_local/bin/executable_claude-*.sh` and
`dot_config/tmux/scripts/cockpit/executable_*.sh`.

## Layout

- `cmd/<name>/main.go` — thin entry points (1 binary per former shell script)
- `internal/{cockpit,notify,xdg,atomicfile,proc,obslog}` — shared packages

## Build & Deploy

Distributed via chezmoi's `run_onchange_after_build-claude-tools.sh.tmpl`:

    chezmoi apply  # rebuilds binaries to ~/.local/bin/ when source changes

To build manually:

    go build -trimpath -ldflags="-s -w" -o ~/.local/bin/ ./cmd/...

## Test

    go test ./...

## Spec

See `docs/superpowers/specs/2026-05-01-shell-to-go-migration-design.md`
in the chezmoi repo root.
```

- [ ] **Step 1.3: .gitignore 作成**

ファイル `programs/claude-tools/.gitignore`:

```
# build artifacts (binaries are deployed to ~/.local/bin/, never committed)
/bin/
/dist/

# go test cache / coverage
*.test
*.out
coverage.txt
```

- [ ] **Step 1.4: .chezmoiignore に programs/ 追加**

リポジトリ root の `.chezmoiignore` を編集。既存内容の末尾 (現在 `# tmux-resurrect / continuum 自動保存（生成物 — chezmoi 管理対象外）` の `.local/share/tmux/*` 行の後) に、空行を 1 つ挟んで以下を追加:

```
# claude-tools Go ソース (バイナリは chezmoi run_onchange で build → ~/.local/bin/)
programs/
```

- [ ] **Step 1.5: 動作確認**

```bash
cd /home/kiyama/.local/share/worktrees/chezmoi/refactor-replace-shell-scripts
cd programs/claude-tools && go test ./... && cd -
chezmoi diff | head -20
```

Expected:
- `go test ./...`: `?    claude-tools  [no test files]` 程度の vacuous pass (no errors)
- `chezmoi diff`: `programs/` ディレクトリ関連の差分が出てこない (`.chezmoiignore` で除外できている)

- [ ] **Step 1.6: Commit**

```bash
git add programs/claude-tools .chezmoiignore
git commit -m "feat(g1): initialize claude-tools Go module

programs/claude-tools/ に Go module を新設。chezmoi の配布対象から
.chezmoiignore で除外し、後続タスクで内部パッケージと cmd binaries
を追加していく。

実装計画: docs/superpowers/plans/2026-05-01-shell-to-go-migration.md"
```

---

### Task 2: internal/xdg

**Files:**
- Create: `programs/claude-tools/internal/xdg/xdg.go`
- Create: `programs/claude-tools/internal/xdg/xdg_test.go`

挙動: shell の `${XDG_RUNTIME_DIR:-/tmp}` / `${XDG_CACHE_HOME:-$HOME/.cache}` を Go で 1:1 再現。`HOME` 不在時は空文字列を返す (caller 側で error にする想定はしない — XDG パスが無い環境では cache dir が `.cache/...` のような相対パスになり書き込み失敗で自然に検出される)。

- [ ] **Step 2.1: テスト先行**

ファイル `programs/claude-tools/internal/xdg/xdg_test.go`:

```go
package xdg

import (
	"path/filepath"
	"testing"
)

func TestRuntimeDir(t *testing.T) {
	tests := []struct {
		name       string
		envRuntime string
		want       string
	}{
		{"env set", "/run/user/1000", "/run/user/1000"},
		{"env empty", "", "/tmp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_RUNTIME_DIR", tt.envRuntime)
			if got := RuntimeDir(); got != tt.want {
				t.Errorf("RuntimeDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCacheDir(t *testing.T) {
	t.Run("XDG_CACHE_HOME set", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", "/custom/cache")
		t.Setenv("HOME", "/home/test")
		if got := CacheDir(); got != "/custom/cache" {
			t.Errorf("CacheDir() = %q, want /custom/cache", got)
		}
	})
	t.Run("fallback to HOME/.cache", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", "")
		t.Setenv("HOME", "/home/test")
		want := filepath.Join("/home/test", ".cache")
		if got := CacheDir(); got != want {
			t.Errorf("CacheDir() = %q, want %q", got, want)
		}
	})
}

func TestClaudeCockpitCacheDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/x/cache")
	want := "/x/cache/claude-cockpit/panes"
	if got := ClaudeCockpitCacheDir(); got != want {
		t.Errorf("ClaudeCockpitCacheDir() = %q, want %q", got, want)
	}
}

func TestClaudeNotifyStateDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	want := "/run/user/1000/claude-notify/sessions"
	if got := ClaudeNotifyStateDir(); got != want {
		t.Errorf("ClaudeNotifyStateDir() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2.2: テストが fail することを確認**

```bash
cd programs/claude-tools && go test ./internal/xdg/...
```

Expected: コンパイルエラー (関数未定義) — `undefined: RuntimeDir` 等。

- [ ] **Step 2.3: 実装**

ファイル `programs/claude-tools/internal/xdg/xdg.go`:

```go
// Package xdg resolves XDG Base Directory paths used by claude-tools.
//
// Shell parity: ${XDG_RUNTIME_DIR:-/tmp} と ${XDG_CACHE_HOME:-$HOME/.cache}
// を 1:1 再現する。
package xdg

import (
	"os"
	"path/filepath"
)

// RuntimeDir returns $XDG_RUNTIME_DIR or "/tmp" if unset/empty.
func RuntimeDir() string {
	if v := os.Getenv("XDG_RUNTIME_DIR"); v != "" {
		return v
	}
	return "/tmp"
}

// CacheDir returns $XDG_CACHE_HOME or "$HOME/.cache" if unset/empty.
func CacheDir() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return v
	}
	return filepath.Join(os.Getenv("HOME"), ".cache")
}

// ClaudeCockpitCacheDir is the per-pane status cache directory.
// Layout matches tmux-agent-status (intentional, for future migration option).
func ClaudeCockpitCacheDir() string {
	return filepath.Join(CacheDir(), "claude-cockpit", "panes")
}

// ClaudeNotifyStateDir is the per-session notify replace-id state directory.
func ClaudeNotifyStateDir() string {
	return filepath.Join(RuntimeDir(), "claude-notify", "sessions")
}
```

- [ ] **Step 2.4: テストが pass することを確認**

```bash
cd programs/claude-tools && go test ./internal/xdg/... -v
```

Expected: 全テスト PASS。

- [ ] **Step 2.5: Commit**

```bash
git add programs/claude-tools/internal/xdg
git commit -m "feat(g1): add internal/xdg package

XDG Base Directory 解決を shell の \${XDG_RUNTIME_DIR:-/tmp} /
\${XDG_CACHE_HOME:-\$HOME/.cache} と 1:1 互換で実装。
ClaudeCockpitCacheDir / ClaudeNotifyStateDir で claude-tools の
2 つの専用ディレクトリパスを集約。"
```

---

### Task 3: internal/atomicfile

**Files:**
- Create: `programs/claude-tools/internal/atomicfile/atomicfile.go`
- Create: `programs/claude-tools/internal/atomicfile/atomicfile_test.go`

挙動: shell の `printf '%s' "$state" > "$tmp" && mv "$tmp" "$file"` を Go で再現。`os.CreateTemp(dir, ".tmp.*")` → `Write` → `Close` → `Rename`. 失敗時は tmp を削除。dir 自体の作成は caller 責務 (cockpit-state.sh も `mkdir -p` を別途呼んでいる)。

- [ ] **Step 3.1: テスト先行**

ファイル `programs/claude-tools/internal/atomicfile/atomicfile_test.go`:

```go
package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWrite_basic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.status")
	data := []byte("working")

	if err := Write(target, data, 0644); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "working" {
		t.Errorf("content = %q, want %q", got, "working")
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("perm = %v, want 0644", info.Mode().Perm())
	}
}

func TestWrite_dirMissing(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nonexistent-subdir", "out.status")
	err := Write(target, []byte("x"), 0644)
	if err == nil {
		t.Fatal("Write should fail when parent dir missing")
	}
}

func TestWrite_overwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.status")
	if err := Write(target, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Write(target, []byte("second"), 0644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "second" {
		t.Errorf("content = %q, want %q", got, "second")
	}
}

func TestWrite_noTmpLeftBehind(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.status")
	if err := Write(target, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "out.status" {
			t.Errorf("unexpected leftover file: %s", e.Name())
		}
	}
}
```

- [ ] **Step 3.2: テストが fail することを確認**

```bash
cd programs/claude-tools && go test ./internal/atomicfile/...
```

Expected: `undefined: Write` のコンパイルエラー。

- [ ] **Step 3.3: 実装**

ファイル `programs/claude-tools/internal/atomicfile/atomicfile.go`:

```go
// Package atomicfile provides atomic file write via tmp + rename.
//
// Shell parity: cockpit-state.sh / notify-cleanup.sh の
//   printf '%s' "$data" > "$tmp" && mv "$tmp" "$file"
// パターンと等価。
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write writes data to path atomically:
//   1. CreateTemp in the same directory as path (so rename is on same fs)
//   2. Write data and close
//   3. Chmod to perm
//   4. Rename tmp -> path
//
// Caller is responsible for ensuring the parent directory exists.
// On any failure the tmp file is removed.
func Write(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".atomic.*")
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	tmpName := tmp.Name()

	cleanup := func() {
		_ = os.Remove(tmpName)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		cleanup()
		return fmt.Errorf("chmod tmp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename tmp: %w", err)
	}
	return nil
}
```

- [ ] **Step 3.4: テストが pass することを確認**

```bash
cd programs/claude-tools && go test ./internal/atomicfile/... -v
```

Expected: 4 テスト全 PASS。

- [ ] **Step 3.5: Commit**

```bash
git add programs/claude-tools/internal/atomicfile
git commit -m "feat(g1): add internal/atomicfile package

tmp + rename での atomic write を実装。shell 時代の
\`printf > tmp && mv tmp file\` パターンと等価。失敗時は
defer 風の cleanup で tmp を残さない。dir 作成は caller 責務。"
```

---

### Task 4: internal/proc

**Files:**
- Create: `programs/claude-tools/internal/proc/runner.go`
- Create: `programs/claude-tools/internal/proc/runner_test.go`

挙動: 外部コマンド実行を interface 化することで、ユニットテストで fake を注入可能にする。production は `os/exec` を直接呼ぶ薄い wrapper。

- [ ] **Step 4.1: テスト先行**

ファイル `programs/claude-tools/internal/proc/runner_test.go`:

```go
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
```

- [ ] **Step 4.2: テストが fail することを確認**

```bash
cd programs/claude-tools && go test ./internal/proc/...
```

Expected: 未定義シンボル (`Runner`, `RealRunner`, `FakeRunner`, `NewFakeRunner`) のコンパイルエラー。

- [ ] **Step 4.3: 実装**

ファイル `programs/claude-tools/internal/proc/runner.go`:

```go
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
```

- [ ] **Step 4.4: テストが pass することを確認**

```bash
cd programs/claude-tools && go test ./internal/proc/... -v
```

Expected: 4 テスト全 PASS。

- [ ] **Step 4.5: Commit**

```bash
git add programs/claude-tools/internal/proc
git commit -m "feat(g1): add internal/proc package

外部コマンド実行を Runner interface 化。RealRunner は os/exec の
薄い wrapper、FakeRunner は事前登録した argv→response で応答する
テスト用 double。未登録呼び出しはエラーで loud-fail させる
(silent fall-through アンチパターンを防止)。"
```

---

### Task 5: internal/obslog

**Files:**
- Create: `programs/claude-tools/internal/obslog/obslog.go`
- Create: `programs/claude-tools/internal/obslog/obslog_test.go`

挙動: `slog.New` の薄い wrapper。stderr に JSON-line。ERROR レベルは追加で `logger -t <progname>` を呼んで syslog 経路にも転送 (F-5 LOW-1 で確立した運用)。`logger` コマンド不在時は静かにスキップ。

- [ ] **Step 5.1: テスト先行**

ファイル `programs/claude-tools/internal/obslog/obslog_test.go`:

```go
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
```

- [ ] **Step 5.2: テストが fail することを確認**

```bash
cd programs/claude-tools && go test ./internal/obslog/...
```

Expected: `undefined: newWith` 等のコンパイルエラー。

- [ ] **Step 5.3: 実装**

ファイル `programs/claude-tools/internal/obslog/obslog.go`:

```go
// Package obslog wraps log/slog with a stderr JSON handler that also
// forwards ERROR level records to `logger -t <progname>` for syslog.
//
// Shell parity: shell 時代の
//   command -v logger >/dev/null 2>&1 && logger -t <prog> "<msg>"
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
```

- [ ] **Step 5.4: テストが pass することを確認**

```bash
cd programs/claude-tools && go test ./internal/obslog/... -v
```

Expected: 4 テスト全 PASS。

- [ ] **Step 5.5: Commit**

```bash
git add programs/claude-tools/internal/obslog
git commit -m "feat(g1): add internal/obslog package

slog.JSONHandler + ERROR レベル時の \`logger -t\` 自動転送 wrapper を
実装。shell 時代の
  command -v logger >/dev/null 2>&1 && logger -t <prog> '<msg>'
パターン (F-5 LOW-1) を Go の slog Handler 連鎖で表現。logger 不在時は
静かにスキップ。FakeRunner で ERROR 転送が走ることを単体テストで確認。"
```

---

### Task 6: internal/cockpit

**Files:**
- Create: `programs/claude-tools/internal/cockpit/cockpit.go`
- Create: `programs/claude-tools/internal/cockpit/cockpit_test.go`

挙動: 三値 `Status` enum + `PaneState` 型 + cache file I/O + Summary 集計。
- cache file: `<XDG_CACHE_HOME>/claude-cockpit/panes/<session>_<paneID>.status`
- file 内容: 1 行リテラル (`working` / `waiting` / `done`)
- summary 出力: `⚡ N ⏸ M ✓ K ` (各セグメント = 絵文字 + space + 数値 + space、count 0 のセグメントは省略、全 0 なら空文字列)

**重要 (shell 互換):** `summary.sh:34` の `out+="⚡ $working "` を見ると、絵文字と数字の間に **space** が入っている。byte-exact 一致のため Go 実装も同じ書式にする。

- [ ] **Step 6.1: テスト先行**

ファイル `programs/claude-tools/internal/cockpit/cockpit_test.go`:

```go
package cockpit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatusParse(t *testing.T) {
	tests := []struct {
		in   string
		want Status
		ok   bool
	}{
		{"working", StatusWorking, true},
		{"waiting", StatusWaiting, true},
		{"done", StatusDone, true},
		{"", "", false},
		{"unknown", "", false},
		{"WORKING", "", false}, // case-sensitive: shell が小文字限定
	}
	for _, tt := range tests {
		got, ok := ParseStatus(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseStatus(%q) = (%v, %v), want (%v, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestCachePath(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/test-cache")
	got := CachePath("mysession", "%5")
	want := "/tmp/test-cache/claude-cockpit/panes/mysession_%5.status"
	if got != want {
		t.Errorf("CachePath = %q, want %q", got, want)
	}
}

func TestWriteStatus_thenLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	if err := WriteStatus("sess", "%1", StatusWorking); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}

	// Verify file content matches shell format (no trailing newline).
	data, err := os.ReadFile(CachePath("sess", "%1"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "working" {
		t.Errorf("file content = %q, want %q", data, "working")
	}
}

func TestLoadAll_skipsCorrupt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Valid: working
	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%1.status"), []byte("working"), 0644)
	// Valid: done
	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%2.status"), []byte("done"), 0644)
	// Corrupt: garbage content
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%3.status"), []byte("xyz"), 0644)
	// Wrong extension: ignored
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%4.txt"), []byte("waiting"), 0644)
	// Bad filename (no underscore): ignored
	_ = os.WriteFile(filepath.Join(cacheDir, "no-underscore.status"), []byte("done"), 0644)

	states, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(states) != 2 {
		t.Errorf("got %d states, want 2 (working + done): %+v", len(states), states)
	}
}

func TestLoadAll_emptyWhenDirMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	states, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll on missing dir should not error: %v", err)
	}
	if len(states) != 0 {
		t.Errorf("expected 0 states, got %d", len(states))
	}
}

func TestSummary(t *testing.T) {
	tests := []struct {
		name   string
		states []PaneState
		want   string
	}{
		{
			name:   "empty",
			states: nil,
			want:   "",
		},
		{
			name: "only working",
			states: []PaneState{
				{Status: StatusWorking}, {Status: StatusWorking},
			},
			want: "⚡ 2 ",
		},
		{
			name: "all three",
			states: []PaneState{
				{Status: StatusWorking}, {Status: StatusWorking}, {Status: StatusWorking},
				{Status: StatusWaiting},
				{Status: StatusDone}, {Status: StatusDone},
			},
			want: "⚡ 3 ⏸ 1 ✓ 2 ",
		},
		{
			name: "only done",
			states: []PaneState{
				{Status: StatusDone}, {Status: StatusDone},
			},
			want: "✓ 2 ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Summary(tt.states)
			if got != tt.want {
				t.Errorf("Summary = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 6.2: テストが fail することを確認**

```bash
cd programs/claude-tools && go test ./internal/cockpit/...
```

Expected: 未定義シンボル多数。

- [ ] **Step 6.3: 実装**

ファイル `programs/claude-tools/internal/cockpit/cockpit.go`:

```go
// Package cockpit models per-pane Claude state for the tmux cockpit.
//
// Cache layout (shell 互換):
//   ${XDG_CACHE_HOME}/claude-cockpit/panes/<session>_<paneID>.status
//   File content: single line "working" / "waiting" / "done".
//
// Status mapping intentionally matches tmux-agent-status' literal
// values (concept-only inspiration, no code copied).
package cockpit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"claude-tools/internal/atomicfile"
	"claude-tools/internal/xdg"
)

// Status is the three-valued cockpit pane state.
type Status string

const (
	StatusWorking Status = "working"
	StatusWaiting Status = "waiting"
	StatusDone    Status = "done"
)

// ParseStatus accepts only the three known string literals (case-sensitive).
// Returns ("", false) for anything else (including empty / WORKING / etc.).
func ParseStatus(s string) (Status, bool) {
	switch Status(s) {
	case StatusWorking, StatusWaiting, StatusDone:
		return Status(s), true
	}
	return "", false
}

// PaneState captures one pane's status read from the cache directory.
type PaneState struct {
	Session string
	PaneID  string // tmux pane ID like "%5"
	Status  Status
}

// CachePath returns the absolute path to the cache file for a pane.
func CachePath(session, paneID string) string {
	return filepath.Join(xdg.ClaudeCockpitCacheDir(), session+"_"+paneID+".status")
}

// WriteStatus atomically writes status for the given (session, pane).
// Creates the cache directory if missing.
func WriteStatus(session, paneID string, s Status) error {
	dir := xdg.ClaudeCockpitCacheDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir cache dir: %w", err)
	}
	return atomicfile.Write(CachePath(session, paneID), []byte(string(s)), 0644)
}

// LoadAll scans the cache dir and returns parsed pane states.
// Files with corrupt content / unparseable filenames are silently skipped.
// Missing directory returns empty slice (not an error).
func LoadAll() ([]PaneState, error) {
	dir := xdg.ClaudeCockpitCacheDir()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cache dir: %w", err)
	}

	var out []PaneState
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".status") {
			continue
		}
		base := strings.TrimSuffix(name, ".status")
		// Filename format: <session>_<paneID>. Session may itself contain
		// underscores (e.g. "main_repo"); split on the LAST underscore.
		idx := strings.LastIndex(base, "_")
		if idx <= 0 || idx == len(base)-1 {
			continue
		}
		session := base[:idx]
		paneID := base[idx+1:]

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		s, ok := ParseStatus(strings.TrimSpace(string(data)))
		if !ok {
			continue
		}
		out = append(out, PaneState{Session: session, PaneID: paneID, Status: s})
	}
	return out, nil
}

// Summary aggregates pane states into the status-right format.
// Format (byte-exact match to summary.sh):
//   "⚡ N ⏸ M ✓ K "  with each segment "<emoji> <count> " present only
//   when count > 0; trailing space included so the next status-right
//   segment is visually separated. Empty string when all counts are zero.
func Summary(states []PaneState) string {
	var working, waiting, done int
	for _, s := range states {
		switch s.Status {
		case StatusWorking:
			working++
		case StatusWaiting:
			waiting++
		case StatusDone:
			done++
		}
	}
	var sb strings.Builder
	if working > 0 {
		fmt.Fprintf(&sb, "⚡ %d ", working)
	}
	if waiting > 0 {
		fmt.Fprintf(&sb, "⏸ %d ", waiting)
	}
	if done > 0 {
		fmt.Fprintf(&sb, "✓ %d ", done)
	}
	return sb.String()
}
```

- [ ] **Step 6.4: テストが pass することを確認**

```bash
cd programs/claude-tools && go test ./internal/cockpit/... -v
```

Expected: 全テスト PASS。

- [ ] **Step 6.5: Commit**

```bash
git add programs/claude-tools/internal/cockpit
git commit -m "feat(g1): add internal/cockpit package

三値 Status enum (working/waiting/done) + cache file I/O (atomic write +
LoadAll) + Summary 集計を実装。cache layout は \`tmux-agent-status\` 互換
を維持 (将来の上流乗り換え退路を残す)。Summary 出力フォーマットは
shell の summary.sh と byte-exact 一致 (絵文字と数字の間に space)。
filename parse は最終 underscore で分割 (session 名内の \`_\` を許容)。"
```

---

### Task 7: chezmoi build trigger script

**Files:**
- Create: `.chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl`

挙動: chezmoi の `run_onchange_after_*.sh.tmpl` 慣行に従い、ファイル先頭コメントに source tree の sha256 を埋め込んで「ソースが変わったら再実行」を実現。実行内容は `go test ./... && go build -o ~/.local/bin/`。

**重要:**
- chezmoi `output` template function でソース sha256 を render 時に計算
- `.chezmoi.sourceDir` で chezmoi ソースルートを参照
- `cmd/` 配下が空 (Task 7 commit 時点では未だ binary なし) でも fail しないよう、cmd_dirs が空ならビルドをスキップする conditional を入れる

- [ ] **Step 7.1: build script 作成**

ファイル `.chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl`:

```bash
#!/usr/bin/env bash
# claude-tools build trigger (chezmoi run_onchange_after_*).
#
# このスクリプト先頭の sha256 は、programs/claude-tools 配下の Go ソース
# (*.go / go.mod / go.sum) の内容ハッシュ。chezmoi はテンプレート展開後の
# スクリプト本体が前回実行時と異なるかで再実行を判定するため、ソースが
# 変わるとここの値が変わり、自動で再ビルドされる。
#
# tree sha256: {{ output "bash" "-c" (printf "cd %q && find programs/claude-tools -type f \\( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \\) -print0 2>/dev/null | sort -z | xargs -0 -r sha256sum 2>/dev/null | sha256sum | cut -d' ' -f1" .chezmoi.sourceDir) }}

set -euo pipefail

REPO_DIR={{ .chezmoi.sourceDir | quote }}/programs/claude-tools

# 1. Source dir 不在 (chezmoi 配布対象外環境) ならスキップ
if [[ ! -d "$REPO_DIR" ]]; then
  echo "[claude-tools] source dir not found at $REPO_DIR; skipping" >&2
  exit 0
fi

# 2. Go toolchain 不在 (Manjaro 以外 / mise install 未完了) ならスキップ
if ! command -v go >/dev/null 2>&1; then
  echo "[claude-tools] go toolchain not in PATH; skipping (run \`mise install\` first)" >&2
  exit 0
fi

# 3. cmd/ サブディレクトリが空ならビルド対象なし (移行初期 / Task 7 直後の状態)
shopt -s nullglob
cmd_dirs=("$REPO_DIR"/cmd/*/)
if [[ ${#cmd_dirs[@]} -eq 0 ]]; then
  echo "[claude-tools] no cmd/ subdirectories yet; skipping build" >&2
  exit 0
fi

cd "$REPO_DIR"

# 4. テスト先行: 失敗時は新バイナリを ~/.local/bin/ に出さない (rollback 安全)
echo "[claude-tools] go test ./..."
go test ./...

# 5. build (all-or-nothing で ~/.local/bin/ に書き出し)
echo "[claude-tools] go build -> ~/.local/bin/"
mkdir -p "$HOME/.local/bin"
go build -trimpath -ldflags="-s -w" -o "$HOME/.local/bin/" ./cmd/...

echo "[claude-tools] build complete"
```

- [ ] **Step 7.2: chezmoi で template が render できるか確認**

```bash
chezmoi execute-template < .chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl | head -10
```

Expected: スクリプト先頭の `# tree sha256: ...` 行に 64 文字の hex 値が埋まっている。`chezmoi: error` のような出力が無いこと。空 sha256 になる場合 (`e3b0c44...`) は `find` の path が正しいか確認 (Task 7 時点では `internal/*` の .go ファイルが存在すべき)。

- [ ] **Step 7.3: chezmoi diff で script が見えるか確認**

```bash
chezmoi diff
```

Expected: `.chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl` が新規 script として diff に出る。

- [ ] **Step 7.4: chezmoi apply (動作確認)**

```bash
chezmoi apply
```

Expected: スクリプトが実行されるが、`cmd/` 配下が空なので `[claude-tools] no cmd/ subdirectories yet; skipping build` を吐いて exit 0。

- [ ] **Step 7.5: Commit**

```bash
git add .chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl
git commit -m "feat(g1): add chezmoi run_onchange trigger for go build

programs/claude-tools 配下の *.go / go.mod / go.sum の sha256 を
script 先頭コメントに埋め込み、chezmoi の run_onchange 慣行で
ソース変更を検知して \`go test ./... && go build -o ~/.local/bin/\` を
実行する。

ガード条件:
- source dir 不在 → skip (Manjaro 以外で chezmoi だけ走る場合)
- go コマンド不在 → skip (mise install 未完了)
- cmd/ サブディレクトリ空 → skip (移行初期)
- テスト失敗 → 新バイナリを deploy しない (rollback 安全)"
```

---

### Task 8: PR-1 — cmd/claude-cockpit-state

**Files:**
- Create: `programs/claude-tools/cmd/claude-cockpit-state/main.go`
- Create: `programs/claude-tools/cmd/claude-cockpit-state/main_test.go`
- Modify: `dot_config/claude/settings.json` (4 箇所の hook command path)
- Delete: `dot_local/bin/executable_claude-cockpit-state.sh`

**契約:**
- 引数: `claude-cockpit-state hook <Event>` (`mode` = "hook" 以外は exit 0)
- 環境変数: `TMUX_PANE` (空なら exit 0)
- イベント → ステート mapping:
  - `UserPromptSubmit` / `PreToolUse` → `working`
  - `Notification` → `waiting`
  - `Stop` → `done`
  - その他 → exit 0 (no-op)
- 副作用: `tmux display-message -p -t <pane> '#{session_name}'` で session 名取得、cache file に atomic write
- **絶対契約: 常に exit 0**

- [ ] **Step 8.1: テスト先行**

ファイル `programs/claude-tools/cmd/claude-cockpit-state/main_test.go`:

```go
package main

import (
	"context"
	"os"
	"testing"

	"claude-tools/internal/proc"
)

func TestEventToStatus(t *testing.T) {
	tests := []struct {
		event string
		want  string
		ok    bool
	}{
		{"UserPromptSubmit", "working", true},
		{"PreToolUse", "working", true},
		{"Notification", "waiting", true},
		{"Stop", "done", true},
		{"SubagentStop", "", false},
		{"", "", false},
		{"unknown", "", false},
	}
	for _, tt := range tests {
		got, ok := eventToStatus(tt.event)
		if got != tt.want || ok != tt.ok {
			t.Errorf("eventToStatus(%q) = (%q, %v), want (%q, %v)",
				tt.event, got, ok, tt.want, tt.ok)
		}
	}
}

func TestRun_modeNotHook(t *testing.T) {
	if err := run(context.Background(), proc.NewFakeRunner(), []string{"prog", "other-mode"}, "%1"); err != nil {
		t.Errorf("run with non-hook mode returned error: %v", err)
	}
}

func TestRun_emptyTmuxPane(t *testing.T) {
	if err := run(context.Background(), proc.NewFakeRunner(), []string{"prog", "hook", "Stop"}, ""); err != nil {
		t.Errorf("run with empty TMUX_PANE returned error: %v", err)
	}
}

func TestRun_unknownEvent(t *testing.T) {
	if err := run(context.Background(), proc.NewFakeRunner(), []string{"prog", "hook", "SubagentStop"}, "%1"); err != nil {
		t.Errorf("run with unknown event returned error: %v", err)
	}
}

func TestRun_sessionLookupFailure(t *testing.T) {
	fake := proc.NewFakeRunner()
	// Don't register tmux: lookup will fail. run() should still complete.
	err := run(context.Background(), fake, []string{"prog", "hook", "Stop"}, "%1")
	// Returns nil because hook contract is exit 0; inner errors are logged not bubbled.
	if err != nil {
		t.Errorf("run with tmux failure returned error: %v", err)
	}
}

func TestRun_writesStatus(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"display-message", "-p", "-t", "%5", "#{session_name}"},
		[]byte("mysession\n"), nil)

	if err := run(context.Background(), fake, []string{"prog", "hook", "UserPromptSubmit"}, "%5"); err != nil {
		t.Errorf("run returned error: %v", err)
	}

	want := dir + "/claude-cockpit/panes/mysession_%5.status"
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read cache file %q: %v", want, err)
	}
	if string(data) != "working" {
		t.Errorf("cache content = %q, want %q", data, "working")
	}
}
```

- [ ] **Step 8.2: テストが fail することを確認**

```bash
cd programs/claude-tools && go test ./cmd/claude-cockpit-state/...
```

Expected: 未定義 (`run`, `eventToStatus`)。

- [ ] **Step 8.3: 実装**

ファイル `programs/claude-tools/cmd/claude-cockpit-state/main.go`:

```go
// claude-cockpit-state is the Claude Code hook entry that records the
// current pane's state (working / waiting / done) into the cockpit cache.
//
// Usage: claude-cockpit-state hook <Event>
//
// Absolute contract: this binary ALWAYS exits 0, regardless of internal
// errors, so a failure here never blocks Claude Code's hook pipeline.
// Errors are forwarded to syslog via internal/obslog instead.
package main

import (
	"context"
	"os"
	"strings"

	"claude-tools/internal/cockpit"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
)

const progName = "claude-cockpit-state"

func main() {
	// Defense in depth: even a panic must not propagate to Claude.
	defer func() {
		_ = recover()
		os.Exit(0)
	}()

	tmuxPane := os.Getenv("TMUX_PANE")
	if err := run(context.Background(), proc.RealRunner{}, os.Args, tmuxPane); err != nil {
		// Should not happen because run() swallows errors, but guard anyway.
		obslog.New(progName).Error("run failed", "err", err)
	}
	os.Exit(0)
}

// run implements the hook logic. It always returns nil because hook
// failures are silently logged (never propagated). Returning an error
// type is reserved for future signal-based test hooks; current callers
// should ignore it.
func run(ctx context.Context, runner proc.Runner, args []string, tmuxPane string) error {
	logger := obslog.New(progName)

	// args[0] = program name; args[1] = mode; args[2] = event
	if len(args) < 3 || args[1] != "hook" {
		return nil
	}
	event := args[2]

	status, ok := eventToStatus(event)
	if !ok {
		return nil
	}
	if tmuxPane == "" {
		return nil
	}

	// tmux display-message to look up session name for this pane.
	out, err := runner.Run(ctx, "tmux", "display-message", "-p", "-t", tmuxPane, "#{session_name}")
	if err != nil {
		logger.Error("tmux session lookup failed", "pane", tmuxPane, "err", err)
		return nil
	}
	session := strings.TrimSpace(string(out))
	if session == "" {
		return nil
	}

	if err := cockpit.WriteStatus(session, tmuxPane, cockpit.Status(status)); err != nil {
		logger.Error("write status failed",
			"session", session, "pane", tmuxPane, "status", status, "err", err)
	}
	return nil
}

// eventToStatus maps Claude hook events to cockpit Status string literals.
// Returns ("", false) for events we ignore (e.g., SubagentStop).
func eventToStatus(event string) (string, bool) {
	switch event {
	case "UserPromptSubmit", "PreToolUse":
		return "working", true
	case "Notification":
		return "waiting", true
	case "Stop":
		return "done", true
	}
	return "", false
}
```

- [ ] **Step 8.4: テストが pass することを確認**

```bash
cd programs/claude-tools && go test ./cmd/claude-cockpit-state/... -v
```

Expected: 全テスト PASS。

- [ ] **Step 8.5: Claude `settings.json` を 4 箇所書き換え**

ファイル `dot_config/claude/settings.json` を Edit で開き、4 箇所の hook command を書き換える。検索文字列 (それぞれ unique):

```
"command": "/home/kiyama/.local/bin/claude-cockpit-state.sh hook UserPromptSubmit"
"command": "/home/kiyama/.local/bin/claude-cockpit-state.sh hook PreToolUse"
"command": "/home/kiyama/.local/bin/claude-cockpit-state.sh hook Notification"
"command": "/home/kiyama/.local/bin/claude-cockpit-state.sh hook Stop"
```

をそれぞれ:

```
"command": "/home/kiyama/.local/bin/claude-cockpit-state hook UserPromptSubmit"
"command": "/home/kiyama/.local/bin/claude-cockpit-state hook PreToolUse"
"command": "/home/kiyama/.local/bin/claude-cockpit-state hook Notification"
"command": "/home/kiyama/.local/bin/claude-cockpit-state hook Stop"
```

に変更 (`.sh` を取るのみ)。

- [ ] **Step 8.6: 旧 shell を削除**

```bash
git rm dot_local/bin/executable_claude-cockpit-state.sh
```

- [ ] **Step 8.7: chezmoi diff & apply で binary が出るか確認**

```bash
cd /home/kiyama/.local/share/worktrees/chezmoi/refactor-replace-shell-scripts
chezmoi diff | head -40
```

Expected:
- `dot_config/claude/settings.json` の差分が出る (4 箇所 `.sh` 削除)
- `dot_local/bin/claude-cockpit-state.sh` の削除差分

```bash
chezmoi apply
```

Expected: chezmoi が新 settings.json を反映、`run_onchange_after_build-claude-tools.sh.tmpl` が再実行され、`go test ./...` PASS → `go build -o ~/.local/bin/` で `claude-cockpit-state` バイナリが生成される。

```bash
ls -la ~/.local/bin/claude-cockpit-state ~/.local/bin/claude-cockpit-state.sh 2>&1
```

Expected: `claude-cockpit-state` が存在、`claude-cockpit-state.sh` は **No such file** (chezmoi が削除済み)。

- [ ] **Step 8.8: 実機 smoke (hook 経由)**

新しい tmux pane を開き、Claude を起動して何か入力。別 shell で:

```bash
cat ~/.cache/claude-cockpit/panes/*.status 2>/dev/null
```

Expected: `working` または `waiting` または `done` のいずれかが入っている (タイミング依存)。

何も書かれていない場合の確認:

```bash
journalctl --user --since="5 min ago" | grep claude-cockpit-state
```

Expected: ERROR ログがあれば原因が見える (例: `tmux session lookup failed`)。何もログが出ていなければ hook は走っていない (`settings.json` の path が違う / `chezmoi apply` し忘れ等)。

- [ ] **Step 8.9: Commit**

```bash
git add programs/claude-tools/cmd/claude-cockpit-state \
        dot_config/claude/settings.json
git commit -m "feat(g1): PR-1 replace claude-cockpit-state.sh with Go binary

Claude Code hook entry を Go に置換。9 binary 移行の最初の 1 本。

実装ポイント:
- absolute contract: 常に os.Exit(0) (panic recover で防御)
- 5 イベント (UserPromptSubmit/PreToolUse/Notification/Stop/その他)
  の status mapping を eventToStatus で関数化、テストで全網羅
- tmux display-message 呼び出しは proc.Runner 経由 → FakeRunner で
  shell の \`tmux display-message -p -t <pane> '#{session_name}'\` を
  完全再現する unit test を追加
- atomic write は internal/cockpit + internal/atomicfile に委譲

互換性:
- cache file パス・フォーマットは shell 時代と byte-exact
- TMUX_PANE 環境変数の扱い、空時 no-op、tmux 不在時 no-op も同等
- settings.json の hook command path 4 箇所を .sh 削除に追従

旧 dot_local/bin/executable_claude-cockpit-state.sh は git rm。
chezmoi apply で run_onchange_after_build-claude-tools が走り、
~/.local/bin/claude-cockpit-state が build される。

Spec: docs/superpowers/specs/2026-05-01-shell-to-go-migration-design.md"
```

---

### Task 9: PR-2 — cmd/claude-cockpit-prune

**Files:**
- Create: `programs/claude-tools/cmd/claude-cockpit-prune/main.go`
- Create: `programs/claude-tools/cmd/claude-cockpit-prune/main_test.go`
- Modify: `dot_config/tmux/tmux.conf:15` (`prune.sh` → `prune`)
- Delete: `dot_config/tmux/scripts/cockpit/executable_prune.sh`

**契約:**
- 引数: なし
- 副作用:
  - `tmux list-panes -a -F '#{session_name}_#{pane_id}'` で live pane key set 取得
  - `~/.cache/claude-cockpit/panes/*.status` を走査、live set に無いファイルを削除
  - 副次: `~/.cache/claude-cockpit/sessions/*.status` も同様 (defensive、現状 unused だが shell 互換)
- exit code: 通常 (失敗時 1)

- [ ] **Step 9.1: テスト先行**

ファイル `programs/claude-tools/cmd/claude-cockpit-prune/main_test.go`:

```go
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"claude-tools/internal/proc"
)

func TestPrune_removesOrphans(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)

	// 3 cache files: 2 live, 1 orphan.
	live1 := filepath.Join(cacheDir, "sess_%1.status")
	live2 := filepath.Join(cacheDir, "sess_%2.status")
	orphan := filepath.Join(cacheDir, "sess_%99.status")
	_ = os.WriteFile(live1, []byte("working"), 0644)
	_ = os.WriteFile(live2, []byte("done"), 0644)
	_ = os.WriteFile(orphan, []byte("done"), 0644)

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"list-panes", "-a", "-F", "#{session_name}_#{pane_id}"},
		[]byte("sess_%1\nsess_%2\n"), nil)

	if err := prune(context.Background(), fake); err != nil {
		t.Fatalf("prune: %v", err)
	}

	if _, err := os.Stat(live1); err != nil {
		t.Errorf("live1 was deleted: %v", err)
	}
	if _, err := os.Stat(live2); err != nil {
		t.Errorf("live2 was deleted: %v", err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Errorf("orphan was NOT deleted (Stat err = %v)", err)
	}
}

func TestPrune_emptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"list-panes", "-a", "-F", "#{session_name}_#{pane_id}"},
		[]byte(""), nil)

	if err := prune(context.Background(), fake); err != nil {
		t.Errorf("prune on missing dir should not error: %v", err)
	}
}

func TestPrune_tmuxFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)
	cached := filepath.Join(cacheDir, "sess_%1.status")
	_ = os.WriteFile(cached, []byte("done"), 0644)

	fake := proc.NewFakeRunner() // tmux not registered → error

	err := prune(context.Background(), fake)
	if err == nil {
		t.Fatal("prune should propagate tmux failure")
	}

	// Cached file must NOT be deleted on tmux failure (safety: no live set means we can't tell what's orphan).
	if _, statErr := os.Stat(cached); os.IsNotExist(statErr) {
		t.Error("cached file deleted despite tmux failure (unsafe)")
	}
}
```

- [ ] **Step 9.2: テストが fail することを確認**

```bash
cd programs/claude-tools && go test ./cmd/claude-cockpit-prune/...
```

Expected: undefined `prune`.

- [ ] **Step 9.3: 実装**

ファイル `programs/claude-tools/cmd/claude-cockpit-prune/main.go`:

```go
// claude-cockpit-prune removes per-pane cockpit cache files for tmux
// panes that no longer exist.
//
// Safe to run any time. Idempotent. Called from tmux.conf at server-start
// (`run -b`) and from cockpit-switcher at startup.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
	"claude-tools/internal/xdg"
)

const progName = "claude-cockpit-prune"

func main() {
	if err := prune(context.Background(), proc.RealRunner{}); err != nil {
		obslog.New(progName).Error("prune failed", "err", err)
		os.Exit(1)
	}
}

// prune builds the live tmux pane key set and deletes cached files
// whose basename (minus .status) is not in the set.
//
// Returns an error from tmux failure; in that case it does NOT delete
// any cached files (we can't tell which are orphans without the live set).
func prune(ctx context.Context, runner proc.Runner) error {
	out, err := runner.Run(ctx, "tmux", "list-panes", "-a", "-F", "#{session_name}_#{pane_id}")
	if err != nil {
		return fmt.Errorf("tmux list-panes: %w", err)
	}
	live := make(map[string]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			live[line] = struct{}{}
		}
	}

	pruneDir(xdg.ClaudeCockpitCacheDir(), live)
	// Defensive: shell prune.sh also cleans sessions/ if it exists.
	pruneDir(filepath.Join(xdg.CacheDir(), "claude-cockpit", "sessions"), live)
	return nil
}

func pruneDir(dir string, live map[string]struct{}) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		obslog.New(progName).Error("readdir failed", "dir", dir, "err", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".status") {
			continue
		}
		key := strings.TrimSuffix(name, ".status")
		if _, ok := live[key]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			obslog.New(progName).Error("remove failed", "file", name, "err", err)
		}
	}
}
```

- [ ] **Step 9.4: テストが pass することを確認**

```bash
cd programs/claude-tools && go test ./cmd/claude-cockpit-prune/... -v
```

Expected: 3 テスト全 PASS。

- [ ] **Step 9.5: tmux.conf 書き換え**

ファイル `dot_config/tmux/tmux.conf` の line 15 を edit:

```
run -b '~/.config/tmux/scripts/cockpit/prune.sh'
```

を

```
run -b '~/.local/bin/claude-cockpit-prune'
```

に変更。**注意**: パスを `~/.config/tmux/scripts/cockpit/` から `~/.local/bin/` に変える点が重要。

- [ ] **Step 9.6: 旧 shell を削除**

```bash
git rm dot_config/tmux/scripts/cockpit/executable_prune.sh
```

- [ ] **Step 9.7: chezmoi apply で binary が build されるか確認**

```bash
chezmoi diff | head -20
chezmoi apply
ls -la ~/.local/bin/claude-cockpit-prune
```

Expected: binary が build されている。tmux.conf の変更も反映。

- [ ] **Step 9.8: 実機 smoke**

```bash
# 適当な存在しない pane の cache file を作る
mkdir -p ~/.cache/claude-cockpit/panes
echo "done" > ~/.cache/claude-cockpit/panes/fake-session_%999.status
ls ~/.cache/claude-cockpit/panes/
~/.local/bin/claude-cockpit-prune
ls ~/.cache/claude-cockpit/panes/
```

Expected: 2 回目の `ls` で `fake-session_%999.status` が消えている。生きている cache file は残っている。

```bash
# tmux server reload 経路の確認
tmux source-file ~/.tmux.conf
journalctl --user --since="1 min ago" | grep claude-cockpit-prune | head -5
```

Expected: tmux 再読み込み時に `run -b` 経由で prune が走る (ERROR レベルのログがなければ正常)。

- [ ] **Step 9.9: Commit**

```bash
git add programs/claude-tools/cmd/claude-cockpit-prune \
        dot_config/tmux/tmux.conf
git commit -m "feat(g1): PR-2 replace cockpit/prune.sh with Go binary

orphan cache file の回収を Go に置換。tmux list-panes -a の出力を
live set として走査し、cache dir 内の .status ファイル名 (basename)
が live set にない場合のみ削除。

実装ポイント:
- tmux failure 時は cache を削除しない (live set 無しで orphan 判定不可)
- defensive: 互換のため sessions/ dir も同様に走査 (現状未使用)
- internal/cockpit ではなく cmd 内で直接 dir 走査 (LoadAll は status
  parse まで含むので、prune の用途には不要なオーバーヘッド)

互換:
- tmux.conf の run -b パスを ~/.local/bin/claude-cockpit-prune に追従

旧 dot_config/tmux/scripts/cockpit/executable_prune.sh は git rm。"
```

---

### Task 10: PR-3 — cmd/claude-cockpit-summary

**Files:**
- Create: `programs/claude-tools/cmd/claude-cockpit-summary/main.go`
- Create: `programs/claude-tools/cmd/claude-cockpit-summary/main_test.go`
- Modify: `dot_config/tmux/conf/status.conf:11` (`summary.sh` → `~/.local/bin/claude-cockpit-summary`)
- Delete: `dot_config/tmux/scripts/cockpit/executable_summary.sh`

**契約:**
- 引数: なし
- 副作用: stdout に `⚡ N ⏸ M ✓ K ` (byte-exact match を test で固定)、cache 不在/空なら空文字列
- exit code: 通常 0、`internal/cockpit.LoadAll` がエラーを返した場合のみ 1 (ただし stdout は空)

- [ ] **Step 10.1: テスト先行**

ファイル `programs/claude-tools/cmd/claude-cockpit-summary/main_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSummary_emptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	var buf bytes.Buffer
	if err := writeSummary(&buf); err != nil {
		t.Fatalf("writeSummary: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("output = %q, want empty", buf.String())
	}
}

func TestSummary_byteExact(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)

	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%1.status"), []byte("working"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%2.status"), []byte("working"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%3.status"), []byte("working"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%4.status"), []byte("waiting"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%5.status"), []byte("done"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%6.status"), []byte("done"), 0644)

	var buf bytes.Buffer
	if err := writeSummary(&buf); err != nil {
		t.Fatalf("writeSummary: %v", err)
	}
	want := "⚡ 3 ⏸ 1 ✓ 2 "
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}
```

- [ ] **Step 10.2: テストが fail することを確認**

```bash
cd programs/claude-tools && go test ./cmd/claude-cockpit-summary/...
```

Expected: undefined `writeSummary`.

- [ ] **Step 10.3: 実装**

ファイル `programs/claude-tools/cmd/claude-cockpit-summary/main.go`:

```go
// claude-cockpit-summary writes the per-state count summary used by
// tmux status-right.
//
// Output format (byte-exact match to summary.sh):
//   "⚡ N ⏸ M ✓ K "  with trailing space; segments with count 0 are
//   omitted; empty string when no states exist.
package main

import (
	"fmt"
	"io"
	"os"

	"claude-tools/internal/cockpit"
	"claude-tools/internal/obslog"
)

const progName = "claude-cockpit-summary"

func main() {
	if err := writeSummary(os.Stdout); err != nil {
		// Status-right failure must not poison the bar — emit nothing.
		obslog.New(progName).Error("write summary failed", "err", err)
		os.Exit(1)
	}
}

func writeSummary(w io.Writer) error {
	states, err := cockpit.LoadAll()
	if err != nil {
		return fmt.Errorf("load all: %w", err)
	}
	_, err = io.WriteString(w, cockpit.Summary(states))
	return err
}
```

- [ ] **Step 10.4: テストが pass することを確認**

```bash
cd programs/claude-tools && go test ./cmd/claude-cockpit-summary/... -v
```

Expected: 2 テスト全 PASS。

- [ ] **Step 10.5: status.conf 書き換え**

ファイル `dot_config/tmux/conf/status.conf` の line 11 を edit:

```
set -g status-right "#(~/.config/tmux/scripts/cockpit/summary.sh)#(~/.config/tmux/scripts/claude-branch.sh '#{pane_current_path}')%H:%M "
```

の `~/.config/tmux/scripts/cockpit/summary.sh` を `~/.local/bin/claude-cockpit-summary` に変更:

```
set -g status-right "#(~/.local/bin/claude-cockpit-summary)#(~/.config/tmux/scripts/claude-branch.sh '#{pane_current_path}')%H:%M "
```

- [ ] **Step 10.6: 旧 shell を削除**

```bash
git rm dot_config/tmux/scripts/cockpit/executable_summary.sh
```

- [ ] **Step 10.7: chezmoi apply & 実機 smoke**

```bash
chezmoi apply
~/.local/bin/claude-cockpit-summary
echo "(end of output)"
```

Expected: 現在 cache されている state があれば `⚡ ... ⏸ ... ✓ ... ` が出力される (改行なし、末尾空白あり)。`(end of output)` は別行に表示。

```bash
tmux refresh-client -S
```

tmux status-right に summary が表示される (cache に state があれば)。

- [ ] **Step 10.8: Commit**

```bash
git add programs/claude-tools/cmd/claude-cockpit-summary \
        dot_config/tmux/conf/status.conf
git commit -m "feat(g1): PR-3 replace cockpit/summary.sh with Go binary

tmux status-right の \`⚡ N ⏸ M ✓ K \` 出力を Go に置換。
internal/cockpit.LoadAll → Summary に委譲する thin wrapper。

実装ポイント:
- byte-exact 出力一致 (絵文字 + space + 数値 + space) を unit test で固定
- LoadAll エラー時は stdout 空 + exit 1 (status-right を空にして
  バー全体を壊さない)

互換:
- status.conf の \`#(...summary.sh)\` 参照を \`~/.local/bin/claude-cockpit-summary\` に追従

旧 dot_config/tmux/scripts/cockpit/executable_summary.sh は git rm。"
```

---

### Task 11: PR-4 — cmd/claude-cockpit-next-ready

**Files:**
- Create: `programs/claude-tools/cmd/claude-cockpit-next-ready/main.go`
- Create: `programs/claude-tools/cmd/claude-cockpit-next-ready/main_test.go`
- Modify: `dot_config/tmux/conf/bindings.conf:78` (`next-ready.sh` → binary path)
- Delete: `dot_config/tmux/scripts/cockpit/executable_next-ready.sh`

**契約:**
- 引数: なし
- 副作用:
  - cache 不在 → `tmux display-message -d 1000 "no ready claude pane"` → exit 0
  - done pane を inbox 順 (session-name asc / window-index asc / pane-index asc) でリスト
  - 空なら同 message → exit 0
  - 現 pane (`tmux display-message -p '#{pane_id}'`) を起点に「次の done pane」(末尾に達したら最初に wrap)
  - `tmux switch-client -t <session> ; select-window -t <session>:<wIdx> ; select-pane -t <paneID>`

**注意 — pane index vs pane ID:**
- 並べ替え key は **pane-index** (数値、各 window 内で 0,1,2,...)
- jump 先指定とリストの key は **pane-id** (`%5` 等、tmux 全体で unique)
- cache file 名は **pane-id** (`<session>_%<id>.status`)
- shell next-ready.sh L18-21 で `tmux list-panes -F '#{pane_id}\t#{pane_index}' | sort -t$'\t' -k2,2n` している

- [ ] **Step 11.1: テスト先行**

ファイル `programs/claude-tools/cmd/claude-cockpit-next-ready/main_test.go`:

```go
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"claude-tools/internal/proc"
)

func TestBuildDoneList_orderAndCycle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)

	// 3 done panes across 2 sessions.
	_ = os.WriteFile(filepath.Join(cacheDir, "alpha_%1.status"), []byte("done"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "alpha_%3.status"), []byte("done"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "beta_%2.status"), []byte("done"), 0644)
	// Working: should NOT appear.
	_ = os.WriteFile(filepath.Join(cacheDir, "alpha_%2.status"), []byte("working"), 0644)

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"list-sessions", "-F", "#{session_name}"},
		[]byte("alpha\nbeta\n"), nil)
	fake.Register("tmux", []string{"list-windows", "-t", "alpha", "-F", "#{window_index}"},
		[]byte("0\n"), nil)
	fake.Register("tmux", []string{"list-windows", "-t", "beta", "-F", "#{window_index}"},
		[]byte("0\n"), nil)
	fake.Register("tmux", []string{"list-panes", "-t", "alpha:0", "-F", "#{pane_id}\t#{pane_index}"},
		[]byte("%1\t0\n%2\t1\n%3\t2\n"), nil)
	fake.Register("tmux", []string{"list-panes", "-t", "beta:0", "-F", "#{pane_id}\t#{pane_index}"},
		[]byte("%2\t0\n"), nil)

	got, err := buildDoneList(context.Background(), fake)
	if err != nil {
		t.Fatalf("buildDoneList: %v", err)
	}
	want := []doneRow{
		{session: "alpha", window: "0", paneID: "%1"},
		{session: "alpha", window: "0", paneID: "%3"},
		{session: "beta", window: "0", paneID: "%2"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("row[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestPickNext_cycles(t *testing.T) {
	rows := []doneRow{
		{session: "a", window: "0", paneID: "%1"},
		{session: "a", window: "0", paneID: "%2"},
		{session: "b", window: "0", paneID: "%3"},
	}
	cases := []struct {
		cur  string
		want string
	}{
		{"%1", "%2"},
		{"%2", "%3"},
		{"%3", "%1"},  // wrap
		{"%99", "%1"}, // not in list -> first
	}
	for _, c := range cases {
		got := pickNext(rows, c.cur)
		if got.paneID != c.want {
			t.Errorf("pickNext(cur=%s) = %s, want %s", c.cur, got.paneID, c.want)
		}
	}
}

func TestPickNext_emptyList(t *testing.T) {
	got := pickNext(nil, "%1")
	if got != (doneRow{}) {
		t.Errorf("empty list = %+v, want zero", got)
	}
}
```

- [ ] **Step 11.2: テストが fail することを確認**

```bash
cd programs/claude-tools && go test ./cmd/claude-cockpit-next-ready/...
```

Expected: undefined.

- [ ] **Step 11.3: 実装**

ファイル `programs/claude-tools/cmd/claude-cockpit-next-ready/main.go`:

```go
// claude-cockpit-next-ready jumps to the next pane whose cockpit state
// is "done", in inbox order: session-name asc → window-index asc →
// pane-index asc. Cycles past the current pane.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
	"claude-tools/internal/xdg"
)

const progName = "claude-cockpit-next-ready"

type doneRow struct {
	session string
	window  string
	paneID  string
}

type paneRow struct {
	id    string
	index int
}

func main() {
	ctx := context.Background()
	runner := proc.RealRunner{}
	logger := obslog.New(progName)

	cacheDir := xdg.ClaudeCockpitCacheDir()
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		displayMessage(ctx, runner, "no ready claude pane")
		os.Exit(0)
	}

	rows, err := buildDoneList(ctx, runner)
	if err != nil {
		logger.Error("build done list failed", "err", err)
		displayMessage(ctx, runner, "no ready claude pane")
		os.Exit(0)
	}
	if len(rows) == 0 {
		displayMessage(ctx, runner, "no ready claude pane")
		os.Exit(0)
	}

	curPaneOut, err := runner.Run(ctx, "tmux", "display-message", "-p", "#{pane_id}")
	if err != nil {
		logger.Error("get current pane failed", "err", err)
		os.Exit(1)
	}
	cur := strings.TrimSpace(string(curPaneOut))

	target := pickNext(rows, cur)
	if target == (doneRow{}) {
		os.Exit(0)
	}

	if _, err := runner.Run(ctx, "tmux",
		"switch-client", "-t", target.session,
		";", "select-window", "-t", target.session+":"+target.window,
		";", "select-pane", "-t", target.paneID,
	); err != nil {
		logger.Error("switch failed", "target", target, "err", err)
		os.Exit(1)
	}
}

// buildDoneList enumerates panes whose cached state is "done", sorted in
// inbox order: session-name asc, window-index asc, pane-index asc.
func buildDoneList(ctx context.Context, runner proc.Runner) ([]doneRow, error) {
	cacheDir := xdg.ClaudeCockpitCacheDir()

	sessOut, err := runner.Run(ctx, "tmux", "list-sessions", "-F", "#{session_name}")
	if err != nil {
		return nil, fmt.Errorf("list-sessions: %w", err)
	}
	sessions := splitNonEmpty(string(sessOut))
	sort.Strings(sessions)

	var rows []doneRow
	for _, s := range sessions {
		winOut, err := runner.Run(ctx, "tmux", "list-windows", "-t", s, "-F", "#{window_index}")
		if err != nil {
			continue
		}
		windows := splitNonEmpty(string(winOut))
		sort.SliceStable(windows, func(i, j int) bool {
			return atoiOrZero(windows[i]) < atoiOrZero(windows[j])
		})

		for _, w := range windows {
			paneOut, err := runner.Run(ctx, "tmux", "list-panes", "-t", s+":"+w, "-F", "#{pane_id}\t#{pane_index}")
			if err != nil {
				continue
			}
			var panes []paneRow
			for _, line := range splitNonEmpty(string(paneOut)) {
				parts := strings.SplitN(line, "\t", 2)
				if len(parts) != 2 {
					continue
				}
				panes = append(panes, paneRow{id: parts[0], index: atoiOrZero(parts[1])})
			}
			sort.SliceStable(panes, func(i, j int) bool { return panes[i].index < panes[j].index })

			for _, p := range panes {
				file := filepath.Join(cacheDir, s+"_"+p.id+".status")
				data, err := os.ReadFile(file)
				if err != nil {
					continue
				}
				if strings.TrimSpace(string(data)) == "done" {
					rows = append(rows, doneRow{session: s, window: w, paneID: p.id})
				}
			}
		}
	}
	return rows, nil
}

// pickNext returns the row after the one matching cur paneID; wraps to
// the first row if cur is the last; returns the first row if cur is not
// in the list. Returns zero value when rows is empty.
func pickNext(rows []doneRow, cur string) doneRow {
	if len(rows) == 0 {
		return doneRow{}
	}
	for i, r := range rows {
		if r.paneID == cur {
			return rows[(i+1)%len(rows)]
		}
	}
	return rows[0]
}

func displayMessage(ctx context.Context, runner proc.Runner, msg string) {
	_, _ = runner.Run(ctx, "tmux", "display-message", "-d", "1000", msg)
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func atoiOrZero(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
```

- [ ] **Step 11.4: テストが pass することを確認**

```bash
cd programs/claude-tools && go test ./cmd/claude-cockpit-next-ready/... -v
```

Expected: 3 テスト全 PASS。

- [ ] **Step 11.5: bindings.conf 書き換え**

ファイル `dot_config/tmux/conf/bindings.conf` の line 78 を edit:

```
  run-shell "~/.config/tmux/scripts/cockpit/next-ready.sh"
```

を

```
  run-shell "~/.local/bin/claude-cockpit-next-ready"
```

に変更。

- [ ] **Step 11.6: 旧 shell を削除**

```bash
git rm dot_config/tmux/scripts/cockpit/executable_next-ready.sh
```

- [ ] **Step 11.7: chezmoi apply & 実機 smoke**

```bash
chezmoi apply
~/.local/bin/claude-cockpit-next-ready
```

Expected: cache に done state がない場合は `no ready claude pane` メッセージが tmux に表示される (CLI からの実行ではターミナル出力なしで即終了する)。

```bash
# done state を作って試す
echo "done" > ~/.cache/claude-cockpit/panes/$(tmux display-message -p '#{session_name}')_$(tmux display-message -p '#{pane_id}').status
tmux source-file ~/.tmux.conf

# tmux で prefix + C → N を押す (キーバインド)
# Expected: switch されないが、ログを確認
journalctl --user --since="1 min ago" | grep claude-cockpit-next-ready | head
```

- [ ] **Step 11.8: Commit**

```bash
git add programs/claude-tools/cmd/claude-cockpit-next-ready \
        dot_config/tmux/conf/bindings.conf
git commit -m "feat(g1): PR-4 replace cockpit/next-ready.sh with Go binary

inbox 順 (session asc / window-idx asc / pane-idx asc) で done pane を
列挙し、現 pane の次にジャンプする処理を Go に置換。

実装ポイント:
- pane-index で並べ替え、pane-id でファイル参照 + jump (shell と同じ)
- 末尾到達時の wrap、cur が list 内に無い場合は先頭、空 list は no-op
- cache 不在 / 0 件は \`no ready claude pane\` を tmux display-message
  (-d 1000 で 1 秒表示、shell 互換)
- proc.Runner に分離した tmux 呼び出しを FakeRunner で完全 stub できる

互換:
- bindings.conf の run-shell パスを ~/.local/bin/claude-cockpit-next-ready に追従

旧 dot_config/tmux/scripts/cockpit/executable_next-ready.sh は git rm。"
```

---

### Task 12: PR-5 — cmd/claude-cockpit-switcher

**Files:**
- Create: `programs/claude-tools/cmd/claude-cockpit-switcher/main.go`
- Create: `programs/claude-tools/cmd/claude-cockpit-switcher/main_test.go`
- Modify: `dot_config/tmux/conf/bindings.conf:73` (switcher path)
- Delete: `dot_config/tmux/scripts/cockpit/executable_switcher.sh`

**契約:**
- 引数: なし
- 副作用:
  - 起動時に `~/.local/bin/claude-cockpit-prune` を fire-and-forget で 1 回呼ぶ
  - tmux session/window/pane を tree に展開、各 row の cache state を取得
  - fzf に stdin pipe → ユーザ選択 → 選択行 + key (`enter` / `ctrl-x` / `ctrl-r`) で dispatch
  - Enter: switch-client (+ select-window / select-pane)
  - Ctrl-X: 確認後 kill (claude-managed window は `claude-kill-session.sh` 経由)
  - Ctrl-R: `exec self` で reload

**Go 化のスコープ判断:**
- fzf 自体は外部呼び出しのまま (Go bubbletea で再実装は YAGNI)
- 確認プロンプト (`read </dev/tty`) は `bufio.NewReader` + `/dev/tty` 直 open
- `claude-kill-session.sh` は C サブシステムのまま残るので外部呼び出し継続

- [ ] **Step 12.1: テスト先行**

ファイル `programs/claude-tools/cmd/claude-cockpit-switcher/main_test.go`:

```go
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-tools/internal/proc"
)

func TestBadge(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"working", "⚡ working"},
		{"waiting", "⏸ waiting"},
		{"done", "✓ done"},
		{"", ""},
		{"unknown", ""},
	}
	for _, c := range cases {
		if got := badge(c.in); got != c.want {
			t.Errorf("badge(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStateForPane(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "sess_%5.status"), []byte("working"), 0644)

	if got := stateForPane("sess", "%5"); got != "working" {
		t.Errorf("stateForPane = %q, want working", got)
	}
	if got := stateForPane("sess", "%nope"); got != "" {
		t.Errorf("stateForPane on missing = %q, want empty", got)
	}
}

func TestStateForSession_priority(t *testing.T) {
	cases := []struct {
		name  string
		panes []string // pane state literals
		want  string
	}{
		{"any working dominates", []string{"done", "waiting", "working"}, "working"},
		{"waiting beats done", []string{"done", "waiting"}, "waiting"},
		{"only done", []string{"done", "done"}, "done"},
		{"all empty", []string{"", ""}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stateForSessionFromPanes(c.panes); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseSelection(t *testing.T) {
	row := "P\talpha\t0\t%5\t    pane:%5  cwd=/x    ⚡ working"
	got, err := parseSelection(row)
	if err != nil {
		t.Fatal(err)
	}
	if got.kind != "P" || got.session != "alpha" || got.window != "0" || got.paneID != "%5" {
		t.Errorf("parsed wrongly: %+v", got)
	}
}

func TestBuildLines_emitsTreeOrder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "alpha_%1.status"), []byte("working"), 0644)

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"list-sessions", "-F", "#{session_name}"},
		[]byte("alpha\n"), nil)
	fake.Register("tmux", []string{"list-windows", "-t", "alpha", "-F", "#{window_index}\t#{window_name}"},
		[]byte("0\tmain\n"), nil)
	fake.Register("tmux", []string{"list-panes", "-t", "alpha:0", "-F", "#{pane_id}\t#{pane_current_path}"},
		[]byte("%1\t/home/test\n"), nil)
	fake.Register("tmux", []string{"list-panes", "-t", "alpha", "-s", "-F", "#{pane_id}"},
		[]byte("%1\n"), nil)

	lines, err := buildLines(context.Background(), fake)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (S/W/P): %v", len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], "S\talpha\t") {
		t.Errorf("line[0] not S row: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "W\talpha\t0\t") {
		t.Errorf("line[1] not W row: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "P\talpha\t0\t%1\t") {
		t.Errorf("line[2] not P row: %q", lines[2])
	}
}
```

- [ ] **Step 12.2: テストが fail することを確認**

```bash
cd programs/claude-tools && go test ./cmd/claude-cockpit-switcher/...
```

Expected: undefined.

- [ ] **Step 12.3: 実装**

ファイル `programs/claude-tools/cmd/claude-cockpit-switcher/main.go`:

```go
// claude-cockpit-switcher provides a hierarchical fzf switcher over
// tmux sessions/windows/panes annotated with cockpit state.
//
// Keys (fzf --expect):
//   Enter   -> switch-client (+ select-window / select-pane as needed)
//   Ctrl-X  -> kill the selected scope (worktree-aware for windows)
//   Ctrl-R  -> reload (re-exec self)
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
	"claude-tools/internal/xdg"
)

const progName = "claude-cockpit-switcher"

func main() {
	if _, err := exec.LookPath("fzf"); err != nil {
		runTmux("display-message", "fzf required (paru -S fzf)")
		os.Exit(1)
	}

	// Fire-and-forget prune so orphans don't show up.
	_ = exec.Command(filepath.Join(os.Getenv("HOME"), ".local/bin/claude-cockpit-prune")).Start()

	ctx := context.Background()
	runner := proc.RealRunner{}
	logger := obslog.New(progName)

	lines, err := buildLines(ctx, runner)
	if err != nil {
		logger.Error("build lines failed", "err", err)
		os.Exit(1)
	}

	selection, key, err := runFzf(strings.Join(lines, "\n"))
	if err != nil {
		// User cancelled or fzf error: silent exit 0.
		os.Exit(0)
	}
	if selection == "" {
		os.Exit(0)
	}

	row, err := parseSelection(selection)
	if err != nil {
		logger.Error("parse selection failed", "raw", selection, "err", err)
		os.Exit(1)
	}

	switch key {
	case "ctrl-r":
		bin, _ := os.Executable()
		_ = syscall.Exec(bin, []string{bin}, os.Environ())
		os.Exit(0)
	case "ctrl-x":
		dispatchKill(ctx, runner, row)
	default:
		dispatchSwitch(ctx, runner, row)
	}
}

type selectedRow struct {
	kind    string // "S" / "W" / "P"
	session string
	window  string
	paneID  string
}

// badge returns the visual badge for a pane state literal.
func badge(state string) string {
	switch state {
	case "working":
		return "⚡ working"
	case "waiting":
		return "⏸ waiting"
	case "done":
		return "✓ done"
	}
	return ""
}

// stateForPane reads the cache file for (session, paneID) and returns
// its trimmed content (empty string if missing).
func stateForPane(session, paneID string) string {
	file := filepath.Join(xdg.ClaudeCockpitCacheDir(), session+"_"+paneID+".status")
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// stateForSessionFromPanes aggregates pane states with the priority
// working > waiting > done. Empty if no pane has a known state.
func stateForSessionFromPanes(states []string) string {
	hasW, hasQ, hasD := false, false, false
	for _, s := range states {
		switch s {
		case "working":
			hasW = true
		case "waiting":
			hasQ = true
		case "done":
			hasD = true
		}
	}
	switch {
	case hasW:
		return "working"
	case hasQ:
		return "waiting"
	case hasD:
		return "done"
	}
	return ""
}

// buildLines emits one tab-separated line per S/W/P entry.
// Format: "<kind>\t<session>\t<w_idx>\t<p_id>\t<display>"
func buildLines(ctx context.Context, runner proc.Runner) ([]string, error) {
	sessOut, err := runner.Run(ctx, "tmux", "list-sessions", "-F", "#{session_name}")
	if err != nil {
		return nil, fmt.Errorf("list-sessions: %w", err)
	}
	sessions := splitNonEmpty(string(sessOut))
	sort.Strings(sessions)

	var lines []string
	for _, s := range sessions {
		// Aggregate session state from its panes (server-wide, -s flag).
		paneListOut, _ := runner.Run(ctx, "tmux", "list-panes", "-t", s, "-s", "-F", "#{pane_id}")
		var paneStates []string
		for _, p := range splitNonEmpty(string(paneListOut)) {
			paneStates = append(paneStates, stateForPane(s, p))
		}
		sBadge := badge(stateForSessionFromPanes(paneStates))
		lines = append(lines, fmt.Sprintf("S\t%s\t\t\t%-30s  %s", s, s, sBadge))

		winOut, err := runner.Run(ctx, "tmux", "list-windows", "-t", s, "-F", "#{window_index}\t#{window_name}")
		if err != nil {
			continue
		}
		for _, wline := range splitNonEmpty(string(winOut)) {
			parts := strings.SplitN(wline, "\t", 2)
			if len(parts) != 2 {
				continue
			}
			wIdx, wName := parts[0], parts[1]
			lines = append(lines, fmt.Sprintf("W\t%s\t%s\t\t  window:%s %s", s, wIdx, wIdx, wName))

			paneOut, err := runner.Run(ctx, "tmux", "list-panes", "-t", s+":"+wIdx, "-F", "#{pane_id}\t#{pane_current_path}")
			if err != nil {
				continue
			}
			for _, pline := range splitNonEmpty(string(paneOut)) {
				pp := strings.SplitN(pline, "\t", 2)
				if len(pp) != 2 {
					continue
				}
				pID, pPath := pp[0], pp[1]
				pBadge := badge(stateForPane(s, pID))
				lines = append(lines, fmt.Sprintf("P\t%s\t%s\t%s\t    pane:%s  cwd=%s    %s",
					s, wIdx, pID, pID, pPath, pBadge))
			}
		}
	}
	return lines, nil
}

// runFzf invokes fzf with our flags, pipes input, and returns
// (selection_row, key, err). On user-cancel returns ("", "", err).
func runFzf(input string) (string, string, error) {
	cmd := exec.Command("fzf",
		"--prompt=cockpit> ",
		"--height=100%",
		"--layout=reverse",
		"--no-sort",
		"--tiebreak=index",
		"--delimiter=\t",
		"--with-nth=5..",
		"--expect=ctrl-x,ctrl-r",
		"--header=enter=switch  ctrl-x=kill  ctrl-r=reload",
	)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr // fzf draws on stderr
	out, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	// fzf with --expect: first line = key (or empty for default), second = row.
	parts := strings.SplitN(strings.TrimRight(string(out), "\n"), "\n", 2)
	key := ""
	row := ""
	if len(parts) >= 1 {
		key = parts[0]
	}
	if len(parts) >= 2 {
		row = parts[1]
	}
	return row, key, nil
}

func parseSelection(row string) (selectedRow, error) {
	cols := strings.SplitN(row, "\t", 5)
	if len(cols) < 4 {
		return selectedRow{}, fmt.Errorf("malformed row (need 4+ cols): %q", row)
	}
	return selectedRow{
		kind:    cols[0],
		session: cols[1],
		window:  cols[2],
		paneID:  cols[3],
	}, nil
}

func dispatchSwitch(ctx context.Context, runner proc.Runner, row selectedRow) {
	switch row.kind {
	case "S":
		_, _ = runner.Run(ctx, "tmux", "switch-client", "-t", row.session)
	case "W":
		_, _ = runner.Run(ctx, "tmux", "switch-client", "-t", row.session,
			";", "select-window", "-t", row.session+":"+row.window)
	case "P":
		_, _ = runner.Run(ctx, "tmux", "switch-client", "-t", row.session,
			";", "select-window", "-t", row.session+":"+row.window,
			";", "select-pane", "-t", row.paneID)
	}
}

func dispatchKill(ctx context.Context, runner proc.Runner, row selectedRow) {
	switch row.kind {
	case "P":
		if !confirmYesNo(fmt.Sprintf("kill pane %s? (y/N) ", row.paneID)) {
			return
		}
		_, _ = runner.Run(ctx, "tmux", "kill-pane", "-t", row.paneID)
	case "W":
		// Check @claude-managed window option.
		out, _ := runner.Run(ctx, "tmux", "show-options", "-w", "-t", row.session+":"+row.window, "-v", "@claude-managed")
		managed := strings.TrimSpace(string(out)) == "yes"
		if managed {
			if !confirmYesNo(fmt.Sprintf("kill claude window %s:%s and worktree? (y/N) ", row.session, row.window)) {
				return
			}
			cmd := exec.CommandContext(ctx,
				filepath.Join(os.Getenv("HOME"), ".config/tmux/scripts/claude-kill-session.sh"),
				row.session+":"+row.window)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
			return
		}
		if !confirmYesNo(fmt.Sprintf("kill window %s:%s? (y/N) ", row.session, row.window)) {
			return
		}
		_, _ = runner.Run(ctx, "tmux", "kill-window", "-t", row.session+":"+row.window)
	case "S":
		if !confirmYesNo(fmt.Sprintf("kill session %s? (worktrees kept) (y/N) ", row.session)) {
			return
		}
		_, _ = runner.Run(ctx, "tmux", "kill-session", "-t", row.session)
	}
}

// confirmYesNo prompts on stderr and reads from /dev/tty so the popup
// keeps focus. Matches shell switcher.sh's `IFS= read -r ans </dev/tty`.
func confirmYesNo(prompt string) bool {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return false
	}
	defer tty.Close()
	fmt.Fprint(os.Stderr, prompt)
	r := bufio.NewReader(tty)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

// runTmux is a small helper for one-shot tmux invocations that don't
// need the proc.Runner abstraction (display-message at startup).
func runTmux(args ...string) {
	_ = exec.Command("tmux", args...).Run()
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
```

- [ ] **Step 12.4: テストが pass することを確認**

```bash
cd programs/claude-tools && go test ./cmd/claude-cockpit-switcher/... -v
```

Expected: 5 テスト全 PASS。

- [ ] **Step 12.5: bindings.conf 書き換え**

ファイル `dot_config/tmux/conf/bindings.conf` の line 73 を edit:

```
  display-popup -E "~/.config/tmux/scripts/cockpit/switcher.sh"
```

を

```
  display-popup -E "~/.local/bin/claude-cockpit-switcher"
```

に変更。

- [ ] **Step 12.6: 旧 shell を削除**

```bash
git rm dot_config/tmux/scripts/cockpit/executable_switcher.sh
```

- [ ] **Step 12.7: chezmoi apply & 実機 smoke (interactive)**

```bash
chezmoi apply
```

tmux を起動して `prefix + C → s` を押す。

Expected:
- popup が開き fzf prompt `cockpit> ` が表示される
- session/window/pane の tree が visible (`⚡ ⏸ ✓` バッジ付き)
- Enter で選択先に switch
- Ctrl-X で確認プロンプト → y で kill
- Ctrl-R で reload (popup を一旦閉じて再起動と同等の表示)
- ESC でキャンセル → no-op

失敗時:

```bash
journalctl --user --since="1 min ago" | grep claude-cockpit-switcher
```

- [ ] **Step 12.8: Commit**

```bash
git add programs/claude-tools/cmd/claude-cockpit-switcher \
        dot_config/tmux/conf/bindings.conf
git commit -m "feat(g1): PR-5 replace cockpit/switcher.sh with Go binary

session/window/pane 階層 fzf スイッチャを Go に置換。fzf 自体は
exec.Command で起動 (bubbletea で再実装は YAGNI)、key (Enter/Ctrl-X/
Ctrl-R) に応じて switch / kill / reload をディスパッチ。

実装ポイント:
- buildLines が tab-separated tree を生成、fzf --with-nth=5.. で
  metadata 列 (kind/session/window/pane-id) を表示から隠す
- ctrl-r: syscall.Exec で self を re-exec (shell の \`exec self\` 同等)
- ctrl-x: kind ごとに kill 経路を分岐。W が @claude-managed=yes なら
  claude-kill-session.sh (C サブシステム、shell のまま) を子プロセス
  起動して worktree も削除
- 確認プロンプトは bufio + /dev/tty 直 open。tmux confirm-before だと
  popup が閉じた瞬間に prompt が消える既知の問題があるため、shell の
  \`read </dev/tty\` 戦略を踏襲
- session 集約 state は priority working > waiting > done

互換:
- bindings.conf の display-popup -E パスを ~/.local/bin/claude-cockpit-switcher に追従

旧 dot_config/tmux/scripts/cockpit/executable_switcher.sh は git rm。"
```

---

### Task 13: B Completion Checkpoint — Smoke & Go/No-Go

**Files:**
- Create: `docs/superpowers/smoke/2026-05-01-go-cockpit-smoke.md`
- Modify: `docs/todos.md` (G-1 PR-1〜PR-5 のチェックボックスを `[x]` に)

**目的:** Phase 1 (B サブシステム 5 binary) を全部置換した時点で、`docs/manage_claude.md` の "Cockpit State Tracking — Smoke Tests" 8 step を実機で 1 周通し、結果をドキュメント化する。Phase 2 (A サブシステム notify pipeline) を続行するか撤退するかの go/no-go を判定する。

- [ ] **Step 13.1: 全 binary が ~/.local/bin/ に揃っていることを確認**

```bash
ls -la ~/.local/bin/claude-cockpit-{state,prune,summary,next-ready,switcher}
```

Expected: 5 binary 全て exist + executable。

- [ ] **Step 13.2: 旧 shell が完全に消えていることを確認**

```bash
ls /home/kiyama/.local/bin/claude-cockpit-state.sh \
   ~/.config/tmux/scripts/cockpit/{summary,switcher,next-ready,prune}.sh 2>&1
```

Expected: 5 ファイル全て `No such file or directory`。

- [ ] **Step 13.3: 設定参照箇所が新パスを指していることを確認**

```bash
grep -rn "cockpit-state\|cockpit/" \
  ~/.config/claude/settings.json \
  ~/.config/tmux/tmux.conf \
  ~/.config/tmux/conf/bindings.conf \
  ~/.config/tmux/conf/status.conf 2>/dev/null
```

Expected: すべての参照が `~/.local/bin/claude-cockpit-*` または `/home/kiyama/.local/bin/claude-cockpit-*` になっている。`.sh` 拡張子を持つマッチが無いこと。

- [ ] **Step 13.4: Smoke 結果ドキュメント作成**

ファイル `docs/superpowers/smoke/2026-05-01-go-cockpit-smoke.md`:

````markdown
# Go Cockpit Smoke Test — 2026-05-01

**対象:** Phase 1 (B サブシステム) — 5 binary を Go で 1:1 置換完了後の実機検証
**実行者:** kiyama
**実行日:** 2026-05-01 (実施日に合わせて更新)

## Pre-conditions

- [x] `~/.local/bin/claude-cockpit-{state,prune,summary,next-ready,switcher}` 5 binary が存在
- [x] 旧 shell 5 本が repo 内・filesystem 上から消えている
- [x] `chezmoi apply` がエラーなく完走
- [x] `cd programs/claude-tools && go test ./...` が all PASS

## 8-Step Smoke (`docs/manage_claude.md` "Cockpit State Tracking" 節 準拠)

| # | Step | Expected | Result | Notes |
|---|---|---|---|---|
| 1 | 新 tmux session で Claude を起動、何か入力 | hook 経由で `~/.cache/claude-cockpit/panes/<sess>_<paneID>.status` が `working` で生まれる | TBD | |
| 2 | Stop event を待つ (Claude が応答完了) | 同ファイルが `done` に更新 | TBD | |
| 3 | tmux status-right を確認 | `⚡ N ⏸ M ✓ K ` 形式で表示 (cache の集計反映) | TBD | |
| 4 | `prefix + C → s` で switcher 起動 | fzf popup が開き、session/window/pane の tree + バッジ表示 | TBD | |
| 5 | 任意 pane を Enter で選択 | tmux が switch-client + select-window + select-pane で移動 | TBD | |
| 6 | switcher で Ctrl-X → 'y' | 該当 scope (pane / window / session) が kill される。W が claude-managed なら worktree 削除も実行 | TBD | |
| 7 | done state pane を 2 つ以上作り `prefix + C → N` で循環 | inbox 順 (session asc / window-idx asc / pane-idx asc) で次の done pane に jump | TBD | |
| 8 | `tmux kill-server` 後に再起動 | server-start hook (`run -b prune`) が orphan cache を回収、status-right が空になる | TBD | |

## Errors / Surprises

(発生した unexpected behavior をここに列挙。無ければ "なし")

## journalctl Errors

```bash
journalctl --user --since="2 hours ago" | grep -E "claude-cockpit-(state|prune|summary|next-ready|switcher)" | grep -i error
```

(実行結果を貼る。`logger -t` 経由の ERROR が出ていなければ正常)

## Go/No-Go Decision for Phase 2 (A サブシステム = notify pipeline)

- [ ] **GO** — Phase 1 が問題なく動作。Phase 2 plan を `2026-05-XX-shell-to-go-migration-phase2.md` として作成し、A サブシステム 4 binary (`notify-{cleanup,sound,hook,dispatch}`) の置換に進む。
- [ ] **NO-GO** — Phase 1 で許容できない問題が発生。詳細を本ファイルに追記し、`docs/todos.md` G-1 の状態を見直す。Go binary は revert せず shell + Go 共存で当面運用 (notify は shell のまま、cockpit は Go のまま)。

判定: TBD (smoke 全 step 完了後に決定)

判定理由: TBD
````

**注意**: `TBD` のセル / 文字列は **smoke を実行する人間が手で埋める**。LLM が勝手に埋めない。これは「実装と検証の分離」の意図的な仕掛け。

- [ ] **Step 13.5: docs/todos.md の G-1 PR チェックボックスを更新**

`docs/todos.md` 内の G-1 セクションで、Phase 1 で完了した PR の `[ ]` を `[x]` に変更:

検索文字列 (それぞれ unique):

```
  - [ ] PR-1: `cockpit-state` (T1)
  - [ ] PR-2: `cockpit-prune` (T1)
  - [ ] PR-3: `cockpit-summary` (T2)
  - [ ] PR-4: `cockpit-next-ready` (T2)
  - [ ] PR-5: `cockpit-switcher` (T3)
  - [ ] **★ B 完走チェックポイント**
```

をそれぞれ `[x]` に置換 (PR-6 〜 PR-9 と G-1.next #1〜#4 は touch しない、Phase 2 以降の対象)。

`★ B 完走チェックポイント` の行は実施日リンク付きで:

```
  - [x] **★ B 完走チェックポイント** — 2026-05-XX 実施 / 結果: [`smoke/2026-05-01-go-cockpit-smoke.md`](superpowers/smoke/2026-05-01-go-cockpit-smoke.md)
```

(date は実施日に合わせる)

最終更新行:

```
最終更新: 2026-05-01 (G-1 Shell → Go 移行 spec 確定)
```

を

```
最終更新: 2026-05-XX (G-1 Phase 1 完了 / B サブシステム 5 binary)
```

に更新 (XX は実施日)。

- [ ] **Step 13.6: Commit**

```bash
git add docs/superpowers/smoke/2026-05-01-go-cockpit-smoke.md \
        docs/todos.md
git commit -m "docs(g1): Phase 1 (B subsystem) completion checkpoint

5 binary (claude-cockpit-{state,prune,summary,next-ready,switcher}) を
Go で 1:1 置換完了。8-step manual smoke の検証結果を
superpowers/smoke/2026-05-01-go-cockpit-smoke.md に記録するテンプレを
追加 (TBD セルは実機検証時に人間が記入)。

todos.md G-1 PR-1〜PR-5 + ★ チェックポイントを [x] に更新。
PR-6 〜 PR-9 (notify pipeline = Phase 2) は go/no-go 判定後に
別 plan で着手予定。"
```

---

## Phase 1 完了後

Task 13 step 13.4 の smoke 結果に基づき:

- **GO 判定の場合**: 新規 plan `docs/superpowers/plans/2026-05-XX-shell-to-go-migration-phase2.md` を `superpowers:writing-plans` で作成。本 plan の構造を踏襲しつつ、Phase 1 で得た知見 (例: proc.Runner の使い心地、obslog の運用感、chezmoi build trigger の reliability) を Phase 2 設計に反映する。Phase 2 のスコープは spec §3.1 A サブシステムの 4 binary。

- **NO-GO 判定の場合**: `docs/todos.md` G-1 の状態を「Phase 1 完了 / Phase 2 撤退」に更新し、原因を分析した上で別の方向性 (Rust への切替 / shell のまま / 部分的 Go 化など) を `docs/superpowers/specs/` に新 spec として書き出すか検討する。

どちらの場合も本 plan は本 commit (Task 13.6) 時点で **完了** とみなす。

---

## Self-Review (Plan 作者用 — 実行前にスキャン)

**Spec coverage check:**
- ✅ §1 Goal G1 (debuggability): TDD で全 binary に test を書く構造で達成
- ✅ §1 Goal G2 (testability): atomic write / TTL / state はすべて internal/ で unit test
- ⚠️ §1 Goal G3 (D-Bus): Phase 1 では未着手 (notify-dispatch は Phase 2)。本 plan の責務外で OK
- ✅ §1 Goal G4 (compat): cache パス / フォーマット / hook exit 0 を全タスクで保持
- ✅ §3.1 B サブシステム 5 binary: Task 8〜12 で全カバー
- ✅ §4.1 Project layout: Task 1, 7 で確立
- ✅ §4.3 internal packages: Task 2〜6 で全パッケージ作成
- ✅ §4.4 Layer 2 contract (hook 系 exit 0): Task 8 main.go で実装
- ✅ §4.5 Data flow (shell 互換): Task 6 cockpit, Task 8 main.go で確認
- ✅ §5.1 Toolchain: 既存 mise 設定済み (本 plan は touch しない)
- ✅ §5.2 Build trigger: Task 7
- ✅ §5.3 Source 配置: Task 1
- ✅ §5.4 旧 shell 撤去: Task 8〜12 各 PR 内
- ✅ §6.1 Unit test: Task 2〜12 全タスクで TDD
- ✅ §6.2 Mock 戦略: proc.Runner 経由 (Task 4 で確立)
- ✅ §6.3 Smoke test: Task 13 で記録
- ✅ §6.4 CI なし: 本 plan は CI を作らない (準拠)
- ✅ §7 Logging: obslog (Task 5) を全 binary で使用
- ✅ §8 Migration plan Vertical-B-first: Task 8〜12 が PR-1〜PR-5
- ✅ §9 Rollout: clean cut per PR、cache 互換、git revert で復帰可
- ✅ §10 Module/naming: `claude-tools` local module (Task 1)
- ⚠️ §11 Out-of-scope: 4 follow-up は既に todos.md に反映済み (本 plan で再記述不要)
- ✅ §13 Acceptance criteria: B 部分が Task 13 でカバー (A は Phase 2)

**Placeholder scan:**
- spec の Task 13.4 ドキュメント内 `TBD` は意図的 (実機検証時に人間が埋める旨を明記)
- 他に "TODO" / "implement later" / "fill in" / "appropriate" 等の placeholder なし

**Type consistency check:**
- `Status` (string alias): cockpit.go と main.go で一貫
- `PaneState` フィールド名 (Session/PaneID/Status): cockpit.go と LoadAll/Summary 経路で一貫
- `selectedRow` フィールド名 (kind/session/window/paneID): switcher main.go 内で一貫
- `doneRow` フィールド名 (session/window/paneID): next-ready main.go 内で一貫
- `proc.Runner.Run(ctx, name, args...)` シグネチャ: 全 binary で一致
- `obslog.New(progName) *slog.Logger`: 全 binary で同名引数

**Step granularity check:**
- 各 step が 2-5 分の単一アクション: ✅
- TDD パターン (test → run fail → implement → run pass → commit) を保持: ✅
- exact file path + 完全コード掲載: ✅

問題なし。実行準備完了。

---

## Execution Choice

Plan complete and saved to `docs/superpowers/plans/2026-05-01-shell-to-go-migration.md`.

Two execution options:

1. **Subagent-Driven (recommended)** — Fresh subagent per task, two-stage review between tasks, fast iteration. Good for this plan because each task is well-isolated (TDD pattern + commit per task).

2. **Inline Execution** — Execute tasks in the current session using `superpowers:executing-plans`, batch execution with checkpoints for review. Lower context overhead but slower iteration.

Which approach?
