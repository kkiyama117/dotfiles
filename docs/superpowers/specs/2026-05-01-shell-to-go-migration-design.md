# Shell → Go 移行 — Claude Tools サブシステム (Design Spec)

- **Date**: 2026-05-01
- **Status**: Approved (brainstorming complete, ready for implementation plan)
- **Owner**: kiyama
- **Repo**: chezmoi (`/home/kiyama/.local/share/chezmoi`)
- **Related**:
  - [`2026-04-30-claude-cockpit-state-tracking-design.md`](./2026-04-30-claude-cockpit-state-tracking-design.md) (B サブシステムの仕様根拠)
  - [`2026-04-30-wired-click-actions-design.md`](./2026-04-30-wired-click-actions-design.md) (A サブシステムの仕様根拠)
  - `docs/todos.md` F-3.next / F-4 / F-5.next (本 spec から派生する follow-up を集約)

---

## 1. Goal

`dot_local/bin/` + `dot_config/tmux/scripts/cockpit/` に蓄積した shell script 群 (~430 行 / 9 ファイル) を **Go** で 1:1 置換する。Rust は将来比較対象として保留 (`docs/todos.md`)。

達成すること:

- **G1 デバッガビリティ**: `set -e` / `trap` / サブシェルの暗黙挙動依存を排除し、stack trace と型で挙動を追えるようにする
- **G2 正しさのテスト化**: atomic write / mtime TTL / `replace-id` state の round-trip を unit test で固める (現状は手動 smoke のみ)
- **G3 D-Bus / setsid 周りの再現性**: `notify-send --print-id --wait` + `gdbus monitor` の連動部分を Go で構造化し、popup state machine の状態遷移を読めるコードにする
- **G4 互換維持**: cache / state ファイルのパス・フォーマット・hook の `exit 0` 絶対契約を完全踏襲。runtime 状態は shell ↔ Go 移行で連続する

## 2. Non-Goals

- **C サブシステム (installer / `.chezmoiscripts/run_*.sh.tmpl` / `tpm-bootstrap.sh`)** の Go 化 — chezmoi template 結合が深い
- **`notify-dispatch` の daemon 化** — 1:1 置換に専念。daemon 集約 (F-3.next L33) は別 spec
- **Rust 実装** — Go 完走後の学習比較
- **Windows / WSL / macOS 対応** — tmux + D-Bus + wired + systemd + xdotool/swaymsg 全て Linux 専用領域につき不要
- **Claude Code 以外の hook 対応** (Codex CLI など)
- **CI / Docker / GitHub Actions** — 個人 dotfile につき YAGNI
- **アーキテクチャ刷新** — 「shell を Go に置き換える」のみ。daemon 化 / 状態モデル変更 / state file レイアウト変更は本 spec のスコープ外

## 3. Scope

### 3.1 移行対象 (9 binary)

**A サブシステム — notify pipeline (4 binary)**

| 旧 shell | 新 Go binary | 役割 |
|---|---|---|
| `dot_local/bin/executable_claude-notify-hook.sh` | `cmd/claude-notify-hook` | Claude hook orchestrator (env → dispatcher 起動 + sound worker 起動) |
| `dot_local/bin/executable_claude-notify-dispatch.sh` | `cmd/claude-notify-dispatch` | popup 起動 + D-Bus action loop (1 popup = 1 process) |
| `dot_local/bin/executable_claude-notify-sound.sh` | `cmd/claude-notify-sound` | 通知音再生 |
| `dot_local/bin/executable_claude-notify-cleanup.sh` | `cmd/claude-notify-cleanup` | state file TTL 剪定 (systemd timer から daily 起動) |

**B サブシステム — cockpit (5 binary)**

| 旧 shell | 新 Go binary | 役割 |
|---|---|---|
| `dot_local/bin/executable_claude-cockpit-state.sh` | `cmd/claude-cockpit-state` | Claude hook → 三値 state (working/waiting/done) を atomic write |
| `dot_config/tmux/scripts/cockpit/executable_summary.sh` | `cmd/claude-cockpit-summary` | tmux status-right `⚡N ⏸M ✓K ` を出力 |
| `dot_config/tmux/scripts/cockpit/executable_switcher.sh` | `cmd/claude-cockpit-switcher` | session/window/pane 階層 fzf スイッチャ |
| `dot_config/tmux/scripts/cockpit/executable_next-ready.sh` | `cmd/claude-cockpit-next-ready` | inbox 順で done pane に循環ジャンプ |
| `dot_config/tmux/scripts/cockpit/executable_prune.sh` | `cmd/claude-cockpit-prune` | tmux 側で消えた pane の orphan cache 回収 |

### 3.2 移行対象外 (shell + chezmoi template のまま据え置き)

- C サブシステム: `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl`, `.chezmoiscripts/run_onchange_after_enable-claude-notify-cleanup.sh.tmpl`, `dot_config/tmux/scripts/{tpm-bootstrap,tmux-claude-new,claude-kill-session,claude-respawn-pane,claude-pick-branch,claude-branch}.sh`
- ただし C のうち **bootstrap で Go を build するため** の `.chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl` は本 spec で新設する

### 3.3 ターゲット

- OS: **Linux/amd64 のみ**
- 配布対象: ローカル開発機 1 台 (`~kiyama@manjaro`) + 同等 Manjaro/WSL マシン

## 4. Architecture

### 4.1 Project layout

```
programs/claude-tools/                 # chezmoi 管理外 (.chezmoiignore で除外)
├── go.mod                             # module: claude-tools (local module, no go.dev path)
├── go.sum
├── README.md
├── cmd/                               # Layer 2 — N 個の小バイナリ (1 binary = 1 旧 shell)
│   ├── claude-cockpit-state/main.go
│   ├── claude-cockpit-summary/main.go
│   ├── claude-cockpit-switcher/main.go
│   ├── claude-cockpit-next-ready/main.go
│   ├── claude-cockpit-prune/main.go
│   ├── claude-notify-hook/main.go
│   ├── claude-notify-dispatch/main.go
│   ├── claude-notify-sound/main.go
│   └── claude-notify-cleanup/main.go
└── internal/                          # Layer 1 — 共有パッケージ
    ├── cockpit/                       # B 固有: Status enum / cache layout / LoadAll
    ├── notify/                        # A 固有: Popup / replace-id state / D-Bus action loop
    ├── xdg/                           # RuntimeDir / CacheDir 解決 (env優先 + fallback)
    ├── atomicfile/                    # tmp + rename atomic write
    ├── proc/                          # os/exec wrapper (Runner interface, テスト fake 注入用)
    └── obslog/                        # slog + logger -t passthrough
```

### 4.2 2-Layer 構成 (codex + claude 合意)

- **Layer 1 (`internal/`)**: 共有パッケージ。テストの主戦場。バイナリには出ない (Go の internal/ 慣行通り `programs/claude-tools` 配下からのみ import 可能)
- **Layer 2 (`cmd/`)**: 各 main.go は 30〜80 行程度の thin entry point。flag parse → `internal/*` 呼び出し → exit。ビジネスロジックは Layer 2 には書かない

### 4.3 共有パッケージ詳細

#### `internal/xdg`

```go
package xdg

func RuntimeDir() string          // ${XDG_RUNTIME_DIR:-/tmp} 慣行を踏襲
func CacheDir() string            // ${XDG_CACHE_HOME:-${HOME}/.cache}
func ClaudeNotifyStateDir() string // RuntimeDir() + "/claude-notify/sessions"
func ClaudeCockpitCacheDir() string // CacheDir() + "/claude-cockpit/panes"
```

#### `internal/atomicfile`

```go
package atomicfile

// Write writes data to path atomically via tmp + rename.
// dir mkdir is caller's responsibility (errors propagate).
func Write(path string, data []byte, perm os.FileMode) error
```

挙動: `os.CreateTemp(filepath.Dir(path), ".tmp.*")` → `f.Write(data)` → `f.Close()` → `os.Rename(tmp, path)`. tmp が残らないよう失敗時は `defer os.Remove(tmp)` で必ず cleanup.

#### `internal/proc`

```go
package proc

type Runner interface {
    Run(ctx context.Context, name string, args ...string) ([]byte, error)
    Start(ctx context.Context, name string, args ...string) (*exec.Cmd, error)
}

type RealRunner struct{}     // production: 直接 os/exec
type FakeRunner struct{ ... } // test: 期待 args とレスポンスを事前登録
```

外部コマンド (`tmux` / `notify-send` / `gdbus` / `paplay` / `fzf` / `setsid` / `xdotool` / `swaymsg`) は **必ず Runner 経由**。テストは fake 注入で挙動を assertion 可能にする。

#### `internal/obslog`

```go
package obslog

func New(progname string) *slog.Logger
// Default handler: stderr に slog.JSONHandler
// ERROR level の場合は同時に `logger -t <progname>` にも転送
//   (F-5 LOW-1 で確立した syslog 経路を継承)
```

#### `internal/cockpit`

```go
package cockpit

type Status string
const (
    StatusWorking Status = "working"
    StatusWaiting Status = "waiting"
    StatusDone    Status = "done"
)

type PaneState struct {
    Session string
    Pane    int
    Status  Status
    UpdatedAt time.Time
}

func CachePath(session string, pane int) string  // ~/.cache/claude-cockpit/panes/<S>_<P>.status
func WriteStatus(session string, pane int, s Status) error  // atomic write
func LoadAll() ([]PaneState, error)
func Summary(states []PaneState) string  // "⚡3 ⏸1 ✓2 "
```

#### `internal/notify`

```go
package notify

type Popup struct {
    SessionID string  // claude session uuid (replace-id key)
    Title     string
    Body      string
    Actions   []Action
}

type Action struct {
    ID    string  // "default" / "close" など
    Label string
}

// Dispatch sends the popup via notify-send and waits for D-Bus action signals.
// Caller is responsible for setsid + log redirection (claude-notify-hook 側で行う).
func Dispatch(ctx context.Context, p Popup, r proc.Runner) error

// replace-id state I/O (sessions/<sid>.id)
func LoadReplaceID(sessionID string) (uint32, error)
func SaveReplaceID(sessionID string, notifID uint32) error
```

D-Bus listener は `godbus/dbus/v5` を依存に追加し、`org.freedesktop.Notifications` の `ActionInvoked` / `NotificationClosed` signal を 1 popup 分だけ受け、popup ID 一致でハンドル → exit する。

### 4.4 Layer 2 共通契約

- **hook 系** (`cockpit-state`, `notify-hook`, `notify-sound`):
  - **常に `os.Exit(0)`** を絶対契約とする (現 shell の `exit 0` を継承)
  - 内部エラーは `obslog.Error(...)` で syslog に流すのみ。stderr 経由の Claude hook 側にはエラーを伝播させない
  - `defer func() { recover(); os.Exit(0) }()` を main 冒頭に置いて panic も握りつぶす
- **その他** (`cleanup`, `prune`, `summary`, `switcher`, `next-ready`, `dispatch`):
  - 通常の exit code (0 = success, 1+ = failure)
  - tmux status-right から呼ばれる `summary` は失敗時に空文字列を出して exit 1 (status-right を壊さない)

### 4.5 Data flow (現状の state file レイアウトを完全維持)

| データ | パス | フォーマット | 互換性 |
|---|---|---|---|
| cockpit pane status | `~/.cache/claude-cockpit/panes/<session>_<pane>.status` | 1 行: `working` / `waiting` / `done` | shell 時代と同パス・同フォーマット (`tmux-agent-status` 互換) |
| notify replace-id state | `${XDG_RUNTIME_DIR}/claude-notify/sessions/<sid>.id` | 1 行: D-Bus notification id (uint32 文字列) | 同上 |

→ 移行過渡期に shell ↔ Go が並存しても state を共有できる。rollback 時も runtime 状態が連続。

## 5. Build & Deploy

### 5.1 Toolchain

- Go: `mise use -g go@latest` で導入 (`dot_config/mise/config.toml` の `[tools]` に `go = "latest"` を追加)
- 既存 `rustup` / `mise` ベースの bootstrap と同じパターン
- 新規マシンセットアップで `mise install` 1 発で揃う

### 5.2 Build トリガ

`.chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl` を新設:

```bash
#!/usr/bin/env bash
# claude-tools sha256: {{ include "../programs/claude-tools" | sha256sum }}  ※pseudo
set -euo pipefail

REPO_DIR="$(chezmoi source-path)/programs/claude-tools"
[[ -d "$REPO_DIR" ]] || { echo "claude-tools source missing"; exit 0; }

# Manjaro 以外 / Go 不在ならスキップ
command -v go >/dev/null 2>&1 || { echo "go toolchain missing"; exit 0; }

cd "$REPO_DIR"
go test ./...
go build -trimpath -ldflags="-s -w" -o "$HOME/.local/bin/" ./cmd/...
```

- ソース sha256 を template に埋め込んで変更検知 (chezmoi の `run_onchange` 慣行)
- **全 binary 成功時のみ置換** (中途半端な状態を作らない: `go build` は all-or-nothing で `~/.local/bin/` に書く)
- `go test ./...` を build 前に走らせて、テスト失敗時は新バイナリを出さない (rollback 安全)

### 5.3 Source 配置

- chezmoi リポジトリ内 `programs/claude-tools/` に直置き
- `.chezmoiignore` に `programs/` を追加して chezmoi の配布対象から除外
- → 「dotfiles と Go ソースが 1 commit で同期」「`chezmoi apply` 経路で binary が生える」を両立

### 5.4 旧 shell の撤去

各移行 PR で 1 binary 完成 → 同 PR 内で旧 shell を `git rm`:

- `dot_local/bin/executable_claude-*.sh` (5 本)
- `dot_config/tmux/scripts/cockpit/executable_*.sh` (4 本)

build script の出力先 `~/.local/bin/<binname>` が `dot_local/bin/executable_<binname>.sh` の chezmoi 配布先と同じパスになるよう、binary 名は **shell 拡張子を取った形** で揃える (例: `claude-cockpit-state.sh` → `claude-cockpit-state`)。

tmux の起動コマンド側 (`dot_config/tmux/conf/status.conf` 等) で `.sh` をパスに含めている箇所は同 PR で書き換える。

## 6. Testing

### 6.1 Unit test (`internal/*_test.go`)

| パッケージ | 主なテストケース |
|---|---|
| `atomicfile` | 通常書き込み / dir 不在エラー / 書き込み中断時に tmp が残らないか / 並行書き込み (rename atomicity) |
| `xdg` | XDG_RUNTIME_DIR set / unset / XDG_CACHE_HOME set / unset の各パス解決 |
| `cockpit` | Status round-trip (parse → string) / CachePath 計算 / LoadAll で破損ファイル混在時の skip / Summary 集計 (空集合 / 全種混在 / 重複) |
| `notify` | replace-id state I/O round-trip / 不正フォーマット時の 0 fallback / Popup struct → notify-send args の変換 |
| `obslog` | slog レベルフィルタ / ERROR 時に logger 呼び出しが走るか (proc.FakeRunner で観測) |
| `proc` | FakeRunner の登録 / 未登録コマンド呼び出しでテスト失敗 |

### 6.2 Mock 戦略

- 外部コマンド (`tmux`, `notify-send`, `gdbus`, `paplay`, `fzf`, `xdotool`, `swaymsg`) は全て `internal/proc.Runner` 経由
- テストは `proc.FakeRunner` を注入し、期待 argv とレスポンス (stdout / exit code) を事前登録
- D-Bus 連動 (`internal/notify` の listener) は `godbus/dbus/v5` の `Conn` を interface 化して fake bus を注入できるようにする (詳細実装時)

### 6.3 Smoke test

`docs/manage_claude.md` の "Cockpit State Tracking — Smoke Tests" 8 step を **各移行 PR ごとに該当部分のみ実機実行**:

- cockpit-state → step 1〜2 (hook 経由で `working` ファイルが生える)
- cockpit-summary → step 3 (status-right 表示)
- cockpit-prune → step 8 (kill-server 後の cache 回収)
- cockpit-switcher → step 5〜6 (`prefix + C → s`)
- cockpit-next-ready → step 7 (`prefix + C → N`)
- notify-* → `docs/manage_claude.md` §5.7 の手順に従って popup 起動 / クリック / 再生

### 6.4 CI

- なし。個人 dotfile につき GitHub Actions / Docker は YAGNI
- 開発時: `pre-commit` hook で `go test ./...` を `programs/claude-tools/` 配下で走らせる (chezmoi 全体の pre-commit と並列)

## 7. Logging

- `internal/obslog.New(progname)` 経由で `*slog.Logger` を取得
- 出力: stderr に `slog.JSONHandler` で JSON-line
- ERROR レベル: 同時に `logger -t <progname> "<msg> <key=val ...>"` を呼んで journal/syslog に転送 (`logger` 不在時は静かにスキップ)
- hook 系の **fatal でも exit 0**: log だけ吐いて静かに死ぬ。Claude hook の `exit 0` 絶対契約を破らない

journal で grep する例 (運用): `journalctl --user --since=today | grep claude-cockpit-state`

## 8. Migration Plan (Vertical-B-first)

PR 1 本につき shell 1 本 → Go 1 本の置換を原則とする。

| # | binary | tier | PR DoD |
|---|---|---|---|
| 1 | `cockpit-state` | T1 | + `internal/{xdg,atomicfile,obslog,proc}` 同時 commit。`programs/claude-tools/` 初期化。`run_onchange_after_build-claude-tools.sh.tmpl` 新設。hook 契約 (exit 0) を unit test で固定 |
| 2 | `cockpit-prune` | T1 | `internal/cockpit.LoadAll` 追加。tmux pane scan は `proc.Runner` 経由 |
| 3 | `cockpit-summary` | T2 | status-right 出力 byte-exact 一致を test で固定 |
| 4 | `cockpit-next-ready` | T2 | tmux switch-client 経路。ジャンプ順序 (session asc / window idx asc / pane idx asc) を test |
| 5 | `cockpit-switcher` | T3 | fzf に stdin pipe + Enter/Ctrl-X/Ctrl-R キーバインド。fzf 出力 parse は test 化 |
| **★** | — | — | **B 完走チェックポイント**: 8-step smoke 通し。Go 化継続の go/no-go 判定。撤退する場合は (6) 以降を中止、ここまでは shell + Go 共存で安定運用に切替 |
| 6 | `notify-cleanup` | T1 | mtime TTL を `time.Time` で素直に。`base_dir` suffix チェック (env 注入耐性) の test |
| 7 | `notify-sound` | T1 | trivial。`exec.Command("paplay", ...)` |
| 8 | `notify-hook` | T4 | env 受け渡し + `syscall.SysProcAttr.Setsid: true` で hook 親から分離。dispatcher / sound worker の起動順序 |
| 9 | `notify-dispatch` | T5 | `godbus/dbus/v5` で `org.freedesktop.Notifications` の `ActionInvoked` / `NotificationClosed` を listen。popup state machine。最後に最難所 |

### 8.1 各 PR の DoD

- [ ] `go test ./...` が pass
- [ ] 旧 shell を `git rm` する diff が同 PR 内
- [ ] `chezmoi diff` で `~/.local/bin/<binname>` が生える差分を確認
- [ ] `chezmoi apply` で実機反映
- [ ] 該当領域の smoke test 1〜2 ケースを実機で通す (結果を PR description に貼る)
- [ ] `code-reviewer` agent / `go-reviewer` agent によるレビュー (CRITICAL / HIGH 解消)

## 9. Rollout / Rollback

- **Clean cut per PR** (feature flag なし、YAGNI)
- 壊れた場合: `git revert <PR>` 1 発で shell 復帰
- cache / state file のパス・フォーマットは shell 時代と完全互換 → revert 後も runtime 状態が連続
- 過渡期に「同じ binary が shell + Go の両方で動く」状況は作らない (1 PR で 1 binary を atomic 切替)

## 10. Module / Naming

- Go module path: **`claude-tools`** (local module, github.com/... のような外部公開パスを名乗らない)
  - 個人 dotfile につき公開予定なし。GOPROXY 不要
  - 将来 (b) 別 repo + nix overlay に分離する場合のみ `github.com/kkiyama117/claude-tools` 等に rename
- binary 名: 旧 shell の basename から `.sh` を取る (`claude-cockpit-state.sh` → `claude-cockpit-state`)
- binary 出力先: `~/.local/bin/<binname>` (旧 shell の chezmoi 配布先と同じ)

## 11. Out of Scope (本 spec から派生する follow-up)

`docs/todos.md` に追加する 4 件:

1. **Rust 版実装の検討** (Go 完走後、学習比較用)
2. **C サブシステム (installer / `.chezmoiscripts/run_*.sh.tmpl`) の Go 化** (chezmoi template 結合の解消方法を別途設計)
3. **`notify-dispatch` daemon 化** (既存 F-3.next L33 を更新。Go 化完了後の改善ターゲット)
4. **F-4 nix 移行と build トリガ統合** (`run_onchange_after_build-claude-tools.sh.tmpl` を nix overlay 経由 build に振り替える)

## 12. Open Questions

なし (本 spec 時点で全決定済み)。実装中に発生したら `docs/todos.md` に追記する。

## 13. Acceptance Criteria

- [ ] `programs/claude-tools/` 配下に Go プロジェクトが配置され、`go test ./...` が pass する
- [ ] `chezmoi apply` で `~/.local/bin/claude-{cockpit,notify}-*` 9 binary が生成される
- [ ] 旧 shell 9 本が `git rm` 済みで repo から消えている
- [ ] `docs/manage_claude.md` の 8-step smoke が Go binary 環境で全 PASS
- [ ] notify popup の左クリック focus / 右クリック close / 同 session 連発の replace-id 集約が shell 時代と同等に動作
- [ ] `journalctl --user | grep claude-` で hook の動作ログ (情報レベル) と異常時の error ログ (logger -t 経由) の両方が観測できる
