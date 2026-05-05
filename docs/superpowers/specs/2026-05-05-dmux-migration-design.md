# dmux Migration (Worktree Layout Alignment) — Design Spec

- **Date**: 2026-05-05
- **Status**: Code complete (2026-05-05) — manual smoke 1-5 awaiting user execution per `docs/superpowers/smoke/2026-05-05-dmux-migration-smoke.md`
- **Owner**: kiyama
- **Repo**: chezmoi (`/home/kiyama/.local/share/chezmoi`)
- **Related**:
  - [`2026-05-02-tmux-scripts-go-migration-design.md`](./2026-05-02-tmux-scripts-go-migration-design.md) — 親 spec (Phase C で `claude-tmux-new` 等 5 binary を Go 化済み)
  - [`docs/todos.md`](../../todos.md) — F-7 (`/branch-out` 実装) / G-2 PR-C-* (Phase C 完了) / 「デファード」B-3 (dmux lifecycle hook + cockpit 融合, 派生として登録済み)
- **Upstream reference**: [`standardagents/dmux`](https://github.com/standardagents/dmux) (`src/utils/paneNaming.ts` / `src/utils/slug.ts` / `context/HOOKS.md` / `context/API.md`)

---

## 1. Goal

自作の worktree spawner stack (Phase C で Go 化済み: `claude-tmux-new` / `claude-kill-session` / `claude-branch-merge` / `claude-pick-branch`) と slash command 3 本 (`/branch-out` / `/branch-finish` / `/branch-merge`) を **dmux のディレクトリ規約に整合させる最小改修**。dmux TUI 起動の有無に依存しない直接モードを正準動作とし、後日 dmux UI を併用可能な状態を作る。

達成項目:

- **G1 worktree base 移行**: `~/.local/share/worktrees/<repo>/<branch>/` → `<repo>/.dmux/worktrees/<flat-slug>/` (project-relative)
- **G2 slug アルゴリズム互換**: dmux の `sanitizeWorktreeSlugFromBranch()` を `internal/gitwt.SanitizeSlug` として bit-exact 移植
- **G3 cockpit 温存 (B-2)**: `internal/cockpit` / `claude-cockpit-*` / `claude-notify*` / `claude-notifyd` / `claude-respawn-pane` / `claude-branch` は無変更
- **G4 既存テスト互換**: 24+ パッケージ全 PASS、`go test -race ./...` GREEN を維持
- **G5 dmux 配信**: mise の global npm tool として `.chezmoiscripts/run_once_*` 経由で `dmux` を導入。既存 chezmoi 配信フローに統合

## 2. Non-Goals

- **dmux REST API 統合** — `POST /api/panes` / SSE / pane snapshot 等の HTTP クライアント実装 (アプローチ 2 の保留分。dmux TUI 常駐運用が定着してから再検討)
- **`.dmux/hooks/` を介した cockpit 連動** — `worktree_created` / `pre_merge` / `post_merge` から cockpit cache へブリッジする実装 (B-3 派生として `docs/todos.md` 「デファード」に登録済み)
- **dmux ↔ cockpit/notify 双方向同期** — dmux pane status (`agentStatus`) と cockpit cache (`working`/`waiting`/`done`) の相互変換は本 spec の対象外
- **既存 in-flight worktree の自動 migration** — `git worktree move` を含むスクリプト化はせず、§5 の手動 SOP のみ提供 (現状 main worktree 1 本のみ運用、in-flight 0 本)
- **OpenRouter 連携** — dmux の slug 生成は OpenRouter または `claude --no-interactive` 経由だが、本 spec では `/branch-out` の slash command 内で claude (呼び出し元) が自分で命名する既存方式を維持。OpenRouter 依存は導入しない
- **dmux UI のキーバインド統合** — tmux `prefix + C` series のキーバインド (`n` / `o` / `r` / `k`) は無変更。dmux TUI のキーバインド (`n` / `m` / `x` / etc.) との衝突回避は別途検討
- **agent 拡張** — dmux API 上 `agent` は `"claude" | "opencode"` のみ。`opencode` 等の他 agent 対応は本 spec の対象外
- **macOS / WSL 個別対応** — Linux/amd64 (Manjaro) のみ。dmux 自体は cross-platform だが、本 spec の検証対象は親 spec §3.3 踏襲

## 3. Scope

### 3.1 改修対象

| 領域 | 変更 |
|---|---|
| `programs/claude-tools/internal/gitwt/gitwt.go` | `SanitizeSlug` / `DmuxWorktreeRoot` / `EnsureGitignoreEntry` の 3 関数追加 |
| `programs/claude-tools/cmd/claude-tmux-new/main.go` | worktree base を `gitwt.DmuxWorktreeRoot` ベースに切替、dir 名を `gitwt.SanitizeSlug` で算出、`.gitignore` に `.dmux/` を冪等追記、`--worktree-base` フラグを deprecated 化 |
| `programs/claude-tools/cmd/claude-kill-session/main.go` | 削除対象 path が `<main-repo>/.dmux/worktrees/` 配下にあるか軽い sanity check (warn のみ、block しない) |
| `programs/claude-tools/cmd/claude-branch-merge/main.go` | 既存 `gitwt.FindByBranch` 経由のターゲット解決は無変更で動作。テスト期待値の path 文字列のみ更新 |
| `programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-out.md` | `claude-tmux-new` 呼び出しから `--worktree-base` 引数削除、補足セクションの worktree path 例を新規約に更新 |
| `programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-finish.md` | 補足セクションの worktree path 例のみ更新 (挙動不変) |
| `programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/branch-merge.md` | 補足セクションの worktree path 例のみ更新 (挙動不変) |
| `.chezmoiscripts/run_once_*.sh.tmpl` (Manjaro 分岐) | `mise use -g npm:dmux@latest` を 1 行追加 |
| `programs/claude-tools/README.md` | `claude-tmux-new` の note を新規約に書き換え |
| `docs/manage_claude.md` §5.x / `docs/keybinds.md` §I-x | worktree path 例の更新 (実装後の follow-up) |

### 3.2 無変更 (B-2: cockpit 温存)

- `programs/claude-tools/internal/{cockpit,notify,notifyd,obslog,proc,xdg,atomicfile,tmux}/` — 全パッケージ無変更
- `programs/claude-tools/cmd/claude-{cockpit-state,cockpit-prune,cockpit-summary,cockpit-next-ready,cockpit-switcher}/` — cockpit 系 5 binary 無変更
- `programs/claude-tools/cmd/claude-{notify-hook,notify-sound,notify-dispatch,notify-cleanup,notifyd}/` — notify 系 5 binary 無変更
- `programs/claude-tools/cmd/claude-{respawn-pane,branch,pick-branch}/` — UI helpers 無変更
- `dot_config/tmux/conf/bindings.conf` — tmux キーバインドの script path 参照は無変更 (binary path 不変)

## 4. Worktree Layout

### 4.1 新規約

- **Base path**: `<main-repo-toplevel>/.dmux/worktrees/`
  - `<main-repo-toplevel>` は `git rev-parse --show-toplevel` を main worktree (= 元リポジトリの check-out 場所) で叩いた結果
  - 例: chezmoi リポジトリで `/branch-out "test"` → `<HOME>/.local/share/chezmoi/.dmux/worktrees/feat-test/`
- **Dir name**: `<flat-slug>` = `gitwt.SanitizeSlug(branchName)` (§4.2 参照)
- **Branch name**: 既存維持 `<type>/<kebab-summary>` (slash 含む)
  - dmux は `branchPrefix` 設定で同等の prefix を表現するため、ブランチ名は完全互換
  - dmux は worktree dir 名のみ slash を hyphen に flatten する仕様
- **`.gitignore`**: 各リポジトリのトップレベル `.gitignore` に `.dmux/` を 1 行冪等追記 (§4.3 参照)

### 4.2 SanitizeSlug アルゴリズム (Go 移植)

dmux 上流 `src/utils/paneNaming.ts:33-44` を Go に bit-exact 移植する。

```go
// SanitizeSlug converts a branch name into a worktree directory slug,
// matching dmux's sanitizeWorktreeSlugFromBranch() exactly.
//
// Steps:
//   1. trim whitespace
//   2. lowercase
//   3. replace runs of `\` and `/` with single `-`
//   4. replace runs of `[^a-z0-9._-]` with single `-`
//   5. collapse runs of `-` to single `-`
//   6. strip leading and trailing `-`
//   7. strip leading and trailing `.`
//   8. fall back to "pane" if the result is empty
func SanitizeSlug(branchName string) string { /* ... */ }
```

期待される変換例 (table test 7 ケース):

| input | output | 備考 |
|---|---|---|
| `feat/dmux-migration` | `feat-dmux-migration` | 通常の type/kebab |
| `Feat/Dmux Migration` | `feat-dmux-migration` | uppercase + space (space は `[^a-z0-9._-]` に含まれるので `-` に) |
| `chore/v1.2.3` | `chore-v1.2.3` | dot 保持 (chars クラスに含まれる) |
| `feat//double--dash` | `feat-double-dash` | 連続スラッシュ + 連続ハイフンの圧縮 |
| `-feat/leading-dash-` | `feat-leading-dash` | 両端ダッシュの除去 |
| `..weird` | `weird` | leading dot の除去 |
| `///` | `pane` | 残骸が空のときのフォールバック |

### 4.3 `.gitignore` 自動追記

`/branch-out` 実行時、`claude-tmux-new` が main repo の `.gitignore` をチェックし、`.dmux/` 行が存在しなければ末尾に追記する。

```go
// EnsureGitignoreEntry idempotently appends `line` to <repoRoot>/.gitignore.
// Returns (changed=true) if the file was modified, (changed=false) if the line
// already existed (exact match on a single line, leading/trailing whitespace
// ignored). On any I/O error, returns (changed=false, err).
//
// File is opened in append mode with O_CREATE; if the file does not exist, it
// is created. Existing files have a trailing newline ensured before appending.
func EnsureGitignoreEntry(repoRoot, line string) (changed bool, err error) { /* ... */ }
```

挙動契約:

- **冪等**: 既存 `.dmux/` 行があれば noop
- **副作用最小**: write 失敗時は warn のみ (`obslog.Warn`)、`/branch-out` の主処理は継続
- **コミットしない**: `.gitignore` 変更後の `git add` / `git commit` は行わない (ユーザに委ねる)
- **stderr に 1 行 info**: `changed=true` のとき `claude-tmux-new: appended ".dmux/" to .gitignore` を出す

## 5. 既存 worktree のマイグレーション

### 5.1 方針

- **ハードカットオーバー**: 新バイナリは `~/.local/share/worktrees/<repo>/<branch>/` を一切認識しない
- **手動 migrate を SOP として文書化**: in-flight worktree がある場合のみユーザが手動実行
- **現状認識**: 設計時点 (`2026-05-05`) では main worktree (`/home/kiyama/.local/share/chezmoi/`) 1 本のみ運用、in-flight 0 本 (`gitStatus: clean`, `branch: main`) → migrate 不要

### 5.2 手動 migrate SOP

in-flight worktree がある場合の手順:

```bash
# 1. 各 in-flight worktree で uncommitted を退避
cd ~/.local/share/worktrees/<repo>/<branch>
git status                          # uncommitted があれば確認
git stash push -u -m "pre-dmux-migrate" || git commit -am "wip: pre-dmux-migrate"

# 2. main repo に戻って worktree を物理移動
cd <main-repo-toplevel>
mkdir -p .dmux/worktrees
git worktree move ~/.local/share/worktrees/<repo>/<branch> .dmux/worktrees/<flat-slug>

# 3. tmux pane の @claude-worktree オプションを再設定
#    (該当 pane で実行する。<pane-id> は `tmux display-message -p '#D'` で取得)
tmux set-option -p -t <pane-id> @claude-worktree "$(realpath .dmux/worktrees/<flat-slug>)"

# 4. 動作確認
/branch-merge main fetch            # target に統合できるか
# stash した場合は新 path で git stash pop
```

### 5.3 旧 path クリーンアップ

migrate 後、`~/.local/share/worktrees/<repo>/` 配下に空 dir が残る可能性がある。`git worktree prune` を main repo で実行し、必要なら `rm -rf ~/.local/share/worktrees/<repo>/<branch>` (空 dir のみ。worktree 状態が残っていれば prune が片付ける)。

## 6. Component Changes (詳細)

### 6.1 `internal/gitwt/gitwt.go` 追加 API

新規エクスポート:

```go
// SanitizeSlug — see §4.2
func SanitizeSlug(branchName string) string

// DmuxWorktreeRoot returns <repoRoot>/.dmux/worktrees as a clean absolute path.
// Caller is responsible for ensuring repoRoot is the main worktree's toplevel.
func DmuxWorktreeRoot(repoRoot string) string

// EnsureGitignoreEntry — see §4.3
func EnsureGitignoreEntry(repoRoot, line string) (changed bool, err error)
```

廃止候補 (本 spec では削除しない、follow-up):
- `xdg.WorktreesDir()` 等で `~/.local/share/worktrees/` を返している helper があれば不使用化を確認 (実機 grep 後に判断)

### 6.2 `cmd/claude-tmux-new/main.go`

変更点:

1. `parseArgs`:
   - `--worktree-base` フラグを **deprecated**: 受け取っても warn ログ (`claude-tmux-new: --worktree-base is deprecated; using <repo>/.dmux/worktrees instead`) を出して値は無視
   - 既存 `--prompt` / `--no-claude` / `--continue` / `--resume` / `--fork-session` / branch positional は無変更
2. `resolveWorktree`:
   - main repo の toplevel を `gitwt.TopLevel()` で取得
   - base = `gitwt.DmuxWorktreeRoot(toplevel)`
   - dir name = `gitwt.SanitizeSlug(branch)`
   - 既存の `existing → local ref → remote ref → HEAD` 3-path state machine は path 計算のみ差し替え、ロジック不変
3. `setupNewWindow`:
   - 起動直後 (worktree 確定後 / window 作成前) に `gitwt.EnsureGitignoreEntry(toplevel, ".dmux/")` を呼ぶ
   - `changed=true` なら stderr に 1 行 info、`err != nil` なら `obslog.Warn` のみ
   - 既存の 2-pane split + `claude --prompt`/`--continue`/`--resume`/`--fork-session` 起動経路は無変更
4. `claude-` prefix 統一済み binary 名は無変更 (Phase C で確定)

### 6.3 `cmd/claude-kill-session/main.go`

変更点:

1. `isClaudeManaged` の 3 段安全チェック (`@claude-managed=yes` OR pane に `claude` OR session が `claude-` prefix) は無変更
2. worktree path 取得後、削除直前に sanity check を 1 行追加:
   ```go
   if !strings.HasPrefix(wtPath, gitwt.DmuxWorktreeRoot(mainRepo)+string(os.PathSeparator)) {
       obslog.Warn("kill-session: worktree path %q is outside <main-repo>/.dmux/worktrees/, proceeding anyway", wtPath)
   }
   ```
3. block はしない (旧 path / 手動 migrate 中の worktree も削除可能)

### 6.4 `cmd/claude-branch-merge/main.go`

変更点:

1. `gitwt.FindByBranch` は `git worktree list --porcelain` を読むので新 path でも動作。**実装変更なし**
2. テスト期待値 (`main_test.go`) で worktree path を新規約に書き換え:
   - 旧: `~/.local/share/worktrees/chezmoi/feat-x/`
   - 新: `<repo>/.dmux/worktrees/feat-x/`
3. `--squash` / `--no-rebase` / `--fetch` フラグは独自価値として維持 (dmux merge UI には無い)

### 6.5 Slash commands (`branch-out.md` / `branch-finish.md` / `branch-merge.md`)

#### `branch-out.md`

手順 2 のコマンド呼び出しを変更:

```diff
-~/.local/bin/claude-tmux-new '<BRANCH>' \
-  --worktree-base "${XDG_DATA_HOME:-$HOME/.local/share}/worktrees" \
-  --prompt '<MESSAGE>'
+~/.local/bin/claude-tmux-new '<BRANCH>' \
+  --prompt '<MESSAGE>'
```

「補足」セクションの worktree 配置説明を:

```diff
-- worktree 配置: `~/.local/share/worktrees/<repo>/<sanitized-branch>` (XDG 中央集約。slash はハイフンに sanitize される)。
+- worktree 配置: `<repo>/.dmux/worktrees/<sanitized-branch>` (dmux 互換、project-relative。slash はハイフンに sanitize される)。
+- 初回 `/branch-out` 実行時、main repo の `.gitignore` に `.dmux/` 行が無ければ自動追記される (1 回のみ、コミットはユーザに委ねる)。
```

ブランチ名生成ロジック (`<type>/<kebab-summary>` の type 推定 + kebab 規則) は完全に維持。

#### `branch-finish.md`

「補足」セクションの worktree 配置例のみ更新。挙動 (claude-kill-session 呼び出し) は不変。

#### `branch-merge.md`

「補足」セクションの worktree 配置例のみ更新。挙動 (claude-branch-merge 呼び出し) は不変。

## 7. dmux Installation

### 7.1 配信方式

`mise` の global npm tool 機能を利用。既存 mise 環境 (`dot_config/mise/config.toml.tmpl` 等) を活用。

`.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` の Manjaro 分岐 (既に `rustup`, `paru`, etc. を入れている箇所) に追加:

```sh
{{ if eq .chezmoi.osRelease.id "manjaro" -}}
# ...existing rustup / paru / mise installations...
mise use -g npm:dmux@latest
{{ end -}}
```

### 7.2 検証フロー

`run_onchange_after_build-claude-tools.sh.tmpl` の末尾に dmux の存在確認を 1 行追加 (informational):

```sh
if ! command -v dmux >/dev/null 2>&1; then
  echo "[chezmoi] dmux not installed; /branch-out works in standalone mode (run 'mise use -g npm:dmux@latest' to install)" >&2
fi
```

エラーにはせず exit 0。dmux 未インストールでも `claude-tmux-new` は機能する (REST API 統合は本 spec で導入しない)。

### 7.3 アンインストール / バージョン pin

- アンインストール: `mise unuse -g npm:dmux`
- pin: `mise use -g npm:dmux@<version>` で固定。lockfile は `~/.config/mise/config.toml` 側で管理
- `paru` AUR (`dmux-bin` 等) は本 spec で採用しない (上流安定 AUR が未確認のため)

## 8. Error Handling

| 失敗ポイント | 挙動 |
|---|---|
| `git worktree add` が失敗 | 既存どおり stderr 表示 + exit 1。リトライ自動化なし |
| `git rev-parse --show-toplevel` が失敗 (リポジトリ外) | 既存どおり stderr + exit 1 |
| `EnsureGitignoreEntry` が write 失敗 (FS readonly 等) | `obslog.Warn` のみ。`/branch-out` の主処理は継続 |
| `claude-kill-session` の path sanity check NG | `obslog.Warn` のみ。削除は実施 |
| `claude-branch-merge` の rebase conflict / merge conflict | 既存どおり exit 1。conflict はユーザ手動解決 |
| dmux 未インストール状態で `dmux` コマンドが叩かれた | dmux 自身のエラー (npm bin 不在) が表示される。`claude-tmux-new` は影響なし |
| dmux インストール時 `mise use` 失敗 | `run_once_*` が exit 1 で chezmoi apply が止まる。手動で `mise doctor` で原因切り分け |

## 9. Testing Strategy

### 9.1 新規 unit test

| パッケージ | テストケース | 検証内容 |
|---|---|---|
| `internal/gitwt` | `SanitizeSlug` table test 7 ケース | §4.2 の入出力一致 (dmux 上流の挙動と bit-exact) |
| `internal/gitwt` | `DmuxWorktreeRoot` 1 ケース | `filepath.Join` の単純結果 |
| `internal/gitwt` | `EnsureGitignoreEntry` 4 ケース | (1) 既存無 (新規 create) / (2) 既存有 (noop) / (3) 既存有だが trailing newline 無 / (4) write 不可 (mode 0444 で error) |
| `cmd/claude-tmux-new` | `parseArgs` の `--worktree-base` deprecated 経路 1 ケース追加 | warn が出て値が無視されること (capture stderr) |

### 9.2 既存 unit test 修正

| ファイル | 修正内容 |
|---|---|
| `cmd/claude-tmux-new/main_test.go` | `parseArgs` 期待値を `--worktree-base` 抜きに更新、`buildClaudeCommand` テストは無変更 |
| `cmd/claude-kill-session/main_test.go` | `isClaudeManaged` テストは無変更、worktree path を新規約に更新 |
| `cmd/claude-branch-merge/main_test.go` | path 期待値を新規約に更新 |

### 9.3 race detection / coverage

- `go test -race ./...` 全 24+ パッケージ PASS
- `internal/gitwt` カバレッジ目標: 74.6% (現状) → 80%+ (`SanitizeSlug` + `EnsureGitignoreEntry` の table test 追加で改善見込み)
- `cmd/claude-tmux-new` カバレッジ: 24.5% (Phase C 既知の課題) → 本 spec では deprecated 経路追加分のみ寄与。80% 達成は別 follow-up (todos.md G-2 PR-C-4 follow-up に記載済み)

### 9.4 Smoke test

新規ファイル `docs/superpowers/smoke/2026-05-05-dmux-migration-smoke.md` に手順を記載:

1. `/branch-out "test dmux migration smoke"` で `<repo>/.dmux/worktrees/feat-test-dmux-migration-smoke/` が生成されること
2. main repo の `.gitignore` に `.dmux/` 行が自動追記されていること (`grep -E '^\.dmux/?$' .gitignore`)
3. 子 claude pane で `test dmux migration smoke` がプロンプトに pre-fill されていること
4. `/branch-merge main fetch` で target に統合できること
5. `/branch-finish` で worktree + window が削除されること
6. `dmux` を main repo で起動し、`/branch-out` で生成済み worktree を pane として認識・操作できること (dmux UI 統合の最低限の検証 — 失敗しても本 spec では block しない)

## 10. Acceptance Criteria

- [ ] `internal/gitwt` に `SanitizeSlug` / `DmuxWorktreeRoot` / `EnsureGitignoreEntry` の 3 関数が追加され、table test が PASS
- [ ] `claude-tmux-new` が `<repo>/.dmux/worktrees/<flat-slug>/` に worktree を生成
- [ ] `claude-tmux-new` 起動時に main repo の `.gitignore` に `.dmux/` が冪等追記される (changed=true 時に stderr に info)
- [ ] `claude-kill-session` が新 path 規約の worktree を削除できる、旧 path にも警告のみで対応
- [ ] `claude-branch-merge` が新 path 規約のソース worktree を target ブランチに統合できる (`--squash` / `--no-rebase` / `--fetch` 動作)
- [ ] `/branch-out` slash command が `--worktree-base` フラグなしで起動するよう更新
- [ ] `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` の Manjaro 分岐に `mise use -g npm:dmux@latest` が追加され、`chezmoi apply` で dmux が導入される
- [ ] `go test -race ./...` が全 24+ パッケージで PASS
- [ ] `chezmoi diff` で意図した差分のみ (`programs/claude-tools/` Go ソース + `programs/claude-plugins/.../commands/*.md` + `.chezmoiscripts/run_once_*.sh.cmd.tmpl`)
- [ ] §9.4 smoke test 1〜5 が実機で PASS (項目 6 は dmux 統合確認、failure tolerated)
- [ ] `docs/todos.md` の F-7 entry に「dmux 互換化済み」の補足追加、G-2 PR-C-* との関係を相互リンク

## 11. Follow-ups (本 spec 後の派生タスク)

実装完了後 (Phase D 着手前) に判断:

1. **D-1**: dmux REST API 統合の検討再開 (アプローチ 2)。dmux TUI が日常運用に組み込まれた段階で、`claude-tmux-new` を probe → REST API fallback 構造に拡張するか判断
2. **D-2**: `internal/cockpit` を `.dmux/hooks/{worktree_created,pre_merge,post_merge}` から writeable にする薄いブリッジ (B-3 の段階導入)
3. **D-3**: `cmd/claude-tmux-new` のカバレッジ 80% 達成 (G-2 PR-C-4 follow-up と統合)
4. **D-4**: `xdg.WorktreesDir()` 系の旧 path helper があれば削除 (実機 grep 後判断)
5. **D-5**: `dmux` の version pin 戦略 (`mise use -g npm:dmux@<version>`) を `dot_config/mise/config.toml.tmpl` に明示するか判断
6. **D-6**: tmux キーバインド (`prefix + C` series) と dmux TUI キーバインド (`n` / `m` / `x`) の衝突回避設計 (両方を tmux 内で常用するシナリオが定着したら)

## 12. References

- 上流 dmux ソース (commit time of writing は `main` HEAD):
  - [`src/utils/paneNaming.ts`](https://github.com/standardagents/dmux/blob/main/src/utils/paneNaming.ts) — `sanitizeWorktreeSlugFromBranch` / `resolvePaneNaming`
  - [`src/utils/slug.ts`](https://github.com/standardagents/dmux/blob/main/src/utils/slug.ts) — `generateSlug` (本 spec では未利用)
  - [`context/HOOKS.md`](https://github.com/standardagents/dmux/blob/main/context/HOOKS.md) — `.dmux/hooks/` 仕様 (本 spec では未利用、B-3 で参照予定)
  - [`context/API.md`](https://github.com/standardagents/dmux/blob/main/context/API.md) — REST API 仕様 (本 spec では未利用、D-1 で参照予定)
- 親 spec: `docs/superpowers/specs/2026-05-02-tmux-scripts-go-migration-design.md`
- 関連 plan: `docs/superpowers/plans/2026-05-02-tmux-scripts-go-migration.md`
- todos: `docs/todos.md` の F-7 / G-2 PR-C-1〜PR-C-5 / 「デファード」B-3 派生
