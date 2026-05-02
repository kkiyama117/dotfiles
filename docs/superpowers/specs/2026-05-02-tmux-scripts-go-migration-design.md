# tmux Scripts → Go 移行 (C サブシステム前半) — Design Spec

- **Date**: 2026-05-02
- **Status**: Approved (brainstorming complete, ready for implementation plan)
- **Owner**: kiyama
- **Repo**: chezmoi (`/home/kiyama/.local/share/chezmoi`)
- **Related**:
  - [`2026-05-01-shell-to-go-migration-design.md`](./2026-05-01-shell-to-go-migration-design.md) (親 spec、本 spec で Non-Goal を部分解除)
  - [`./2026-05-02-notify-dispatch-daemon-design.md`](./2026-05-02-notify-dispatch-daemon-design.md) (D-4 完了済 reference)
  - `docs/superpowers/plans/2026-05-02-shell-to-go-migration-phase2.md` (A サブシステム plan、本 spec の前提)

---

## 1. Goal

`dot_config/tmux/scripts/` 配下の対話 UI / git worktree 操作系 shell **5 本 / ~280 行** を Go binary で 1:1 置換し、Phase 1/2 で確立した Go 基盤の上に「対話 UI / git worktree ops / tmux 状態操作」レイヤを集約する。

| 旧 shell | 行数 | 新 Go binary |
|---|---:|---|
| `dot_config/tmux/scripts/executable_claude-branch.sh` | 8 | `cmd/claude-branch` |
| `dot_config/tmux/scripts/executable_claude-respawn-pane.sh` | 17 | `cmd/claude-respawn-pane` |
| `dot_config/tmux/scripts/executable_claude-kill-session.sh` | 110 | `cmd/claude-kill-session` |
| `dot_config/tmux/scripts/executable_tmux-claude-new.sh` | 226 | `cmd/claude-tmux-new` |
| `dot_config/tmux/scripts/executable_claude-pick-branch.sh` | 60 | `cmd/claude-pick-branch` |

達成項目:

- **G1 porcelain parser のテスト化**: `git worktree list --porcelain` の多行 state machine を `internal/gitwt` 内で table-driven test で固める
- **G2 tmux argv の reproducibility**: 4 binary が叩く tmux 副作用を `internal/tmux` の `Client.<method>` 経由に集約し、FakeRunner で argv assertion を可能にする
- **G3 互換維持**: tmux window options (`@claude-managed` / `@claude-worktree` / `@claude-main-repo`)、cockpit cache パス (`~/.cache/claude-cockpit/panes/<S>_<P>.status`)、tmux session/window 命名 sanitize (`[^a-zA-Z0-9._-]` → `-`)、`printf %q` 互換 quoting を完全踏襲
- **G4 hook 契約継続**: `claude-branch` は status-right から無限呼び出しされるため、空入力 / git 不在 / branch 取得失敗 すべて exit 0 + 空文字列出力を維持

## 2. Non-Goals

- **`tpm-bootstrap.sh`** — installer-domain (machine 初回 + network I/O + plugin manager 契約) として D subsystem spec へ繰延 (§11 follow-up #1)
- **`.chezmoiscripts/run_*.sh.tmpl`** — 親 spec の Non-Goal 据え置き (chezmoi template 結合)
- **tmux key-binding (`bindings.conf`) の構造変更** — script path 書き換えのみ。binding 構造 (display-popup / confirm-before / run-shell ラッピング) は変更しない
- **`claude-branch` 出力フォーマット変更** — 現状 `[<branch>] ` (角括弧 + trailing space) を完全踏襲 (status-right の他要素との並びが既に組まれている)
- **エラー UX の刷新** — `tmux display-message` + log file (`/tmp/tmux-claude-new.log` 等) の現行運用を維持。slog/logger 経路は ERROR レベルのみ追加
- **新規 OS 対応** — Linux/amd64 のみ (親 spec §3.3 踏襲)

### 2.1 親 spec Non-Goal の解除根拠

親 spec (`2026-05-01-shell-to-go-migration-design.md` §2) は「**C サブシステム (installer / `.chezmoiscripts/run_*.sh.tmpl` / `tpm-bootstrap.sh`) — chezmoi template 結合が深い**」を Non-Goal として明記していた。本 spec はこの Non-Goal を **部分的に解除** する。

- **解除する範囲**: `dot_config/tmux/scripts/` 配下の **plain executable_*.sh 5 本** (`.tmpl` 拡張子を持たない静的 shell)
- **解除しない範囲**: `.chezmoiscripts/run_*.sh.tmpl` および `tpm-bootstrap.sh` (network I/O + run_once + chezmoi script template 結合)
- **判断軸**: 親 spec の Non-Goal 理由は **chezmoi template binding** だが、対象 5 本は template binding を持たない静的 shell。よって元の理由が当該範囲に **適用されない** ことが Non-Goal 解除の正当性となる

## 3. Scope

### 3.1 移行対象 (5 binary)

§1 の表の通り。全て `dot_config/tmux/scripts/` 配下、`.sh` 拡張子を取った同名 binary に置換 (例外: `tmux-claude-new` のみ `.sh` 取り後に **`claude-` prefix で揃える** ため `claude-tmux-new` に rename — `~/.local/bin/claude-*` で grep 一貫性を取る)。

### 3.2 移行対象外 (本 spec)

- `dot_config/tmux/scripts/executable_tpm-bootstrap.sh` (D subsystem へ)
- `dot_config/tmux/scripts/cockpit/*.sh` (Phase 1 で B subsystem として完了済み)

### 3.3 ターゲット

- OS: **Linux/amd64 のみ** (親 spec §3.3 踏襲)
- 実機: ローカル開発機 (`~kiyama@manjaro`)
- tmux: 3.6+ (`new-window -S` を使用)

## 4. Architecture

### 4.1 Project layout (本 spec で追加するもののみ)

```
programs/claude-tools/
├── internal/
│   ├── tmux/                          # 新規 (~150 行 + test ~100 行)
│   │   ├── tmux.go
│   │   └── tmux_test.go
│   └── gitwt/                         # 新規 (~120 行 + test ~80 行)
│       ├── gitwt.go
│       └── gitwt_test.go
└── cmd/                               # 新規 5 binary
    ├── claude-branch/main.go          # ~30 行
    ├── claude-tmux-new/main.go        # ~150 行
    ├── claude-pick-branch/main.go     # ~50 行
    ├── claude-kill-session/main.go    # ~100 行
    └── claude-respawn-pane/main.go    # ~30 行
```

既存 `internal/{xdg,atomicfile,proc,obslog}` (Phase 1) はそのまま再利用。`internal/cockpit` は cache パス helper のみ参照 (`claude-kill-session` の cache cleanup)。

### 4.2 `internal/tmux` API

```go
package tmux

type Client struct{ runner proc.Runner }

func New(r proc.Runner) *Client

// Display は status line に短文を出す (失敗無視)。
func (c *Client) Display(ctx context.Context, msg string)

// HasSession は tmux has-session -t "=<name>" の exit を bool で返す。
func (c *Client) HasSession(ctx context.Context, name string) bool

// SetWindowOption は tmux set-option -w -t <target> -o <key> <value>。
func (c *Client) SetWindowOption(ctx context.Context, target, key, value string) error

// ShowWindowOption は tmux show-options -w -t <target> -v <key>。未設定時は ("", nil)。
func (c *Client) ShowWindowOption(ctx context.Context, target, key string) (string, error)

// ListPanes は tmux list-panes -t <target> -F <format>。改行区切り。
func (c *Client) ListPanes(ctx context.Context, target, format string) ([]string, error)

// DisplayMessageGet は tmux display-message -p [-t <target>] <format>。
func (c *Client) DisplayMessageGet(ctx context.Context, target, format string) (string, error)

// NewSessionDetached は tmux new-session -d -s <name> -n <window> -c <cwd>。
func (c *Client) NewSessionDetached(ctx context.Context, session, window, cwd string) error

// NewWindowSelectExisting は tmux new-window -S -t <session>: -n <window> -c <cwd>。
func (c *Client) NewWindowSelectExisting(ctx context.Context, session, window, cwd string) error

// SplitWindowH は tmux split-window -h -t <target> -c <cwd>。
func (c *Client) SplitWindowH(ctx context.Context, target, cwd string) error

// SelectPaneTitle は tmux select-pane -t <target> -T <title> (失敗無視)。
func (c *Client) SelectPaneTitle(ctx context.Context, target, title string)

// SendKeys は tmux send-keys -t <target> <keys...> (Enter は呼び出し側で必要なら "Enter" 付与)。
func (c *Client) SendKeys(ctx context.Context, target string, keys ...string) error

// SwitchClient は tmux switch-client -t <target>。
func (c *Client) SwitchClient(ctx context.Context, target string) error

// AttachSessionExec は exec.Command("tmux", "attach-session", "-t", target) を syscall.Exec で
// 現プロセス置換 (TTY 維持)。switcher と同じパターン。
func (c *Client) AttachSessionExec(target string) error

// KillWindow は tmux kill-window -t <target>。
func (c *Client) KillWindow(ctx context.Context, target string) error

// RespawnPaneKill は tmux respawn-pane -k -t <target>。
func (c *Client) RespawnPaneKill(ctx context.Context, target string) error

// Sanitize は tmux session/window 名で許される文字以外を '-' に置換する。
// 受理: [a-zA-Z0-9._-]。それ以外は '-'。
func Sanitize(s string) string

// ShellQuote は POSIX シェル単引用符でエスケープした文字列を返す。
// 空文字列は '' を返し、内部の ' は '\'' に展開。
// bash `printf %q` とはバイト列が一致しないが、bash に食わせて再パースした
// 結果として **同じ引数値** を生成する (semantic equivalence)。
func ShellQuote(s string) string
```

設計判断:
- `Sanitize` / `ShellQuote` は package-level helper (Client に紐付かない、cmd 全体で共有)
- 失敗無視 (display-message / select-pane title) は戻り値なし。失敗を伝播すべき (kill-window / switch-client / set-window-option) は error 返却
- すべて `proc.Runner` 経由 → FakeRunner でテスト可能
- `AttachSessionExec` は `syscall.Exec` 直叩き (proc.Runner 経由不可)。switcher の ctrl-r reload と同じ理由

### 4.3 `internal/gitwt` API

```go
package gitwt

type Worktree struct {
    Path   string
    Branch string  // "refs/heads/<x>" の <x> 部分。detached は ""
    HEAD   string  // commit hash (porcelain が返した形のまま)
}

type Client struct{ runner proc.Runner }

func New(r proc.Runner) *Client

// MainRepo は worktree list の先頭エントリ path を返す。git 外なら ("", err)。
func (c *Client) MainRepo(ctx context.Context, cwd string) (string, error)

// ListPorcelain は cwd の git worktree list --porcelain を parse して返す。
func (c *Client) ListPorcelain(ctx context.Context, cwd string) ([]Worktree, error)

// FindByBranch は ListPorcelain から branch 一致のものを 1 件返す。
// なければ (Worktree{}, false, nil)。
func (c *Client) FindByBranch(ctx context.Context, cwd, branch string) (Worktree, bool, error)

// LocalBranches は git for-each-ref --format='%(refname:short)' refs/heads。
func (c *Client) LocalBranches(ctx context.Context, cwd string) ([]string, error)

// HasLocalRef は git show-ref --verify --quiet refs/heads/<branch> の exit を bool で返す。
func (c *Client) HasLocalRef(ctx context.Context, cwd, branch string) bool

// HasRemoteRef は origin/<branch> 用 (refs/remotes/origin/<branch>)。
func (c *Client) HasRemoteRef(ctx context.Context, cwd, branch string) bool

// Add は次の 3 形態を呼び分ける wrapper:
//   AddExistingLocal:  git -C <cwd> worktree add <path> <branch>
//   AddTrackingRemote: git -C <cwd> worktree add -b <branch> <path> origin/<branch>
//   AddFromHead:       git -C <cwd> worktree add -b <branch> <path> HEAD
func (c *Client) AddExistingLocal(ctx context.Context, cwd, path, branch string) error
func (c *Client) AddTrackingRemote(ctx context.Context, cwd, path, branch string) error
func (c *Client) AddFromHead(ctx context.Context, cwd, path, branch string) error

// Remove は git -C <main> worktree remove <target> --force。stderr capture。
func (c *Client) Remove(ctx context.Context, mainRepo, target string) (stderr string, err error)

// Prune は git -C <main> worktree prune (失敗無視)。
func (c *Client) Prune(ctx context.Context, mainRepo string)

// CurrentBranch は git -C <cwd> branch --show-current。空文字列 OK。
func (c *Client) CurrentBranch(ctx context.Context, cwd string) (string, error)

// TopLevel は git -C <cwd> rev-parse --show-toplevel。
func (c *Client) TopLevel(ctx context.Context, cwd string) (string, error)
```

#### `ListPorcelain` parser 仕様 (本 spec の核)

入力例:
```
worktree /home/kiyama/.local/share/chezmoi
HEAD abc1234567890
branch refs/heads/develop

worktree /home/kiyama/.local/share/worktrees/chezmoi/feat-c-subsystem-design
HEAD def4567890123
branch refs/heads/feat/c-subsystem-design

worktree /tmp/detached-checkout
HEAD 9876543210abc
detached
```

state machine:
- `worktree <path>` 行 → 新エントリ開始 (進行中があれば確定して append)
- `HEAD <hash>` 行 → 現エントリの `HEAD`
- `branch refs/heads/<x>` 行 → 現エントリの `Branch` (`refs/heads/` prefix 除去)
- `detached` 行 → no-op (Branch は "" のまま)
- 空行 → エントリ確定 (次の `worktree` 行で再開)
- EOF → 進行中エントリがあれば確定

table-driven test の case (§6.1 参照): 単一 main / main + 1 worktree / main + N worktrees / detached HEAD / branch ref が一致しない / 空入力 / 末尾空行なし / `\n` のみの空行混在 / `branch` 行が `refs/heads/` 以外 (tag 等)。

### 4.4 5 cmd 設計概要

| binary | 主要 API 利用 | 特殊事項 |
|---|---|---|
| `claude-branch` | `gitwt.CurrentBranch` のみ | argv[1] を cwd に。`[<branch>] ` を出力。失敗時 exit 0 + 何も出さない (status-right 安全) |
| `claude-tmux-new` | tmux 全部 + gitwt 全部 + `os/exec` (fzf interactive、`syscall.Exec` for attach) | argparse: 位置 `<branch>` + flag (`--from-root [<id>]` / `--no-claude` / `--worktree-base <dir>` / `--prompt <text>`)。`tmux.ShellQuote` で initial prompt を send-keys に流す |
| `claude-pick-branch` | `os/exec` (fzf interactive) + `syscall.Exec` で `claude-tmux-new` 切替 | branch 一覧は `gitwt.LocalBranches`。fzf 不在で exit 1 + display-message |
| `claude-kill-session` | tmux options/list-panes/kill-window + gitwt.{ListPorcelain,Remove,Prune,TopLevel} + cockpit cache rm | argv[1] に explicit target 受け取り。安全チェック 3 段 (managed=yes / pane_current_command=claude / legacy `claude-*` session prefix) |
| `claude-respawn-pane` | tmux list-panes + respawn-pane + send-keys | 現 session の claude pane を grep、なければ current pane |

### 4.5 Data flow / 互換維持

| データ | 互換維持の対象 | 維持手段 |
|---|---|---|
| tmux window options | `@claude-managed` / `@claude-worktree` / `@claude-main-repo` | `tmux.SetWindowOption` で同 key を書く |
| cockpit cache パス | `${XDG_CACHE_HOME:-~/.cache}/claude-cockpit/panes/<session>_<pane_id>.status` | `xdg.ClaudeCockpitCacheDir()` を再利用、`claude-kill-session` の cleanup ループで `os.Remove` |
| tmux session/window 名 sanitize | `[^a-zA-Z0-9._-]` → `-` | `tmux.Sanitize` 1 か所 |
| `printf %q` 互換 quoting | initial prompt の send-keys | `tmux.ShellQuote` (POSIX 単引用符 + `'\''` エスケープ) |
| log file | `/tmp/tmux-claude-new.log`, `/tmp/claude-pick-branch.log` | 当面維持 (debug 容易性、tmux popup の stdout が見えない事情)。slog の JSONHandler とは別物として共存 |
| binary 名 | shell の basename から `.sh` を取る | 例外: `tmux-claude-new` → `claude-tmux-new` (prefix を `claude-` で揃える) |

### 4.6 hook / exit code 契約 (親 spec §4.4 踏襲)

- `claude-branch` のみ **常に exit 0** (status-right 経由で無限呼び出しされるため)。`defer recover() + os.Exit(0)` で panic も握りつぶす
- 残り 4 本は通常 exit code (0 / 1+)
- `tmux display-message` での error 通知は維持 (key-binding 経由で stderr が見えないため)

## 5. Build & Deploy

Phase 1 で確立した `.chezmoiscripts/run_onchange_after_build-claude-tools.sh.tmpl` がそのまま動く。`programs/claude-tools/` 配下のソース hash が変わると `chezmoi apply` で `go test ./... && go build ./cmd/...` が走り、5 binary が `~/.local/bin/` に追加される。

## 6. Testing

### 6.1 Unit test

| package | 主なテスト |
|---|---|
| `internal/tmux` | 各 method が `proc.Runner` に渡す argv が想定通り (`"tmux"`, `"set-option"`, `"-w"`, `"-t"`, target, `"-o"`, key, value 等)。`Sanitize` は table-driven (alnum 通過 / `/` → `-` / 連続記号 / 空文字列 / unicode コードポイント)。`ShellQuote` は table-driven (空 / 空白あり / quote / 改行 / 日本語 / `\` / `$`、bash の `printf %q` 出力との一致を fixture で 1 case 確認) |
| `internal/gitwt` | `parsePorcelain` を §4.3 列挙 9 case で固定。`CurrentBranch` の空出力時の挙動。`ListPorcelain` は FakeRunner で stdout 注入。`Add{ExistingLocal,TrackingRemote,FromHead}` の argv 期待値検証 |
| `cmd/claude-branch` | 引数なし → exit 0 出力空 / git 不在 → exit 0 出力空 / 通常 → `[<branch>] ` |
| `cmd/claude-respawn-pane` | claude pane 存在 → 該当 pane に respawn / 不在 → current pane に respawn / send-keys が `claude --continue` Enter で発射 |
| `cmd/claude-kill-session` | 安全チェック 3 段の matrix (managed=yes / pane_current_command=claude / legacy `claude-*` / 全部 NG → exit 1) / worktree が main_repo と同じ → 削除しない / pinned tag がなく pane_current_path fallback / cache cleanup 後に `<S>_<P>.status` が消えている |
| `cmd/claude-tmux-new` | argparse (位置 + 4 flag + mutual exclusive `--from-root` × `--no-claude` / `--prompt` × `--no-claude`)。既存 worktree 検出 / `--worktree-base` 指定 / fresh worktree (local ref / remote ref / 新規) の 3 分岐。fresh window では split + select-pane + send-keys 順序。initial prompt は ShellQuote 済 |
| `cmd/claude-pick-branch` | fzf 不在で exit 1 + display-message / 候補 0 で display-message + exit 0 / pick → argv 組み立て関数 (`buildExecArgs(branch, passthrough []string) []string`) を分離し、test ではこの関数の戻り値を直接 assertion (実 `syscall.Exec` は wrapper 内に閉じ込め test では呼ばない) |

`go test -race ./...` が pass、coverage 80% 目標 (`go test -cover ./...`)。

### 6.2 Smoke test

`docs/superpowers/smoke/2026-05-02-go-tmux-scripts-smoke.md` を新設して結果を記録。

| 領域 | 手順 |
|---|---|
| C-1 | tmux pane の status-right に `[<branch>] ` が表示。git 外 cwd で空。worktree 切替で追従 |
| C-2 | 2-pane window (claude pane あり) で `prefix + C → r` → claude pane が新規 process で再起動。1-pane window で同操作 → current pane が再起動 |
| C-3 | `prefix + C → x` (confirm-before) → window kill + `git worktree list` から消失 + `~/.cache/claude-cockpit/panes/<S>_<P>.status` が消失。安全チェック失敗ケース (managed タグなし、claude プロセスなし、非 legacy session) で `display-message` だけ出て kill されない |
| C-4 | `claude-tmux-new <new-branch>` で worktree + window + 2-pane + `claude` 起動。`--no-claude` で 1-pane shell only。`--from-root` (id 省略) で fzf による root session 選択 → `--resume <id> --fork-session` 起動。`--prompt "テスト 日本語 'quote'"` で claude に safe quoting で渡る |
| C-5 | `prefix + C → n` (popup) → fzf branch 選択 → C-4 と同じフロー |
| 中間 | C-3 完走時に C-1〜C-3 を 1 周まとめて再走 |

### 6.3 CI

なし (親 spec §6.4 踏襲)。`pre-commit` で `go test ./...` 実行を継続。

## 7. Logging

`internal/obslog.New(progname)` 経由で stderr JSON-line。互換のため `/tmp/tmux-claude-new.log` / `/tmp/claude-pick-branch.log` の **既存 plain text log は併走** (debug 容易性、tmux popup の stdout が見えない事情)。slog の構造化情報は別経路。

journal で grep する例 (運用): `journalctl --user --since=today | grep claude-tmux-new`

## 8. Migration Plan (5 PR、tier 順)

PR 1 本につき shell 1 本 → Go 1 本 atomic swap (親 spec §8 踏襲)。`internal/tmux` / `internal/gitwt` は **初出 PR にバンドル** (Phase 1 が `internal/{xdg,atomicfile,obslog,proc}` を PR-1 cockpit-state と同 commit したのと同パターン)。

| # | binary | tier | bundled internal additions |
|---|---|---|---|
| C-1 | `claude-branch` | T1 | `internal/gitwt` 初期化 (`New`, `CurrentBranch`) |
| C-2 | `claude-respawn-pane` | T1 | `internal/tmux` 初期化 (`Display`, `ListPanes`, `DisplayMessageGet`, `RespawnPaneKill`, `SendKeys`) |
| C-3 | `claude-kill-session` | T3 | `tmux.{KillWindow, ShowWindowOption}`、`gitwt.{ListPorcelain, Remove, Prune, TopLevel}` |
| **★** | — | — | **C 中間チェックポイント**: C-1〜C-3 通し smoke。残作業の go/no-go 判定 |
| C-4 | `claude-tmux-new` | T5 | `tmux.{NewSessionDetached, NewWindowSelectExisting, SplitWindowH, SetWindowOption, SwitchClient, AttachSessionExec, SelectPaneTitle, ShellQuote, Sanitize, HasSession}`、`gitwt.{MainRepo, FindByBranch, HasLocalRef, HasRemoteRef, AddExistingLocal, AddTrackingRemote, AddFromHead}` |
| C-5 | `claude-pick-branch` | T2 | `gitwt.LocalBranches` 追加 |

### 8.1 各 PR の DoD

- [ ] `cd programs/claude-tools && go test -race ./...` pass
- [ ] 旧 shell を `git rm` する diff が同 PR 内
- [ ] tmux conf 側 (`bindings.conf` / `status.conf` / `tmux.conf`) のパス書き換えが同 PR 内 (該当する PR のみ)
- [ ] `chezmoi diff` で `~/.local/bin/<binname>` が増える差分を確認
- [ ] `chezmoi apply` で実機反映
- [ ] §6.2 該当領域の smoke を実機で通し、結果を PR description に貼る
- [ ] `code-reviewer` / `go-reviewer` agent レビュー (CRITICAL / HIGH 解消)

### 8.2 旧 shell + tmux conf 書き換え対応表

| PR | git rm | conf 書き換え |
|---|---|---|
| C-1 | `dot_config/tmux/scripts/executable_claude-branch.sh` | `dot_config/tmux/conf/status.conf` L11: `~/.config/tmux/scripts/claude-branch.sh '#{pane_current_path}'` → `~/.local/bin/claude-branch '#{pane_current_path}'` |
| C-2 | `executable_claude-respawn-pane.sh` | `bindings.conf` L71: `~/.config/tmux/scripts/claude-respawn-pane.sh` → `~/.local/bin/claude-respawn-pane` |
| C-3 | `executable_claude-kill-session.sh` | `bindings.conf` L86: 同上パターン |
| C-4 | `executable_tmux-claude-new.sh` | (binding 内では直接呼ばれない、pick-branch 経由) |
| C-5 | `executable_claude-pick-branch.sh` | `bindings.conf` L61, L66: 同上パターン。pick-branch から `claude-tmux-new` 起動経路も `~/.local/bin/claude-tmux-new` に固定 (PR-4 で binary は既存) |

## 9. Rollout / Rollback

- **Clean cut per PR** (feature flag なし、YAGNI)
- 壊れた場合: `git revert <PR>` 1 発で shell 復帰
- tmux window options / cockpit cache パスは shell 時代と完全互換 → revert 後も runtime 状態が連続
- 過渡期に「同じ binary が shell + Go の両方で動く」状況は作らない (1 PR で 1 binary atomic 切替)

## 10. Risks / Mitigations

| リスク | 影響 | 緩和 |
|---|---|---|
| `git worktree list --porcelain` parser のエッジケース漏れ (detached / 古い git) | kill-session が worktree を見つけ損ねて消し残し / 誤削除 | table-driven test 9 case + smoke で実機 (手元 git) パース確認。Remove は **path が main_repo と等しい場合スキップ** という既存 invariant を保つ |
| `printf %q` 相当の shell quoting 互換崩れ | initial prompt 内の特殊文字で send-keys が壊れる | `internal/tmux.ShellQuote` の table-driven test に **`bash -c 'printf "%s\n" <quoted>'` で再パースした結果が原文字列と一致** することを確認する round-trip case を 1 件入れる (semantic equivalence; CI なしなので手元で 1 度確認し fixture 化) |
| `tmux attach-session` を `syscall.Exec` で呼ぶ際の TTY 引き継ぎ | `prefix +`-binding pipe 経由だと "not a terminal" になる | 親 shell も `if [ -n "$TMUX" ]; then switch-client else attach-session` 分岐を持つ。Go でも `os.Getenv("TMUX") != ""` で switch-client 経路、外なら syscall.Exec attach。binding 経由の起動は常に `TMUX` set なので switch-client 経路に入る |
| C-3 で削除対象 worktree が pane_current_path 経由の fallback 解決のみで取れていた場合の誤削除 | 関係ない repo を消す | `wt_root != main_repo` invariant を Go でも厳守。table-driven test の 1 case として「fallback 解決の wt_root が main_repo と等しい」場合に **何もしない** ことを検証 |
| C-4 で fresh window 判定 (`pane_count <= 1`) が race で誤判定 | 既存 window に余計な split を作る | tmux 側が直列実行を保証 (key-binding pipe は逐次)。リスクは低いが test では `ListPanes` を FakeRunner で 1 行 / 2 行 注入して両分岐を固定 |
| log file `/tmp/...` への書き込みが multi-user で衝突 | 1 人運用なので実質ゼロ | 維持。`O_APPEND` open で既存挙動踏襲 |
| binding.conf 書き換え漏れ | tmux popup が old `.sh` を呼んで not found | 各 PR commit 内で `rg "tmux/scripts/<binname>\.sh" dot_config/tmux/` で 0 件確認 |

## 11. Out of Scope (本 spec 派生 follow-up)

`docs/todos.md` の G セクションに追加:

1. **D subsystem spec — `tpm-bootstrap` + `.chezmoiscripts/run_*.sh.tmpl` の Go 化** (本 spec から繰延、親 spec §11 #2 を継承)
2. **`internal/tmux.ShellQuote` を switcher / その他 send-keys 経路に再利用** (現 switcher は send-keys を使っていないが、将来の拡張で再利用余地あり)
3. **log file の `/tmp/*.log` から `~/.cache/claude-tools/<bin>.log` への移動** (multi-machine / non-root tmpfs 整理。低優先)

## 12. Open Questions

なし。実装中に発生したら `docs/todos.md` に追記する。

## 13. Acceptance Criteria

- [ ] `programs/claude-tools/internal/{tmux,gitwt}/` が `go test -race ./...` pass
- [ ] `programs/claude-tools/cmd/claude-{branch,tmux-new,pick-branch,kill-session,respawn-pane}/` 5 binary が build
- [ ] `chezmoi apply` で `~/.local/bin/claude-{branch,tmux-new,pick-branch,kill-session,respawn-pane}` が生える
- [ ] 旧 shell 5 本が `git ls-files | rg 'tmux/scripts/(executable_(claude-(branch|kill-session|pick-branch|respawn-pane))|executable_tmux-claude-new)\.sh'` で 0 件
- [ ] `dot_config/tmux/conf/{bindings,status}.conf` および `dot_config/tmux/tmux.conf` 内に `.config/tmux/scripts/.*\.sh` への参照が **`tpm-bootstrap.sh` のみ**
- [ ] §6.2 smoke 全 5 領域 PASS、`docs/superpowers/smoke/2026-05-02-go-tmux-scripts-smoke.md` に記録
- [ ] `journalctl --user | grep claude-` で 5 binary の動作ログ (情報レベル) と異常時 error の両方が観測可
