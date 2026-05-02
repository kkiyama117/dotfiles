# lazygit Config Migration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

作成日: 2026-05-02
ブランチ想定: `feat/lazygit-config` (本プランは `docs/lazygit` ブランチで作成)
関連: [`docs/application_summary/lazygit.md`](../../application_summary/lazygit.md) / [`docs/keybinds.md`](../../keybinds.md)

**Goal:** lazygit を本リポジトリ (chezmoi dotfiles) で **再現可能かつ宣言的に** 管理する。新規マシンの bootstrap (`chezmoi init --apply`) だけで、インストール・delta 統合済み config・tmux popup ショートカット・既存ワークフロー (`prefix + g` worktree picker / claude_table) との非衝突を保証する。

**Architecture:** 4 つの独立した編集面で構成する。
1. **インストール経路**: `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` の paru PACKAGES heredoc に `lazygit` を追加（Manjaro guard 内、既存パターンに準拠）。
2. **config 本体**: `dot_config/lazygit/config.yml` を新規作成。delta ペイジャ統合 / 既存 git 設定 (`dot_config/git/`) 尊重 / nerd-font 前提。テンプレ展開は不要のため `.tmpl` 化しない。
3. **tmux 統合**: `dot_config/tmux/conf/bindings.conf` に **`prefix + G` (大文字)** で lazygit popup を bind（小文字 `g` は worktree picker で占有済のため衝突回避）。`-d "#{pane_current_path}"` で起点 PWD 継承。
4. **ドキュメント追従**: `docs/keybinds.md` Part II §2.1.1 と Part I §I-6 にエントリ追加、`docs/application_summary/lazygit.md` に「dotfiles 統合済」追記。

**Tech Stack:** chezmoi / lazygit ≥ 0.50 / git-delta ≥ 0.18 (既に `cargo install git-delta` 経由でインストール済み) / tmux 3.6 / paru / Manjaro Linux.

**Spec:** （本プランのみ。スコープが小さいため別 spec は作らない。設計判断は各 Task 冒頭に inline で記述する。）

**Files Affected:**

| chezmoi source | apply target | Action |
|---|---|---|
| `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` | (run-once shell) | **Modify** (PACKAGES heredoc に `lazygit` 1 行追加) |
| `dot_config/lazygit/config.yml` | `~/.config/lazygit/config.yml` | **Create** (新規) |
| `dot_config/tmux/conf/bindings.conf` | `~/.config/tmux/conf/bindings.conf` | **Modify** (`bind G display-popup ...` 追加) |
| `docs/keybinds.md` | (docs only) | **Modify** (§2.1.1 に `prefix + G`, §I-6 に config.yml パス) |
| `docs/application_summary/lazygit.md` | (docs only) | **Modify** (「この dotfiles リポへの示唆」節を「実装済み」に更新) |

**Out of scope (将来追加検討):**

- `customCommands` による AI commit (`!aic` 相当) — Claude Code 連携が固まってから別プランで扱う。
- `lazygit.nvim` プラグイン導入 — Neovim plugin manager 全体方針に依存するため別議論。
- macOS 対応の Homebrew install — 現状 dotfiles の主ターゲットが Manjaro なのでスコープ外。
- カラースキーム / theme カスタマイズ — まず素の挙動で運用してから判断。

---

## Task 1: Add lazygit to paru install list

**Files:**
- Modify: `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl:63-82` (PACKAGES heredoc)

**目的:** 新規 Manjaro マシンで `chezmoi init --apply kkiyama117` が走った瞬間に lazygit がインストールされる状態を作る。既存パッケージ (bat / fd / fzf / tmux 等) と並べるため heredoc に 1 行追加するだけ。

- [ ] **Step 1: PACKAGES heredoc に lazygit を追加**

heredoc 末尾どこでも可だが、視認性のため `lsd` の直後 (`navi` の前) に挿入する:

```diff
 bat
 fd
 lsd
+lazygit
 navi
 onefetch
 ripgrep
```

> 既に lazygit が手動 install 済みでも `paru -S ... --needed --noconfirm` は idempotent なので副作用なし。

- [ ] **Step 2: 既存マシンで反映**

```bash
paru -S lazygit --needed --noconfirm
command -v lazygit && lazygit --version
```

期待: `paru` が pacman 公式リポからの lazygit を入れる。バージョン 0.50 以降であること。

- [ ] **Step 3: chezmoi 経由の verify**

```bash
chezmoi diff .chezmoiscripts/run_once_all_os.sh.cmd.tmpl
# diff に lazygit 追加行が出ること
```

`run_once_*` は state-tracked なので、テンプレートが変わると次回 `chezmoi apply` で再実行される (`chezmoi state delete-bucket --bucket=scriptState` でリセット可能)。今回は手動で `paru -S lazygit` 済みなら再実行は無害。

---

## Task 2: Create `dot_config/lazygit/config.yml`

**Files:**
- Create: `dot_config/lazygit/config.yml` (新規)

**目的:** delta ページャ統合 + 既存 `dot_config/git/config.tmpl` (Bitwarden 注入の user.* / signingkey) を尊重した最小設定。色や key bind はデフォルト維持し、まずは "動く・読みやすい diff" の 2 点に絞る。

設計判断:
- **`.tmpl` 化しない**: Bitwarden / OS 分岐は不要 (lazygit は user 情報を git config から読む)。
- **delta は `--paging=never`** で渡す: lazygit 側でページングしているため delta 内蔵 less は二重ページングになる。lazygit 公式 wiki の推奨パターン。
- **`os.editPreset: nvim`**: `dot_config/nvim/` が別途存在する想定で nvim を使う。
- **`gui.nerdFontsVersion: "3"`**: PlemolJP (`ttf-plemoljp-bin`) は Nerd Font v3 ベース、PACKAGES に既存。
- **`git.commit.verbose: true`**: コミットメッセージ編集中に diff を別 panel で表示。
- **`promptToReturnFromSubprocess: false`**: tmux popup 起動時に lazygit を抜けたら即 popup を閉じたい (Enter 待ちが煩わしい)。

- [ ] **Step 1: ファイル作成**

`dot_config/lazygit/config.yml` を以下の内容で新規作成:

```yaml
# ~/.config/lazygit/config.yml
# 管理: chezmoi (dot_config/lazygit/config.yml)
# 詳細: docs/application_summary/lazygit.md / docs/keybinds.md Part I §I-6

gui:
  # PlemolJP (ttf-plemoljp-bin) は Nerd Font v3 ベース。
  nerdFontsVersion: "3"
  # ファイルパネルをツリー表示 (long path のリポで便利)。
  showFileTree: true
  # 起動時の loader 表示 (大規模リポでの待ち時間可視化)。
  showBottomLine: true
  # command log は閉じておく (画面を広く使う)。
  showCommandLog: false

git:
  # delta は cargo install git-delta で入っている (/usr/bin/delta)。
  # lazygit 側でページングしているため delta は --paging=never で渡す。
  paging:
    colorArg: always
    pager: delta --paging=never --line-numbers --side-by-side
  commit:
    # ~/.config/git/config.tmpl で Bitwarden から signingkey が注入される。
    # gpg sign は git config 側 (commit.gpgsign) に従わせるため lazygit では触らない。
    signOff: false
    # コミットメッセージ編集中に diff を別 panel で表示。
    verbose: true
  # auto-fetch を有効化。upstream 状態を最新に近づける。
  autoFetch: true
  log:
    order: topo-order
    showGraph: always

# tmux popup 起動時に lazygit 終了で即 popup を閉じる (Enter 待ち抑止)。
promptToReturnFromSubprocess: false

os:
  # 外部エディタは nvim。chezmoi 管理の dot_config/nvim/ がエントリポイント。
  editPreset: nvim

# startup popup を抑止 (毎回出る release notes が煩わしい)。
disableStartupPopups: true
confirmOnQuit: false
```

> ⚠ lazygit 公式 wiki に注意。`os.open` 等のテンプレ構文は **lazygit 独自** (Go text/template ではない、`{{filename}}` 形式)。chezmoi の `.tmpl` 解釈とは別物だが、ファイル名を `config.yml` (`.tmpl` なし) にすれば衝突回避できる。本プランでは `.tmpl` 化しない。

- [ ] **Step 2: chezmoi diff & apply**

```bash
chezmoi diff dot_config/lazygit/config.yml
chezmoi apply dot_config/lazygit/config.yml
ls -la ~/.config/lazygit/config.yml
```

- [ ] **Step 3: 動作確認**

```bash
cd ~/.local/share/chezmoi
lazygit
```

確認項目:
- [ ] 起動時に startup popup が出ない (`disableStartupPopups: true`)
- [ ] `<space>` でファイルステージ → `c` でコミットしたとき、message buffer 下部に diff が出る (`commit.verbose: true`)
- [ ] commit 一覧で graph が常時表示される (`log.showGraph: always`)
- [ ] 任意のコミットを `<enter>` で開いた diff が **delta スタイル** (line numbers + side-by-side) で出る
- [ ] `q` で終了したとき "Press enter to return" が出ない (`promptToReturnFromSubprocess: false`)

---

## Task 3: Bind tmux popup for lazygit (`prefix + G`)

**Files:**
- Modify: `dot_config/tmux/conf/bindings.conf` (新規 bind を追加)

**目的:** どの pane からでも 1 ストロークで lazygit を起動できるようにする。`prefix + g` (小文字) は **既に worktree picker で占有**されているため、大文字 `G` を使う (Part II §2.1 / §2.1.2 でも未使用)。

設計判断:
- **キー選定**: `prefix + G` を採用。理由:
  - Part II §2.1.2 (tmux デフォルト全リスト) で `G` は未使用 (`prefix + ?` で確認可)
  - 小文字 `g` (worktree) との対称性で覚えやすい (`g` = worktree、`G` = git GUI)
  - claude_table (`prefix + C, ...`) に押し込む案もあったが、Claude 文脈と独立した汎用ツールなので prefix-table 直下が適切
- **popup サイズ**: 95%×95% — 部分表示だと commit graph と diff panel が窮屈、かつステータスバーは残したい
- **PWD 継承**: `-d "#{pane_current_path}"` — 現 pane のリポジトリで lazygit を開く (claude_table.s と同じパターン)
- **`-E`**: subprocess 終了で popup を閉じる (Task 2 の `promptToReturnFromSubprocess: false` と一致)

- [ ] **Step 1: bindings.conf に追加**

`prefix + g` (worktree picker) の bind の **直後**に追加すると、g/G ペアが視覚的に並ぶ:

```diff
 # prefix + g : 現リポジトリの git worktree から選んで新ウィンドウで開く
 bind g display-popup -E "git worktree list --porcelain | awk '/^worktree /{print \$2}' | fzf --prompt='worktree> ' --height=100% | xargs -r -I {} tmux new-window -c {}"

+# prefix + G : 現 pane の cwd で lazygit を popup 起動
+bind -N "lazygit popup (cwd)" G display-popup -E -w 95% -h 95% -d "#{pane_current_path}" "lazygit"
+
```

> `-N` ノートを付けることで `prefix + ?` の list-keys 出力に説明文が出る (claude_table と同流儀)。

- [ ] **Step 2: chezmoi apply & tmux reload**

```bash
chezmoi apply dot_config/tmux/conf/bindings.conf
tmux source-file ~/.config/tmux/tmux.conf
```

- [ ] **Step 3: 手動スモーク**

任意の tmux pane で:
1. `prefix + G` で lazygit popup が 95%×95% で開くこと
2. lazygit 内で `q` を押した瞬間に popup が消えること (Press enter 待ちが出ないこと)
3. 起点ディレクトリが現 pane の `pwd` であること (`echo $PWD` と lazygit 上部のリポ名一致)
4. `prefix + ?` → `less` 内で `lazygit popup (cwd)` の行が見えること

衝突確認:
- [ ] `prefix + g` (小文字) が引き続き worktree picker として機能する
- [ ] claude_table (`prefix + C, *`) に副作用がない

---

## Task 4: Update docs

**Files:**
- Modify: `docs/keybinds.md` (Part I §I-6, Part II §2.1.1)
- Modify: `docs/application_summary/lazygit.md` (「この dotfiles リポへの示唆」節)

**目的:** keybinds.md がリポジトリ全キーバインドの正典である規約を維持する。

- [ ] **Step 1: `docs/keybinds.md` Part II §2.1.1 にエントリ追加**

`prefix + g` の行の直後に追加:

```diff
 | `prefix + g` | git worktree fzf picker | `bindings.conf` |
+| `prefix + G` | lazygit popup (現 pane cwd, 95%×95%) | `bindings.conf` |
```

- [ ] **Step 2: `docs/keybinds.md` Part I §I-6 にファイル追加**

ファイル位置クイックリンク表に追加:

```diff
 | `dot_config/tmux/scripts/` | claude 関連ヘルパースクリプト群 |
+| `dot_config/lazygit/config.yml` | lazygit 設定（delta ページャ統合 / nvim 連携） |
```

- [ ] **Step 3: `docs/application_summary/lazygit.md` の末尾節更新**

「## この dotfiles リポへの示唆」節を「## このリポでの統合状況」に書き換え、各項目を完了マーク+実装パスに更新:

```diff
-## この dotfiles リポへの示唆
+## このリポでの統合状況
 
-- 既に **paru でインストール済み**かは要確認（`run_once_all_os.sh.cmd.tmpl` への追加候補）
-- `dot_config/lazygit/config.yml` を chezmoi 管理にすると、**delta 統合・カスタムコマンド・キーバインド**が再現可能
-- shuntaka 氏の **`!aic` AI commit** や m3tech の **`prefix + g` popup** を chezmoi 化すれば tmux/Claude ワークフローと統合できる
+- ✅ paru インストール: `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` の PACKAGES に追加済
+- ✅ delta ペイジャ統合: `dot_config/lazygit/config.yml` で `delta --paging=never --side-by-side`
+- ✅ tmux popup: `prefix + G` で現 pane の cwd で起動 (`dot_config/tmux/conf/bindings.conf`)
+- 🔲 AI commit (`!aic`) カスタムコマンド: 別プランで Claude Code 連携が固まってから扱う
+- 🔲 `lazygit.nvim` プラグイン: Neovim plugin manager 全体方針と合わせて別議論
```

- [ ] **Step 4: `docs/todos.md` への追記** (任意・低優先)

実装完了後、`docs/todos.md` の「完了」セクションに 1 行追加:

```markdown
- [x] lazygit を chezmoi 管理化（install / config / tmux popup / docs）— `2026-05-02-lazygit-config-migration` プラン参照
```

---

## Task 5: End-to-end smoke test

**目的:** Task 1-4 の差分が **新規 shell + 新規 tmux session** で整合することを確認する。chezmoi state や zsh キャッシュに依存しない再現性を保証。

- [ ] **Step 1: Verify chezmoi diff is clean**

```bash
cd ~/.local/share/chezmoi
chezmoi diff
```

期待: lazygit 関連 4 ファイル以外に diff が出ないこと。

- [ ] **Step 2: Fresh tmux session smoke**

```bash
tmux kill-server  # ⚠ 他作業中の session も落ちる。先に detach 必要なものを保存
tmux new-session -ds smoke -c ~/.local/share/chezmoi
tmux attach -t smoke
```

中で:
1. `prefix + G` → lazygit popup 起動
2. ファイルステージ → コミット (verbose で diff 表示確認)
3. delta diff スタイル確認
4. `q` → popup 即時クローズ
5. `prefix + g` (小文字) で worktree picker が動くこと (回帰防止)

- [ ] **Step 3: bootstrap シミュレーション (optional)**

新規マシン相当の confirm をしたい場合:

```bash
# scriptState をリセットして run_once を強制再実行
chezmoi state delete-bucket --bucket=scriptState
chezmoi apply
# → paru -S lazygit が走り --needed で no-op
```

---

## Risks / Considerations

| リスク | 緩和策 |
|---|---|
| 既存 `~/.config/lazygit/config.yml` を上書きする | Task 0 の調査で「現在 unmanaged な config 無し」確認済 (`ls ~/.config/lazygit/` が空)。chezmoi diff は新規ファイル追加のみを示すはず |
| `prefix + G` が将来 tmux 標準で割り当てられる | tmux 3.6 までは未使用。`prefix + ?` で常時確認可能。衝突したら `dot_config/tmux/conf/bindings.conf` の bind 行を unbind+rebind で対応 |
| delta が無いマシンで lazygit が壊れる | delta は `cargo install git-delta` で入る (`run_once_all_os.sh.cmd.tmpl:40`)。Manjaro 以外では `run_once` 自体が `exit 0` するので問題化しない |
| popup サイズが小ディスプレイで窮屈 | 95%×95% は狭ディスプレイでも実用範囲。狭すぎたら `-w 100% -h 100%` に変更 |
| lazygit が大規模リポで固まる | `docs/application_summary/lazygit.md` §既知の制限 で文書化済。回避は repo 側の問題 (gitui への一時切替を検討) |

---

## Estimated Effort

| Task | 見積 |
|---|---|
| Task 1: paru install 行追加 | 5 分 |
| Task 2: config.yml 作成 | 15 分 (調整含む) |
| Task 3: tmux bind 追加 | 5 分 |
| Task 4: docs 更新 | 10 分 |
| Task 5: smoke test | 10 分 |
| **合計** | **約 45 分** |

---

## Dependencies / Pre-flight

- [x] lazygit 機能サマリ作成済: [`docs/application_summary/lazygit.md`](../../application_summary/lazygit.md)
- [x] delta インストール済 (`/usr/bin/delta` 存在確認済)
- [x] gh / bat / fzf インストール済
- [ ] 本プランのレビュー & 承認

実装着手は **本プランがマージされた後**、`feat/lazygit-config` ブランチを切って Task 1→5 の順で進める。
