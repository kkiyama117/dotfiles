# 複数 Claude Code セッション管理ガイド

作成日: 2026-04-29
対象: ローカル開発で 2〜複数の Claude Code セッションを並行運用したいケース
前提環境: Linux (Manjaro) / zsh + sheldon / tmux

---

## 1. Claude Code 標準機能（追加インストール不要）

### セッション管理コマンド
- `claude --resume` / `claude -c` — 直近のセッションを再開（会話履歴を引き継ぐ）
- `claude --continue --fork-session` — 現在の履歴を引き継ぎつつ**新しい session ID** を発行。複数ターミナルでの並行作業に最適
- `/resume` — セッション一覧から選択して再開

### Git Worktree（公式推奨）
ディレクトリ単位でセッションを完全分離する最も堅実な方法。
```bash
# フィーチャーブランチ用に独立したディレクトリを作成
git worktree add ../feature-x feature/x
cd ../feature-x
claude   # 新しいセッションが別ディレクトリで開始

# 終了時
cd ..
git worktree remove feature-x
```
利点:
- ディレクトリ単位でセッションが独立 (`.claude/projects/` の session ID も自動分離)
- branch / working tree が物理的に分かれるため context 競合が起きない

### バックグラウンド実行
```bash
claude "task description" &
```
Bash tool 内では `run_in_background` パラメータで tool-level のバックグラウンド化も可能。

---

## 2. ECC（Everything Claude Code）スキル

`settings.json` に登録済みのスキルから、複数セッション運用に直接効くもの:

| スキル | 用途 | 適規模 |
|---|---|---|
| `/dmux-workflows` | tmux ペイン管理で複数エージェント並行（`n` で追加・`m` で結果統合） | 3〜5 並行 |
| `/claude-devfleet` | DAG（有向非環グラフ）計画 → 隔離 worktree → parallel dispatch | 大規模・複数機能同時開発 |
| `/autonomous-agent-harness` | persistent memory + scheduling で完全自動化 | 長時間ループ |
| `/autonomous-loops` | quality gate / recovery 付き連続ループ | 反復タスク |

### 使用例
```text
# tmux ベースで軽量並行
claude "/dmux-workflows"
# セッション内: n → "Fix auth bug" / n → "Optimize DB queries" / m で統合

# 計画駆動で大規模並行
claude "/claude-devfleet"
# "Refactor API layer, add caching, update tests, migrate DB"
# → 自動 DAG 計画 → 複数 worktree で parallel dispatch
```

---

## 3. 推奨セットアップ（ソロ開発 2〜4 セッション）

最小コストで最高信頼性のセットアップは **Git Worktree + `--fork-session` + tmux ウィンドウ分割**。

### tmux + worktree パターン
```bash
# tmux ウィンドウ 1: フィーチャー A
tmux new-window -n claude-a
git worktree add ../feat-a feature/a
cd ../feat-a
claude --fork-session "Build feature A"

# tmux ウィンドウ 2: フィーチャー B
tmux new-window -n claude-b
git worktree add ../feat-b feature/b
cd ../feat-b
claude --fork-session "Build feature B"

# tmux ウィンドウ 3: テスト・リファクタ（main 留置）
tmux new-window -n claude-test
cd /path/to/main
claude --fork-session "Run full test suite"

# 後から再開
tmux attach -t <session>
```

### スケール判断
| 状況 | 推奨アプローチ |
|---|---|
| 2〜4 個の独立タスク | Git Worktree + `--fork-session` + tmux |
| 5 個前後の軽量並行 | `/dmux-workflows` |
| 計画駆動の大規模並行 | `/claude-devfleet` |
| 長時間自動ループ | `/autonomous-agent-harness` / `/autonomous-loops` |

---

## 4. 注意点

- `BW_SESSION` 等の機密 env は worktree 横断で継承されるため、不要セッションでは `bw_lock` で破棄する（CLAUDE.md 参照）
- `chezmoi apply` 対象のリポジトリ（`~/.local/share/chezmoi`）で worktree を切るときは、`.chezmoiignore` の更新影響範囲に注意
- 並行セッション同士で同じファイルを編集しないようタスク分割すること（git worktree でも working tree が同じ branch を指すと衝突する）

---

## 5. このリポジトリ独自の tmux + 通知セットアップ

このリポジトリ (`chezmoi`) は §3 のワークフローを **キーバインド一発** で実行できる tmux 統合を備える。生の `tmux new-window` / `git worktree add` を毎回打つ代わりに `prefix + C, n` のような二段ショートカットで等価操作が走る。

> **使い方の早見表**は [`docs/claude_tmux_cheatsheet.md`](./claude_tmux_cheatsheet.md) を参照。本節は実装側の仕様・設計判断を記録する。

### 5.1 tmux prefix と直下バインド

prefix は **`C-b`** (tmux デフォルト)。`dot_config/tmux/conf/options.conf` で `set -g prefix C-b`。過去に C-t / C-Space / C-\ / C-] / F12 / C-a を試したが、いずれも真因 (Ghostty + X11 + GTK DE での `focus-events on` 暴発、ghostty-org/ghostty#9620) を見落とした早合点だった。現在は kitty 主運用に移行したため `focus-events on` でも `C-b` 暴発は再現せず、resurrect / nvim autoread / pane border 動的更新の恩恵を得るために `on` を維持している — 詳細は [`docs/keybinds.md`](./keybinds.md)。

| 打鍵 | 動作 | 実装 |
|---|---|---|
| `prefix + g` | 現リポジトリの **git worktree** を fzf で選び新 window で開く | `bindings.conf` インライン |
| `prefix + s` | **全 tmux session** を fzf で選び switch-client | `bindings.conf` インライン |
| `prefix + C` | **claude_table** へ二段プレフィックス遷移（次のキー待ち） | `switch-client -T claude_table` |
| `prefix + h/j/k/l` | vim 風 pane 移動 | — |
| `prefix + \|` / `_` | カレント PWD を継承して水平 / 垂直分割 | — |
| `prefix + M-c` / `M-C` | 現 PWD で新 window / 新 session | — |
| `prefix + J/K` / `L/H` | 次/前 window / 次/前 session | — |
| `prefix + q` | 現 pane を kill | — |

### 5.2 claude_table（`prefix + C` 後の二段目）

`prefix + C` を押すと status-left に `⟨claude_table⟩` が表示され、次のキー入力 1 つで以下が走る (`bindings.conf` の `bind -T claude_table ...` 群):

| キー | 動作 | 実装スクリプト |
|---|---|---|
| `c` | 現 pane で `claude --continue` を送信 | inline `send-keys` |
| `n` | git branch を fzf で選び **新規 worktree + 2-pane Claude session** を作成 | `claude-pick-branch.sh` → `tmux-claude-new.sh` |
| `o` | git branch を fzf で選び **新規 worktree (shell-only)** を作成（claude を起動しない） | `claude-pick-branch.sh --no-claude` → `tmux-claude-new.sh --no-claude` |
| `r` | 現 session 内の `claude` pane を kill → `claude --continue` で再起動 | `claude-respawn-pane.sh` |
| `s` | tmux session/window/pane を **階層 fzf** で表示し、cockpit state badge 付きで switch / kill / reload | `cockpit/switcher.sh` |
| `N` | `done` 状態の pane に **inbox 順 (session asc / window idx asc / pane idx asc)** で循環ジャンプ | `cockpit/next-ready.sh` |
| `k` | 現 session が `claude-*` なら **session + worktree を一括削除**（`confirm-before` で確認） | `claude-kill-session.sh` |

### 5.3 ヘルパースクリプト (`dot_config/tmux/scripts/`)

| スクリプト | 役割 |
|---|---|
| `tmux-claude-new.sh <branch> [--from-root [<id>]] [--no-claude]` | branch 名から `claude-<safe>` session 名を作り、`<repo-root>-<safe>` に git worktree を生成、左 pane = shell / 右 pane = `claude --continue --fork-session` の 2-pane session を起動。**branch が既に他 worktree にチェックアウト済みなら**（main repo 含む）`git worktree list` から既存パスを再利用し新規 add は行わない。新規作成時はローカル → `origin/<branch>` → 現在の HEAD の順に解決し、未存在なら HEAD 起点で **新規ブランチを自動作成** する。worktree に対応する `~/.claude/projects/<encoded>/*.jsonl` が **空なら `--continue` を付けず** 素の `claude` を起動して "Fatal Error" を回避。`--from-root` を付けると **メイン worktree の Claude セッション履歴**（`~/.claude/projects/<encoded main repo>/`）から fzf で選択（`<id>` 直指定も可）し、`claude --resume <id> --fork-session` で起動する。`--no-claude` を付けると **claude を起動せず 1-pane shell session のみ** 作成する（`--from-root` と排他） |
| `claude-pick-branch.sh [--no-claude]` | fzf で branch を選択 → `tmux-claude-new.sh` を `exec`（追加引数は passthrough） |
| `cockpit/switcher.sh` | tmux 全 session/window/pane を fzf 階層表示。Enter=switch / Ctrl-X=kill (worktree-aware) / Ctrl-R=reload |
| `cockpit/next-ready.sh` | inbox 順で `done` 状態の pane に循環ジャンプ |
| `claude-respawn-pane.sh` | session 内で `pane_current_command == claude` の pane を見つけて `respawn-pane -k` |
| `claude-kill-session.sh` | `claude-*` session 限定で `kill-session` + `git worktree remove --force` |
| `cockpit/summary.sh` | hook 駆動キャッシュ（`~/.cache/claude-cockpit/panes/*.status`）から `⚡ N ⏸ M ✓ K ` 形式で status-right に出力 |
| `cockpit/prune.sh` | tmux に存在しない pane の cache file を削除（idempotent） |
| `claude-cockpit-state.sh` | Claude hook entry — UserPromptSubmit/PreToolUse → working、Notification → waiting、Stop → done を atomic write |
| `claude-branch.sh <path>` | path の git branch を `[branch] ` 形式で status-right に出力 |
| `tpm-bootstrap.sh` | TPM (`~/.config/tmux/plugins/tpm/`) のクローン + `install_plugins` 実行（idempotent） |

### 5.4 zsh ラッパー

シェル直叩き用に zsh 関数を 1 つ用意 (`dot_config/zsh/rc/my_plugins/tmux.zsh`):

```zsh
tmux_claude_new <branch>   # ~/.config/tmux/scripts/tmux-claude-new.sh の薄ラッパー
```

`prefix + C, n` の popup と同じロジックを通常のシェルからも呼べる。

### 5.5 ステータスバー

`dot_config/tmux/conf/status.conf`:

- **左**: `[<session>:<window>:<pane>]` + `⌘` (prefix 入力中) + `⟨claude_table⟩` (key-table 進入中)
- **右**: `⚡ N ⏸ M ✓ K ` (cockpit state aggregate; hook 駆動キャッシュから生成、空なら非表示) + `[branch]` (現 pane の git branch) + 時刻
- 5 秒間隔で更新

`dot_config/tmux/conf/claude.conf` で pane-border-format を上書きし、**`pane_current_command == claude` の pane は黄色枠**でハイライト。

### 5.6 TPM プラグイン

`dot_config/tmux/conf/plugins.conf` で TPM 経由で 3 つ宣言:

| プラグイン | 役割 | 関連設定 |
|---|---|---|
| `tmux-plugins/tmux-resurrect` | `prefix + Ctrl-s` で session 状態 (pane 内容含む) を保存 / `prefix + Ctrl-r` で復元 | `@resurrect-capture-pane-contents 'on'`, `@resurrect-strategy-nvim 'session'` |
| `tmux-plugins/tmux-continuum` | 15 分間隔で自動保存 + tmux 起動時に自動復元 | `@continuum-save-interval '15'`, `@continuum-restore 'on'` |
| `tmux-plugins/tmux-yank` | コピーモードのバッファをシステムクリップボードへ | — |

TPM 自体は `~/.config/tmux/plugins/tpm/` (XDG パス) に配置。新規マシンでは `chezmoi run_once` 内で `tpm-bootstrap.sh` が走り自動インストールされる。手動実行も同コマンドで可能。

### 5.7 デスクトップ通知 + サウンド

ターン完了 / 入力待ちで **音 + デスクトップ popup** を出す。hook 入口は `dot_local/bin/executable_claude-notify-hook.sh` で、内部で sound 再生 (`claude-notify-sound.sh`) と popup + click action (`claude-notify-dispatch.sh`) を fork する 3 ファイル構成:

| トリガー | 音 | 通知タイトル | urgency |
|---|---|---|---|
| `Notification` (入力待ち) | `message.oga` | Claude Code | normal |
| `Stop` (ターン完了) | `complete.oga` | Claude Code | normal |
| `subagent-stop` | `bell.oga` | Claude Code | low |
| `error` | `dialog-error.oga` | Claude Code | critical |

クリックアクション (`dot_config/wired/wired.ron` の `shortcuts` で定義):

| ボタン | 動作 |
|---|---|
| 左クリック | 発火元の tmux session/pane に focus を戻し、popup は自動で消える (`claude-notify-dispatch.sh` が `--action=default=Focus` を介して受信 → `tmux switch-client` + `select-pane` → `gdbus CloseNotification`) |
| 中クリック | 滞留した popup を一括 close (`notification_closeall`) |
| 右クリック | この popup だけ close (`notification_close`)。focus 等の副作用なし |

tmux 外で起動された Claude (素の terminal) は左クリック時 `journalctl --user -t claude-notify-dispatch` に "focus skipped" を出した上で、`xdotool` (X11) / `swaymsg` (Wayland) で kitty / ghostty / wezterm / Alacritty 系の terminal window を class ベースで raise する fallback を実行する (cwd ベース絞り込みは F-3.next の残タスク)。

同一セッションの de-dup (F-3.next #1):

`CLAUDE_NOTIFY_SESSION_ID` が渡された呼び出しでは、dispatcher が `${XDG_RUNTIME_DIR:-/tmp}/claude-notify/sessions/<sid>.id` に直近の notification id を保存し、次回起動時に `notify-send --replace-id` で再利用する。同セッションの連続発火は **popup 1 個に in-place 更新** され、別セッション同士は独立した popup として表示される。state file は session 終了後も残るが、古い id への replace は wired が無視するため無害 (cleanup は別 follow-up)。

ポイント:

- `notify-send --expire-time=0` で **自動消去せず居残る**（ユーザがクリックして dismiss するまで表示）
- `pw-play` → `paplay` → `ffplay` の順でフォールバック再生
- 通知 daemon は **wired-notify** (`dot_config/wired/wired.ron` + `dot_config/systemd/user/wired.service`)。`--expire-time=0` を尊重して popup が永続化される
- フック登録は `dot_config/claude/settings.json` の `hooks.Notification` / `hooks.Stop` 双方とも新設 orchestrator (`claude-notify-hook.sh`) を呼び出し、内部で `claude-notify-sound.sh` (sound 専用 worker) と `claude-notify-dispatch.sh` (popup + click action ハンドラ) を fork する 3 ファイル構成

## 6. 公式ドキュメント

- セッション管理: <https://code.claude.com/docs/en/how-claude-code-works.md>
- Agent Teams: <https://code.claude.com/docs/en/agent-teams>
- Common workflows（worktree 含む）: <https://code.claude.com/docs/en/common-workflows>
- ECC スキル一覧: <https://github.com/affaan-m/everything-claude-code>

## Cockpit State Tracking — Smoke Tests

> 4/30 spec の手動検証手順。chezmoi apply 後・tmux reload 後に通すこと。

1. `tmux-claude-new.sh feature-foo` と `tmux-claude-new.sh feature-bar` で 2 セッション作成
2. `claude-foo` で `Hello` と送信 → status-right が `⚡ 1 ` に更新（最大 5 秒）
3. Claude が応答完了し ESC で待機 → status-right が `✓ 1 ` に変わる
4. もう片方でも送信 → 状態が混在表示される（条件次第で `⚡ 1 ⏸ 1` 等）
5. `prefix + C` → `N` で done 側 pane にジャンプできる
6. `prefix + C` → `s` で階層 switcher を開き、`Ctrl-X` で空 pane を kill できる
7. `prefix + C` → `k` で claude-foo セッション + worktree を削除 → `~/.cache/claude-cockpit/panes/claude-foo_*.status` も消える
8. `tmux kill-server` → 再起動後、`~/.cache/claude-cockpit/panes/` の残骸が `prune.sh` により消える

合格条件: 1〜8 すべて期待通りになること。
