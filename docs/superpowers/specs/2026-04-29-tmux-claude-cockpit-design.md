# tmux Claude Cockpit — Design Spec

- **Date**: 2026-04-29
- **Status**: Approved (brainstorming complete, ready for implementation plan)
- **Owner**: kiyama
- **Repo**: chezmoi (`/home/kiyama/.local/share/chezmoi`)
- **Related docs**: [`docs/manage_claude.md`](../../manage_claude.md), [`CLAUDE.md`](../../../CLAUDE.md)

---

## 1. Goal

tmux を Claude Code 専用の作業コックピットとして再構築する。具体的には:

- **A. 並行性**: worktree ごとに独立した tmux セッションを 1 コマンドで立ち上げ・往来する
- **B. 可視性**: 各ペインで Claude が動いているかを視覚的に把握できる
- **C. 永続性**: tmux 再起動後にレイアウトと作業 PWD が自動復元する（Claude 履歴は `--resume` 側に任せる）
- **D. 操作性**: Claude 関連操作を二段プレフィックス（`prefix + c …`）配下に集約し、既存バインドと衝突させない

## 2. Non-Goals

- Claude プロセス自体の永続化や stdout キャプチャ（resurrect の保存対象外。`claude --resume` で十分）
- tmuxinator / zellij など外部オーケストレータへの乗り換え
- 既存の `tmux` / `tmux_claude` 関数の挙動変更（後方互換維持）
- 統合 GUI ダッシュボード（status-bar 範囲で完結）

## 3. Scope

### 3.1 セッション粒度（ハイブリッド方針）
- `main` セッション: 普段使い（既存維持、`tmux` 関数で起動）
- `claude` セッション: 単一 Claude セッション（既存維持、`tmux_claude` 関数で起動）
- `claude-<branch>` セッション: worktree 紐付き Claude セッション（新規 `tmux_claude_new <branch>` で量産）

### 3.2 プラグイン管理
TPM (`tmux-plugins/tpm`) を採用。chezmoi の `run_once_all_os.sh.cmd.tmpl` で TPM を自動 clone、初回のみ `install_plugins` を呼ぶ。導入プラグイン:

| プラグイン | 用途 |
|---|---|
| `tmux-plugins/tmux-resurrect` | レイアウト・PWD・コマンドラインの保存と復元 |
| `tmux-plugins/tmux-continuum` | resurrect の自動保存（15 分間隔）と起動時自動復元 |
| `tmux-plugins/tmux-yank` | コピーモード → システムクリップボード連携 |

## 4. Architecture

### 4.1 ファイルレイアウト
```
dot_config/tmux/
├── tmux.conf                    # ルート設定: source 宣言中心に簡素化
├── conf/
│   ├── options.conf             # set -g 系オプション（mouse, escape-time, base-index 等）
│   ├── bindings.conf            # bind 系（既存 vim 風移動 + 新 c-prefix 群）
│   ├── plugins.conf             # TPM 宣言とプラグイン options
│   ├── status.conf              # status-left / status-right
│   └── claude.conf              # Claude 専用 hooks（pane-border-format 動的色 + key-table）
└── scripts/
    ├── claude-status-count.sh   # `pgrep -c claude` 結果を status-right 用に整形
    ├── claude-branch.sh         # status-right 用: `git -C <path> branch --show-current`
    └── tpm-bootstrap.sh         # TPM clone + install_plugins（chezmoi run_once から呼ぶ）

dot_config/zsh/rc/my_plugins/tmux.zsh
└── (改修) tmux_claude_new <branch> を追加。既存 tmux / tmux_claude は維持

.chezmoiscripts/run_once_all_os.sh.cmd.tmpl
└── (改修) Manjaro ブロックに TPM bootstrap 呼び出し追加

.chezmoiignore
└── (改修) ~/.local/share/tmux/resurrect/ を ignore（生成物のため管理対象外）
```

### 4.2 設定モジュール化
`tmux.conf` は宣言のみ:
```tmux
source-file ~/.config/tmux/conf/options.conf
source-file ~/.config/tmux/conf/bindings.conf
source-file ~/.config/tmux/conf/status.conf
source-file ~/.config/tmux/conf/claude.conf
source-file ~/.config/tmux/conf/plugins.conf  # 末尾必須（TPM 仕様）
run '~/.tmux/plugins/tpm/tpm'
```

理由:
- 800 行規約に収めやすい（現状 80 行 → 拡張時に肥大化を防ぐ）
- 領域別に責務分離（status, claude hooks, bindings）
- ロード順を明示（`plugins.conf` 末尾は TPM の仕様要件）

## 5. Components

### 5.1 `conf/options.conf`
既存の `set -g` 系を全て移管。新規: `pane-border-status top` は維持、`set -g focus-events on`（pane border の動的更新に必要）を追加。

### 5.2 `conf/bindings.conf`
既存バインドを全て移管。直下追加分:

| バインド | 動作 | 実装 |
|---|---|---|
| `prefix + s` | fzf-tmux でセッション選択（既存 choose-tree を置換） | `bind s display-popup -E "tmux list-sessions -F '#S' \| fzf \| xargs tmux switch-client -t"` |
| `prefix + g` | fzf で worktree 選択 → `switch-client` | `display-popup -E` でピッカー起動 |

Claude 専用バインドは **key-table `claude_table` 配下に集約**（`prefix + C` で table 切替 → 後続 1 打鍵）:

```tmux
bind C switch-client -T claude_table

bind -T claude_table c send-keys "claude --continue" Enter
bind -T claude_table n display-popup -E "<branch-picker-script>"
bind -T claude_table r run-shell "<respawn-claude-pane-script>"
bind -T claude_table s display-popup -E "<claude-session-picker-script>"
bind -T claude_table k confirm-before -p "kill session and worktree? (y/n)" "<kill-script>"
```

| バインド列 | 動作 |
|---|---|
| `prefix + C, c` | 現在ペインに `claude --continue` を流す |
| `prefix + C, n` | fzf でブランチ選択 → `tmux_claude_new` 起動 |
| `prefix + C, r` | アクティブ claude ペインを kill → 再起動 |
| `prefix + C, s` | fzf でセッション一覧（`claude-*` のみ） |
| `prefix + C, k` | 現セッション終了 + worktree 削除（確認あり） |

key-table を使う理由:
- tmux の正式機構で「二段プレフィックス」を表現できる
- 既存 `prefix + c`（new-window）と衝突しない（大文字 `C` を使用）
- `claude_table` 内では table が一度しか使えないので、誤爆を防げる

### 5.3 `conf/plugins.conf`
```tmux
set -g @plugin 'tmux-plugins/tpm'
set -g @plugin 'tmux-plugins/tmux-resurrect'
set -g @plugin 'tmux-plugins/tmux-continuum'
set -g @plugin 'tmux-plugins/tmux-yank'

set -g @continuum-restore 'on'
set -g @continuum-save-interval '15'
set -g @resurrect-capture-pane-contents 'on'
set -g @resurrect-strategy-nvim 'session'
```

### 5.4 `conf/status.conf`
- **Status left**: `[#S:#I:#P]` + `#{?client_prefix,⌘ ,}`（プレフィックス入力中インジケータ）
- **Status right**: `#(~/.config/tmux/scripts/claude-status-count.sh) #(~/.config/tmux/scripts/claude-branch.sh #{pane_current_path}) %H:%M`
- **Status interval**: 5 秒（claude プロセス監視のため）

### 5.5 `conf/claude.conf`
**Pane border 動的色** + **key-table 切替バインド** を集約:
```tmux
# pane border の動的色（claude 実行中ペインを黄色枠で強調）
set -g pane-border-format "#[fg=#{?#{==:#{pane_current_command},claude},yellow,colour244}] #{pane_title} [#{pane_current_command}]"

# key-table 切替（実装は bindings.conf 側で完結。ここでは hook と style のみ）
```

注: `pane-border-format` を `claude.conf` に置く理由は、Claude 関連ロジック一式を 1 ファイルに局所化するため（status.conf には書かない）。

### 5.6 `scripts/claude-status-count.sh`
```sh
#!/usr/bin/env bash
n=$(pgrep -c claude || echo 0)
[ "$n" -gt 0 ] && printf "[claude:%d] " "$n"
```

### 5.7 `scripts/claude-branch.sh`
```sh
#!/usr/bin/env bash
# usage: claude-branch.sh <pane-current-path>
b=$(git -C "$1" branch --show-current 2>/dev/null) || exit 0
[ -n "$b" ] && printf "[%s] " "$b"
```

### 5.8 `dot_config/zsh/rc/my_plugins/tmux.zsh` 改修
新規関数 `tmux_claude_new`:

```sh
tmux_claude_new() {
  local branch="$1"
  [ -z "$branch" ] && { echo "usage: tmux_claude_new <branch>"; return 1; }

  # session 名正規化: feature/x → claude-feature-x
  local session_name="claude-${branch//\//-}"
  local repo_root worktree_path
  repo_root=$(git rev-parse --show-toplevel) || return 1
  worktree_path="${repo_root}-${branch//\//-}"

  # worktree が無ければ作成
  if [ ! -d "$worktree_path" ]; then
    git worktree add "$worktree_path" "$branch" || return 1
  fi

  # session 作成（既存ならスキップ）
  if ! tmux has-session -t "$session_name" 2>/dev/null; then
    tmux new-session -d -s "$session_name" -c "$worktree_path"
    tmux split-window -h -t "$session_name" -c "$worktree_path"
    tmux select-pane -t "${session_name}.0" -T work
    tmux select-pane -t "${session_name}.1" -T claude
    tmux send-keys -t "${session_name}.1" "claude --continue --fork-session" Enter
  fi

  # attach or switch
  if [ -n "$TMUX" ]; then
    tmux switch-client -t "$session_name"
  else
    tmux attach-session -t "$session_name"
  fi
}
```

既存 `tmux` / `tmux_claude` 関数は変更しない。

### 5.9 `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` 改修
Manjaro ブロックの末尾に追加:
```sh
# TPM bootstrap
if [ ! -d "$HOME/.tmux/plugins/tpm" ]; then
  git clone --depth 1 https://github.com/tmux-plugins/tpm "$HOME/.tmux/plugins/tpm"
fi
"$HOME/.tmux/plugins/tpm/bin/install_plugins"
```

## 6. Data Flow

### 6.1 `tmux_claude_new feature/x` 実行時
1. zsh 関数が `feature/x` を受領
2. `git rev-parse --show-toplevel` でリポジトリルート取得
3. session 名 `claude-feature-x` に正規化
4. worktree path `<repo>-feature-x` を組み立て、未存在なら `git worktree add`
5. session 未存在なら 2 ペイン構成で作成、右ペインで `claude --continue --fork-session` 起動
6. TMUX 環境内なら `switch-client`、外なら `attach-session`

### 6.2 起動時の永続化復元
1. tmux サーバ起動 → `tpm` ロード → `tmux-continuum` の `restore-on-startup` ロジック発火
2. `~/.local/share/tmux/resurrect/last` を読み、レイアウト・PWD・コマンドラインを復元
3. Claude プロセスは復元されない（pane の PWD と直近コマンドは復元されるので、ユーザが `claude --resume` を打ち直すだけ）

### 6.3 ペイン状態の可視化
1. `status-interval 5` で 5 秒ごとに status-bar が再評価
2. `pane-border-format` の三項演算が各ペインで評価される
3. `pane_current_command == claude` のペインに黄色ボーダーが付く
4. `claude-status-count.sh` で実行中 Claude プロセス数を status-right に表示

## 7. Error Handling

| 失敗ケース | 挙動 |
|---|---|
| `tmux_claude_new` に branch 未指定 | usage 表示 + return 1 |
| `git rev-parse` 失敗（非 git ディレクトリ） | エラーメッセージ + return 1 |
| `git worktree add` 失敗（branch 未存在等） | git のエラー出力をそのまま見せて return 1 |
| TPM 未 clone 状態で tmux 起動 | プラグイン無しで通常起動。`run_once` が次回ログイン時に再試行 |
| `claude` バイナリ未インストール状態で `prefix + c c` | `command not found` がペインに表示される（標準 zsh 挙動） |
| resurrect 保存ファイル破損 | continuum が失敗ログを残し通常起動 |

## 8. Bootstrap & Migration

### 8.1 既存環境への適用フロー
1. このリポジトリで spec 通りに `dot_config/tmux/` 配下を更新
2. `chezmoi diff` で差分確認
3. `chezmoi apply`
4. 既存 tmux サーバを `tmux kill-server` で再起動（resurrect 自動保存はまだ無いので状態は失われる、手動でやり直し）
5. `~/.tmux/plugins/tpm` が `run_once` で clone 済みなら自動でプラグイン展開
6. もし `run_once` が走らない（既存マシン）場合は `~/.config/tmux/scripts/tpm-bootstrap.sh` を直接実行

### 8.2 ロールバック
- spec 適用前に `git tag tmux-pre-cockpit` を打っておけば即時 revert 可能
- TPM プラグインは `~/.tmux/plugins/` 以下にあり chezmoi 管理外なので、不要なら `rm -rf ~/.tmux/plugins/{resurrect,continuum,yank}` で除去

## 9. Testing / Verification

### 9.1 受け入れ基準
- [ ] `chezmoi apply` 後、`tmux kill-server && tmux` で問題なく起動する
- [ ] `tmux source ~/.config/tmux/tmux.conf` がエラー無し
- [ ] `prefix + I` で TPM がプラグインを取得できる（または `run_once` で自動取得済み）
- [ ] `tmux_claude_new test-branch` が新セッションを作り claude を立ち上げる
- [ ] worktree が `git worktree list` に出る
- [ ] claude 実行中ペインの border が黄色になる
- [ ] status-right に `[claude:1]` が表示される
- [ ] `tmux kill-server` 後の再起動でレイアウトが復元される
- [ ] `prefix + c s` で fzf-tmux ピッカーが出る
- [ ] 既存 `tmux` / `tmux_claude` 関数の挙動が変わっていない

### 9.2 手動検証手順
```sh
# 1. 既存セッションを破棄して clean start
tmux kill-server
tmux                        # main セッション、3 分割で起動するはず

# 2. claude 用テスト
git worktree list           # 現状確認
tmux_claude_new feat/test   # → セッション claude-feat-test 起動、worktree 作成
# 右ペインで claude --continue が走り、border が黄色

# 3. 永続化テスト
# 何か作業した状態で
tmux kill-server
tmux                        # → レイアウトが復元されるか確認

# 4. 後片付け
tmux kill-session -t claude-feat-test
git worktree remove <repo>-feat-test
```

## 10. Open Questions

- [ ] fzf 未インストールの場合のフォールバック挙動（Manjaro なら paru で同梱される想定。WSL/Windows 側は別途検討）
- [ ] `tmux-yank` のクリップボード backend 自動検出が Wayland/X11 環境で正しく動くか（実装後の検証項目）

## 11. References

- TPM: <https://github.com/tmux-plugins/tpm>
- tmux-resurrect: <https://github.com/tmux-plugins/tmux-resurrect>
- tmux-continuum: <https://github.com/tmux-plugins/tmux-continuum>
- tmux-yank: <https://github.com/tmux-plugins/tmux-yank>
- 関連: [`docs/manage_claude.md`](../../manage_claude.md)
