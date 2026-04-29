# Claude Code × tmux 操作チートシート

作成日: 2026-04-29
対象: このリポジトリでセットアップ済みの claude / tmux 統合をすぐ使えるようにする実用リファレンス
詳細仕様（実装スクリプト・status-bar・通知フック等）は [`docs/manage_claude.md`](./manage_claude.md) §5 を参照。

---

## 1. claude コマンド（シェル直叩き）

```bash
claude                                  # 新規セッション
claude --continue                       # 直近セッションを継続（履歴を引き継ぐ）
claude --continue --fork-session        # 履歴を引き継ぎつつ新 session ID を発行（並行作業向け）
```

claude 内のコマンド:

```text
/resume       # セッション一覧から選んで再開
/help         # コマンド一覧
/exit         # セッション終了
```

zsh ラッパー（`dot_config/zsh/rc/my_plugins/tmux.zsh`）:

```bash
tmux_claude_new <branch>   # branch から worktree + 2-pane claude session を作成
                           # `prefix + C, n` の popup と同じロジックを CLI から呼ぶ薄ラッパー
```

---

## 2. tmux ショートカット（prefix = `C-b`）

### 2.1 直下バインド（`C-b` の次に 1 キー）

| 打鍵 | 動作 |
|---|---|
| `C-b g` | 現リポジトリの **git worktree** を fzf で選び新 window で開く |
| `C-b s` | **全 tmux session** を fzf で選び switch |
| `C-b h` / `j` / `k` / `l` | vim 風 pane 移動（左 / 下 / 上 / 右） |
| `C-b \|` | カレント PWD を継承して水平分割 |
| `C-b _` | カレント PWD を継承して垂直分割 |
| `C-b M-c` | 現 PWD で新 window |
| `C-b M-C` | 現 PWD で新 session |
| `C-b J` / `K` | 次 / 前 window |
| `C-b L` / `H` | 次 / 前 session |
| `C-b q` | 現 pane を kill |
| `C-b C` | **claude_table** へ二段プレフィックス遷移（次のキー待ち） |

コピーモード（vi）:

| 打鍵 | 動作 |
|---|---|
| `v` | 選択開始 |
| `V` | 行選択 |
| `C-v` | 矩形選択トグル |
| `y` | 選択コピー |
| `Y` | 行コピー |

### 2.2 claude_table（二段プレフィックス: `C-b C, <key>`）

`C-b C` を押すと status-left に `⟨claude_table⟩` が表示され、次の 1 キーで以下が走る:

| キー | 動作 |
|---|---|
| `c` | **現 pane** で `claude --continue` を実行 |
| `n` | git branch を fzf で選び **新規 worktree + 2-pane Claude session** を作成 |
| `r` | 現 session 内の claude pane を kill → `--continue` で再起動 |
| `s` | `claude-*` 名の session を fzf で選び switch |
| `k` | 現 session が `claude-*` なら **session + worktree を一括削除**（`y/n` で確認） |

> `prefix + C` を押した後に何もしないと自動で table から抜ける。途中でやめたい場合は `Esc` か `C-c`。

---

## 3. 典型フロー

### 3.1 新しいタスクを並行で始める（最頻出）

```text
C-b C, n
  → fzf で branch を選択
  → 自動的に <repo>-<branch-safe> に worktree が切られる
  → 左 pane = shell / 右 pane = claude --continue --fork-session の 2-pane session が起動
```

### 3.2 既存セッションに戻る

```text
C-b C, s   # claude-* セッション専用ピッカー
C-b s      # 全セッションピッカー（claude 以外も含む）
```

### 3.3 Claude の挙動がおかしくなったら

```text
C-b C, r   # 現 session 内の claude pane を kill して --continue で再起動
           # 履歴は維持されるので作業文脈は失われない
```

### 3.4 作業終了・片付け

```text
C-b C, k   # 確認 (y/n) のあと session + worktree を一括削除
```

worktree のみ手動で消したい場合:

```bash
git worktree remove ../<branch-safe>
```

### 3.5 シェルから直接 worktree+claude を立てる

```bash
tmux_claude_new feature/x
# = ~/.config/tmux/scripts/tmux-claude-new.sh feature/x
```

---

## 4. ステータスバーの読み方

設定: `dot_config/tmux/conf/status.conf`（5 秒間隔で更新）

### 左

```
[<session>:<window>:<pane>]  ⌘  ⟨claude_table⟩
                              │   └─ key-table 進入中（C-b C 直後）
                              └──── prefix 入力受付中
```

### 右

```
[claude:N]  [branch]  HH:MM
   │           └─ 現 pane の git branch
   └─ 走っている claude プロセス数（pgrep -c -x claude）
```

### pane 枠

`pane_current_command == claude` の pane は **黄色枠**でハイライトされる（`dot_config/tmux/conf/claude.conf`）。

---

## 5. 通知 / サウンド

設定: `dot_config/claude/settings.json` の `hooks.*` + `dot_local/bin/executable_claude-notify-sound.sh`。

| トリガー | 音 | popup タイトル | urgency |
|---|---|---|---|
| `Notification`（入力待ち） | `message.oga` | Claude Code | normal |
| `Stop`（ターン完了） | `complete.oga` | Claude Code | normal |
| `subagent-stop` | `bell.oga` | Claude Code | low |
| `error` | `dialog-error.oga` | Claude Code | critical |

ポイント:

- `notify-send --expire-time=0` なので **クリックするまで popup が消えない**
- 通知 daemon は **wired-notify**（`dot_config/wired/wired.ron` + `systemd --user` で常駐）
- 音は `pw-play` → `paplay` → `ffplay` の順でフォールバック再生

---

## 6. trouble-shoot 早見表

| 症状 | 確認場所 |
|---|---|
| `C-b` が効かない | Ghostty の `keybind = clear` で `C-b` を消費していないか / Ghostty 併用時のみ `focus-events on` (`options.conf`) との相互作用で 1 回目が落ちる場合あり (ghostty#9620)、その端末では一時的に `tmux set -g focus-events off` |
| `C-b C, n` で popup が出ない | `~/.config/tmux/scripts/claude-pick-branch.sh` の実行ビット / `fzf` がインストール済みか |
| `[claude:N]` が常に 0 | `pgrep -c -x claude` を直接叩いて実プロセスがあるか確認 |
| 通知音が鳴らない | `pw-play /usr/share/sounds/freedesktop/stereo/complete.oga` を直接実行 / wired-notify の起動状態 (`systemctl --user status wired`) |
| popup がすぐ消える | wired-notify が `--expire-time=0` を尊重しているか / 別の通知 daemon (dunst 等) が動いていないか |
| TPM プラグインが入らない | `~/.config/tmux/scripts/tpm-bootstrap.sh` を手動実行 |

---

## 7. ファイル位置クイックリンク

| ファイル | 役割 |
|---|---|
| `dot_config/tmux/tmux.conf` | tmux エントリポイント |
| `dot_config/tmux/conf/options.conf` | prefix / focus-events / 各種オプション |
| `dot_config/tmux/conf/bindings.conf` | 直下バインド + claude_table |
| `dot_config/tmux/conf/status.conf` | status-bar フォーマット |
| `dot_config/tmux/conf/claude.conf` | pane-border ハイライト |
| `dot_config/tmux/conf/plugins.conf` | TPM プラグイン宣言 |
| `dot_config/tmux/scripts/` | claude 関連ヘルパースクリプト群 |
| `dot_config/zsh/rc/my_plugins/tmux.zsh` | `tmux_claude_new` 関数 |
| `dot_config/claude/settings.json` | Claude Code フック登録 |
| `dot_local/bin/executable_claude-notify-sound.sh` | 通知音 + popup 統合フック |

---

## 8. 関連ドキュメント

- 実装詳細・設計判断: [`docs/manage_claude.md`](./manage_claude.md)
- キーバインド全体（SKK / tmux / zsh の重複検査含む）: [`docs/keybinds.md`](./keybinds.md)
- 公式: <https://code.claude.com/docs/en/common-workflows>
