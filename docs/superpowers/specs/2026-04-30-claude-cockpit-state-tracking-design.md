# Claude Cockpit — State Tracking & Hierarchical Switcher (Design Spec)

- **Date**: 2026-04-30
- **Status**: Approved (brainstorming complete, ready for implementation plan)
- **Owner**: kiyama
- **Repo**: chezmoi (`/home/kiyama/.local/share/chezmoi`)
- **Extends**: [`2026-04-29-tmux-claude-cockpit-design.md`](./2026-04-29-tmux-claude-cockpit-design.md)
- **Inspired by**: [`samleeney/tmux-agent-status`](https://github.com/samleeney/tmux-agent-status) (MIT) — concepts only, no code copied or vendored

---

## 1. Goal

Cockpit v1（4/29 スペック）が完成し worktree 量産フローは安定したが、**「どの Claude が今プロンプト待ちか」が一目で分からない** 問題が残った。本スペックはそれを解決する追加機能を定義する。

具体的に実現すること:

- **G1 観測精度**: `pgrep -x claude` ベースの「起動有無」検出を、Claude hooks が書き出す **`working / waiting / done`** の三値ステートに置き換える
- **G2 集約サマリ**: status-right に `⚡ 3 ⏸ 1 ✓ 2` を出す（`[claude:N]` を廃止）
- **G3 階層スイッチャ**: `prefix + C` → `s` で session → window → pane を 1 popup で扱える fzf スイッチャを提供
- **G4 Next-ready ジャンプ**: `prefix + C` → `N` で **応答待ち**（`waiting`）→ **完了済み**（`done`）の優先順で pane に inbox 順循環移動。各バケット内は `session 名昇順 → window-index 昇順 → pane-index 昇順`。`waiting` を優先するのは、Claude が許可・入力を求めている pane の方が緊急度が高いため

## 2. Non-Goals

- `tmux-agent-status` プラグイン本体の導入・vendor・fork（**しない**）
- Codex CLI 連携（こちらは Codex を使っていない）
- 常駐サイドバー pane（pane を奪うため、現行レイアウトと衝突）
- 上流の `deploy-sessions.sh` manifest ベース一括起動（既存 `tmux-claude-new.sh` が ad-hoc 用途には十分）
- Park / Wait モード（YAGNI、需要が出たら別 spec）
- SSH リモート tmux 監視
- Claude プロセスの永続化（4/29 spec の Non-Goal を継承）

## 3. Inspiration vs Independence

`tmux-agent-status` のアーキテクチャ上の **アイデアのみ** を取り込む。コードのコピー・vendoring・fork は行わない。

| 上流から取り込むアイデア | 取り込み方法 |
|---|---|
| Claude hooks による状態追跡 | こちらで hook script をゼロから書く |
| `~/.cache/<plugin>/{sessions,panes}/` のキャッシュレイアウト | パス先頭だけ `claude-cockpit/` に変えて **構造は同一** に保つ（将来上流へ乗り換える退路として） |
| 三値ステート（`working / waiting / done`） | 文字列リテラルとして同一値を採用 |
| `status-right` への集約サマリ表示 | こちらで集計スクリプトを書く |
| 階層 fzf スイッチャ（session → window → pane） | こちらで fzf プロンプトをゼロから書く（折り畳みは実装せず全展開） |
| Next-ready ジャンプ（inbox order） | こちらで実装 |

**取り込まないアイデア**: 常駐 daemon、サイドバー pane、Park / Wait、Codex hook、SSH 連携、manifest deploy。

## 4. Scope

### 4.1 状態モデル

| 状態 | 意味 | 表示記号 | 遷移トリガー |
|---|---|---|---|
| `working` | Claude が tool/モデル実行中 | `⚡` | `UserPromptSubmit` / `PreToolUse` |
| `waiting` | Claude がユーザー入力待ち | `⏸` | `Notification` |
| `done` | ターン完了 | `✓` | `Stop` |
| 未定義 | hook が未到達、または Claude 未起動 | （集計対象外） | — |

`SubagentStop` は無視（親 Claude の `working` を維持。subagent の終了通知音は既存 hook が引き続き担当する）。

### 4.2 状態の永続化レイアウト

```
$XDG_CACHE_HOME/claude-cockpit/        ($XDG_CACHE_HOME 未設定なら $HOME/.cache)
├── panes/
│   └── <tmux-session>_<pane-id>.status     # 真のソース。中身は "working" | "waiting" | "done" の 1 行
└── sessions/
    └── <tmux-session>.status                # panes/ から導出。最重要度 (working > waiting > done) を集約
```

- pane id は tmux の `%12` 形式（`%` を含むがファイル名として OK）
- session 集約ファイルは **status-right 用キャッシュ**ではなく、panes/ を都度集計するなら不要。**初版では panes/ のみ実装する**。`sessions/` ディレクトリは作成のみ行い、本 spec のコードは書き込まない（上流互換のためにレイアウトは予約。`prune.sh` / `kill-session` 側は防御的に sessions/ も掃除する）
- 書き込みは atomic: `printf '%s' working > $tmp && mv $tmp $final`

### 4.3 ファイル構成（chezmoi 内）

```
dot_config/tmux/scripts/cockpit/                ← 新設サブディレクトリ
├── executable_summary.sh                        status-right が呼ぶ集計スクリプト
├── executable_switcher.sh                       claude_table.s が呼ぶ階層 fzf
├── executable_next-ready.sh                     claude_table.N が呼ぶ
└── executable_prune.sh                          orphan 状態ファイルの掃除

dot_local/bin/
└── executable_claude-cockpit-state.sh           hook entry: 状態書き出し専用（責務分離）

dot_config/claude/settings.json (.tmpl)         ← UserPromptSubmit / PreToolUse hook 追加
dot_config/tmux/conf/status.conf                ← summary.sh 呼び出しに差替え
dot_config/tmux/conf/bindings.conf              ← claude_table.s を switcher.sh に差替え + claude_table.N 新設

DELETE:
dot_config/tmux/scripts/executable_claude-status-count.sh   pgrep ベース廃止
dot_config/tmux/scripts/executable_claude-pick-session.sh   switcher が機能上位互換
```

`claude-notify-hook.sh` / `claude-notify-dispatch.sh` / `claude-branch.sh` / `claude.conf`（pane border 動的色）/ `tmux-claude-new.sh` / `claude-pick-branch.sh` / `claude-respawn-pane.sh` / `claude-kill-session.sh` は **改変なし**（kill-session のみ §6.4 で cache 削除を 1 行追加）。

## 5. Architecture

### 5.1 データフロー

```
Claude (1 session = 1 pane を想定)
  │ UserPromptSubmit / PreToolUse / Stop / Notification → atomic write
  │ SessionEnd                                          → rm -f
  ▼
~/.local/bin/claude-cockpit-state.sh hook <event>
  │   read $TMUX_PANE → resolve session via tmux display-message
  │   atomic write or rm -f
  ▼
$XDG_CACHE_HOME/claude-cockpit/panes/<session>_<pane>.status
  │ on-demand read by (filter: pane_current_command == claude):
  ▼
- conf/status.conf  → cockpit/summary.sh         (5s polling, status-interval)
- prefix+C → s      → cockpit/switcher.sh        (fzf popup)
- prefix+C → N      → cockpit/next-ready.sh      (one-shot jump)

orphan cleanup:
  tmux server 起動時 / claude_table.s 起動冒頭 / Ctrl-R 明示再描画 → cockpit/prune.sh
  (cleanup criterion: pane が消えた OR pane_current_command != claude)
```

**daemon を持たない**。状態は hook の I/O のみで更新され、読み手が都度集計する。pane 数 ≤ 数十のスケールで十分。

**Cleanup 三層**:
1. **graceful**: Claude の `SessionEnd` hook (`/exit` / `/clear` / `/logout`) で当該 status file を即時 `rm -f`
2. **defensive (read-time)**: reader (summary / next-ready / switcher) が `pane_current_command != claude` の pane に対応する file を集計から除外。SessionEnd が発火しない経路（SIGKILL / OOM / pane closed without /exit）でも幽霊カウントを回避
3. **prune (sweep)**: `prune.sh` が tmux 起動時 / switcher 起動時に live claude pane に対応しない status file を削除

`docs/todos.md` F-7 で eBPF ベースのリアルタイム検出を follow-up として記録（kernel `sched_process_exit` トレースで `comm == "claude"` の終了を即時捕捉する案）。

### 5.2 コンポーネント責務

| コンポーネント | 責務 | 入力 | 出力 |
|---|---|---|---|
| `claude-cockpit-state.sh hook <event>` | 状態書き出し | `$TMUX_PANE`, hook payload (stdin) | `panes/<S>_<P>.status` 更新 |
| `cockpit/summary.sh` | status-right 用集計文字列を生成 | `panes/*.status` | 例: `⚡ 3 ⏸ 1 ✓ 2 ` （末尾スペース 1） |
| `cockpit/switcher.sh` | 階層 fzf スイッチャ起動 | tmux server, `panes/*.status` | tmux switch-client / kill 系コマンド実行 |
| `cockpit/next-ready.sh` | `waiting` を優先しつつ `done` も含めた **要対応 pane** に循環移動 | tmux server, `panes/*.status` | tmux switch-client + select-pane |
| `cockpit/prune.sh` | tmux に存在しない pane の状態ファイルを削除 | tmux server, `panes/*.status` | `rm` のみ（副作用 only） |

### 5.3 上流互換性の扱い

- キャッシュディレクトリ名だけ `tmux-agent-status` → `claude-cockpit` に変える
- 内部レイアウト（`panes/<session>_<pane>.status`、文字列値）は同一
- 将来上流プラグインを導入したくなった場合、こちら側 hook を無効化＋シンボリックリンク `claude-cockpit/panes` → `tmux-agent-status/panes` で **状態を継承可能**（spec レベルでの確約は不要、退路としての記述）

## 6. Behavior Detail

### 6.1 状態遷移

```
        ┌─────── UserPromptSubmit / PreToolUse ───────┐
        ▼                                             │
   [working] ───────────── Stop ──────────► [done]    │
        │                                     │       │
        │                                     │       │
        └── Notification ─► [waiting] ─── UserPromptSubmit ─┘
```

実装は先行状態を見ない冪等なマップ:
- `UserPromptSubmit` → `working`
- `PreToolUse`        → `working`
- `Stop`              → `done`
- `Notification`      → `waiting`

### 6.2 tmux コンテキスト解決

```sh
tmux_pane="${TMUX_PANE:-}"
[ -z "$tmux_pane" ] && exit 0   # tmux 外の Claude → 何もしない（hook は成功扱い）

tmux_session=$(tmux display-message -p -t "$tmux_pane" '#{session_name}' 2>/dev/null) || exit 0
[ -z "$tmux_session" ] && exit 0

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
mkdir -p "$cache_dir" 2>/dev/null || exit 0

# atomic write
file="$cache_dir/${tmux_session}_${tmux_pane}.status"
tmp="$file.$$.tmp"
printf '%s' "$state" > "$tmp" && mv "$tmp" "$file"
```

### 6.3 階層スイッチャ（`prefix + C` → `s`）

**表示形式**（全展開、折り畳みなし）:
```
session-A                              ⚡ working
  window:0 work
    pane:%12  cwd=~/repo               ⚡ working
    pane:%13  cwd=~/repo               ✓ done
  window:1 docs
    pane:%14  cwd=~/repo-feat          ⏸ waiting
session-B                              ✓ done
  window:0
    pane:%20                           ✓ done
```

**行種別**: 各行は tab 区切りのメタ列を持ち、表示時は cut する:
```
<kind>\t<session>\t<window-idx>\t<pane-id>\t<display-text>
kind ∈ { S, W, P }    # session / window / pane
```

**キーバインド**:
- `Enter`: kind に応じて
  - S: `tmux switch-client -t <session>`
  - W: `tmux switch-client -t <session> \; select-window -t <window-idx>`
  - P: `tmux switch-client -t <session> \; select-window -t <window-idx> \; select-pane -t <pane-id>`
- `Ctrl-X`: kind に応じて
  - P: `tmux kill-pane -t <pane-id>`（即時、確認なし）
  - W: `tmux confirm-before -p "kill window? "  "kill-window -t ..."`
  - S: 既存 `claude-kill-session.sh` を流用（worktree も削除、確認込み）
- `Ctrl-R`: `cockpit/prune.sh` を実行して再描画

**フィルタ**: 全 tmux session を対象（claude-* に絞らない）。状態ファイルがない pane は表示するが状態列は空欄。

### 6.4 Next-ready（`prefix + C` → `N`）

巡回範囲: **全 tmux session 横断**（claude-* に限定しない。状態ファイルが `waiting` または `done` のものすべてが対象）。

優先順位は二段階:

1. **`waiting`**（Notification 発火 = Claude が許可・入力を求めている、最も対応緊急度が高い）
2. **`done`**（Stop 発火 = Claude のターンが終わった、確認可能）

各バケット内の inbox order は `session 名昇順 → window-index 昇順 → pane-index 昇順`。最終的な巡回リストは `<全 waiting 行> ++ <全 done 行>` で、`waiting` の pane が `done` の pane より常に先に来る。

現在 pane の **巡回リスト上の次の行**（末尾なら先頭に循環）にジャンプ。`waiting` も `done` も 0 件の場合は `tmux display-message "no ready claude pane"` で 1 秒間メッセージ表示。

### 6.5 status-right 集約

`cockpit/summary.sh` は以下を出力:
- working > 0 のみ: `⚡ N `
- 複数の状態が混在: `⚡ N ⏸ M ✓ K `（0 件項目はサプレス）
- すべて未定義 / 0: 空文字列

`status.conf` 側:
```tmux
set -g status-right "#(~/.config/tmux/scripts/cockpit/summary.sh)#(~/.config/tmux/scripts/claude-branch.sh '#{pane_current_path}')%H:%M "
```

`claude-status-count.sh` の呼び出しを削除し、`summary.sh` で置換するのみ。

### 6.6 既存 kill-session への状態キャッシュ追記

`dot_config/tmux/scripts/executable_claude-kill-session.sh` 末尾に:
```sh
cache="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit"
rm -f "$cache/sessions/${session}.status" "$cache/panes/${session}"_*.status 2>/dev/null || true
```

これは新規ファイル追加ではなく、**既存スクリプトへの 2 行追加**。

### 6.7 prune.sh

```
1. tmux list-panes -a -F '#{session_name}_#{pane_id}' を集合 A とする
2. ls $cache/panes/*.status の basename から ".status" を除いたものを集合 B とする
3. B - A の各要素について rm -f $cache/panes/<x>.status
```

呼び出しタイミング:
- `claude_table.s`（switcher 起動）冒頭で 1 回
- `Ctrl-R` で明示再起動
- tmux server 起動時の `run-shell ~/.config/tmux/scripts/cockpit/prune.sh`（`tmux.conf` に追加）

## 7. Key Bindings 変更点

### 7.1 設計方針: claude 系は `prefix + C` 配下に集約

tmux + Claude 関連の操作は **すべて `prefix + C` で入る claude_table の配下に集約** する。トップレベルに `prefix + S` / `prefix + N` のような Claude 専用バインドを足さない。理由:

- 既存の claude_table（`prefix + C` で進入）が **Claude 操作の唯一のエントリ** という建前を崩さない（4/29 spec の設計原則を継承）
- トップレベル keymap が肥大化して既存バインドと衝突するのを避ける
- 「Claude 関連 → まず `prefix + C`」という筋肉記憶 1 本にまとめる

### 7.2 claude_table（`prefix + C` 進入後）

| キー | 旧挙動 | 新挙動 |
|---|---|---|
| `c` | `claude --continue` を現 pane で送る | **不変** |
| `n` | branch picker → `tmux-claude-new` | **不変** |
| `r` | claude ペインを respawn | **不変** |
| `s` | `claude-pick-session.sh`（claude-* フラット fzf） | **`cockpit/switcher.sh` に差替え**（階層 fzf、全 session 対象） |
| `k` | session + worktree kill（confirm 付き） | **不変**（§6.6 で cache 削除 2 行を追加するのみ） |
| `N` | （未定義） | **新設**: `cockpit/next-ready.sh`（`waiting` 優先 → `done` の順で要対応 pane を inbox 順循環ジャンプ） |

### 7.3 トップレベル keymap（`prefix + ...`）への影響

| バインド | 旧挙動 | 新挙動 |
|---|---|---|
| `prefix + s` | 全 session フラット fzf | **不変** |
| `prefix + S` | （未定義） | **設定しない**（claude_table.s に格納） |
| `prefix + N` | （未定義） | **設定しない**（claude_table.N に格納） |

### 7.4 上流 (`tmux-agent-status`) との対応関係

参考までに、上流のデフォルトバインドとこちらの対応:

| 上流 | こちら |
|---|---|
| `prefix + S` | `prefix + C` → `s` |
| `prefix + N` | `prefix + C` → `N` |
| `prefix + W` (wait) | 未実装（Non-Goal） |
| `prefix + p` (park) | 未実装（Non-Goal） |
| `prefix + o` (sidebar) | 未実装（Non-Goal） |

## 8. Error Handling

| ケース | 対応 |
|---|---|
| `mkdir -p` 失敗 | hook は stderr に `logger -t claude-cockpit-state` で 1 行だけ書き、`exit 0`。Claude を絶対に止めない |
| 同時書き込み race | 各 hook は **自 pane のファイルにだけ** 書き、atomic move。複数 Claude が同 pane で動くことはない |
| Claude が SIGKILL → Stop hook 不発 | 状態が `working` のまま残る。`prune.sh` 後の次の switcher 起動または kill-session 時に消える。**許容**（誤検知より誤陰性のほうが安全） |
| tmux 外で起動した Claude | hook 内で `$TMUX_PANE` 未定義 → 即 `exit 0` |
| status-right 5s 周期中に hook 書き込み | atomic move なので部分書き込みは発生しない |
| switcher の中で session が消滅 | tmux 側のコマンドが no-op で失敗 → `display-message` でエラーを 1 行出すのみ |
| キャッシュディレクトリの権限破壊 | hook と reader 双方 `2>/dev/null \|\| true` で握りつぶし、status は無印に |

## 9. Testing Strategy

ユニットテストは **書かない**。総コード量は ~200 行未満で、シェル glue としての複雑度は低い。代わりに以下の **手動スモークテスト** を `docs/manage_claude.md` に追記する:

1. 既存 `tmux-claude-new.sh` で `claude-foo` / `claude-bar` を 2 つ作成
2. `claude-foo` で「`Hello`」と送信 → status-right が `⚡ 1 ` になることを確認
3. Claude が応答完了し ESC で待機 → status-right が `✓ 1 ` になる
4. `claude-bar` でも同様にプロンプト送信 → `⚡ 1 ⏸ 1` のように混在することを確認（条件次第）
5. `prefix + C` → `N` で要対応 pane（`waiting` がいれば `waiting` を優先し、無ければ `done`）にジャンプできる
6. `prefix + C` → `s` で階層スイッチャを開き、`Ctrl-X` で空 pane を kill できる
7. `claude-foo` を `claude_table.k` で kill-session → cache の `panes/claude-foo_*.status` が消える
8. tmux を kill-server → 再起動後、`prune.sh` で残骸が消える

合格条件: 1〜8 全部通る。1 でも失敗したら plan 段階に差し戻し。

## 10. Migration & Rollback

### 10.1 Migration（chezmoi diff → apply で完結）

1. 新ファイル群を chezmoi に追加（4 スクリプト + 1 hook）
2. `dot_config/claude/settings.json.tmpl` に `UserPromptSubmit` / `PreToolUse` の hook entry を **追加**（既存の Stop / Notification は触らない）
3. `dot_config/tmux/conf/status.conf` の `claude-status-count.sh` 呼び出しを `cockpit/summary.sh` に置換
4. `dot_config/tmux/conf/bindings.conf` の `claude_table.s` を `cockpit/switcher.sh` に差替え、`claude_table.N` を `cockpit/next-ready.sh` 呼び出しとして新設
5. `dot_config/tmux/scripts/executable_claude-status-count.sh` / `executable_claude-pick-session.sh` を削除
6. `dot_config/tmux/scripts/executable_claude-kill-session.sh` 末尾に cache 削除 2 行追加
7. `chezmoi diff` 確認 → `chezmoi apply`
8. `tmux source-file ~/.config/tmux/tmux.conf` で reload
9. §9 のスモークテスト実施

### 10.2 Rollback

破壊的変更はゼロ。以下のいずれかで元に戻せる:
- chezmoi リポジトリで `git revert` → `chezmoi apply`
- `~/.claude/settings.json` の追加 hook 行を消す → 状態書き込みが止まる、status-right は無印（空）になるが tmux は壊れない
- `~/.cache/claude-cockpit/` を `rm -rf` → 次回 hook で再生成

## 11. Open Questions

なし（Q1–Q4 全部クローズ）。実装プランへ移行可能。

## 12. References

- 4/29 Cockpit v1 spec: [`2026-04-29-tmux-claude-cockpit-design.md`](./2026-04-29-tmux-claude-cockpit-design.md)
- 4/30 Wired click actions spec: [`2026-04-30-wired-click-actions-design.md`](./2026-04-30-wired-click-actions-design.md)
- 上流（インスピレーション元、コピーなし）: <https://github.com/samleeney/tmux-agent-status>
- Claude Code hooks docs: <https://docs.anthropic.com/en/docs/claude-code/hooks>
