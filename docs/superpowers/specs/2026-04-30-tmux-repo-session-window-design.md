# tmux: Repo-Scoped Session + Branch-Scoped Window (Design Spec)

- **Date**: 2026-04-30
- **Status**: Draft (brainstorming, awaiting user review)
- **Owner**: kiyama
- **Repo**: chezmoi (`/home/kiyama/.local/share/chezmoi`)
- **Extends**: [`2026-04-29-tmux-claude-cockpit-design.md`](./2026-04-29-tmux-claude-cockpit-design.md), [`2026-04-30-claude-cockpit-state-tracking-design.md`](./2026-04-30-claude-cockpit-state-tracking-design.md)

---

## 1. Goal

現行は **session = `claude-<branch>` / 各 session は 2 pane (work + claude)** という flat な構造。これを **session = main repo basename / window = branch (worktree) / 各 window が 2 pane** という階層型に再構成する。

具体的に解決すること:

- **G1 セッション名衝突解消**: 複数リポジトリで `develop` / `main` / `feature-x` などの同名ブランチを持つと、現状の `claude-develop` がリポジトリ間で衝突する。session 名をリポジトリ単位にすれば衝突しない
- **G2 認知モデルの一致**: 「今 chezmoi を触っている → chezmoi session に居る → window 一覧で worktree 切替」という tmux 本来の階層と作業の単位を一致させる
- **G3 cockpit 階層 fzf スイッチャの活用**: `cockpit/switcher.sh` は既に session → window → pane の 3 階層を扱うのに、現行の flat 構造では window 階層が常に 1 段しかなく switcher の表現力が活きていない

## 2. Non-Goals

- 既存 `claude-*` session の自動 in-place マイグレーション (kill-server 経由で自然消滅させる、アクティブ作業中は触らない)
- worktree 自体のレイアウト変更 (`<repo-root>-<safe-branch>` 命名は維持)
- cockpit state tracking のキャッシュ層変更 (`panes/<S>_<P>.status` のキー構造は session 名 + pane_id のままで動く)
- multi-repo 同名 basename の完全自動解決 (Q1 で議論、簡易戦略のみ採用)
- claude プロセス起動方式の変更 (`claude --continue --fork-session` / `--resume` の resolve は worktree path 単位のまま)
- tmux-continuum の保存/復元との互換性チェック (別 follow-up)

## 3. Naming Convention

### 3.1 Session 名 = main worktree の basename

`tmux-claude-new.sh <branch>` 実行時:

```
main_repo=$(git worktree list --porcelain | awk '/^worktree / {print $2; exit}')
repo_name=$(basename "$main_repo")
session="$repo_name"
```

例:
| 現行 | 新 |
|---|---|
| `claude-main` (chezmoi の main) | `chezmoi` |
| `claude-develop` (chezmoi の develop) | `chezmoi` (window=`develop`) |
| `claude-feature-x` (other-repo の feature-x) | `other-repo` (window=`feature-x`) |

### 3.2 Window 名 = branch (slash sanitized)

```
safe="${branch//\//-}"
window_name="$safe"
```

例: `feat/foo` → window 名 `feat-foo`、worktree 名 `<repo-root>-feat-foo` (現行と同じ)。

### 3.3 Pane 構成 = window あたり 2 pane

各 window 内に既存の 2 pane (work + claude) をそのまま展開する。pane title (`work` / `claude`) も現行と同じ。

### 3.4 名前衝突解決

| ケース | 戦略 |
|---|---|
| 同 repo + 同 branch を 2 度叩く | 既存 worktree 検出で `tmux new-window -S` 冪等 (tmux 3.6 の `new-window` には `-A` フラグはなく、同名 window を select する目的では `-S` を使う)。何もせず switch するだけ |
| 同 repo + 異 branch | 別 window として作成 |
| 異 repo + 同 basename (例: `~/code/dotfiles` と `~/work/dotfiles`) | **basename 衝突は手動で命名規約破壊として扱う**。session 名に `repo_name` をそのまま使うため、後発の `tmux new-session -A` が既存 session に attach してしまう (= 別リポの worktree が同じ session 内に混入する) 危険がある。Q1 で自動 disambiguation を検討するが、v1 ではユーザーが片方の repo を rename / symlink で別 basename にする運用とする |
| 異 repo + 異 branch | session が別なので無関係 |

## 4. Behavior Changes (per script)

### 4.1 `tmux-claude-new.sh`

| 工程 | 現行 | 新 |
|---|---|---|
| session 名解決 | `session="claude-${safe}"` | `session="$(basename "$(git worktree list --porcelain \| awk '/^worktree / {print $2; exit}')")"` |
| session 作成 | `tmux new-session -A -d -s "$session" -c "$worktree"` | `tmux new-session -A -d -s "$session" -n "$window_name" -c "$worktree"` (最初の window がブランチ window 自身。`-c $worktree` で worktree 直下から開始し、既定の `zsh` orphan window を残さない) |
| pane 構成 | session 直下に 2 pane | window 直下に 2 pane に変更 |
| window 作成 | (なし、session 直下で完結) | `tmux new-window -S -t "$session:" -n "$window_name" -c "$worktree"` を session 作成直後に実行 (tmux 3.6 の `new-window` で同名 window を select する正しいフラグは `-S`。`-A` は `new-session` のみのフラグ) |
| pane split | `tmux split-window -h -t "$session"` | `tmux split-window -h -t "${session}:${window_name}"` |
| send-keys | `tmux send-keys -t "${session}.1"` | `tmux send-keys -t "${session}:${window_name}.1"` |
| 終結 | `switch-client -t "$session"` | `switch-client -t "${session}:${window_name}"` (window まで指定) |

`--no-claude` / `--from-root` オプションのセマンティクスは window 単位に縮約して同じ意味で使える。

### 4.2 `claude-kill-session.sh`

スコープが大幅に縮む。**「現 window + worktree を削除」に再定義**:

```
current_session=$(tmux display-message -p '#S')
current_window=$(tmux display-message -p '#W')
worktree_path=$(tmux display-message -p '#{pane_current_path}')

# 1. window を kill (session に他 window があれば session は残る)
tmux kill-window -t "${current_session}:${current_window}"

# 2. worktree が main repo でなければ remove
main_repo=...
if [ "$worktree_path" != "$main_repo" ]; then
  git worktree remove --force "$worktree_path"
fi

# 3. cockpit cache 削除 (session 単位ではなく pane_id 単位)
rm -f "$cache_dir/${current_session}_${current_pane_id}.status"
```

副作用変化:
- session 自体は他 window が残っていれば生存 → ユーザは別 worktree の作業を継続できる
- 最後の window を kill すると tmux が自動で session を destroy (既存挙動)
- cache は **session+pane_id 複合キー** だが、session 名がリポ単位で固定になるので削除粒度は pane 単位で十分

### 4.3 `claude-pick-branch.sh`

変更なし。fzf で branch を選んで `tmux-claude-new.sh` に渡す責務だけなので、新形式の session 解決は呼び出し側で完結する。

### 4.4 `claude-respawn-pane.sh`

現行は `claude-` prefix で claude pane を判定している。新形式では session 名から prefix が消えるため:

| 現行判定 | 新判定 |
|---|---|
| `tmux list-sessions \| grep '^claude-'` | pane title が `claude` の pane を直接探す (`tmux list-panes -a -F '#{session_name}:#{window_index}.#{pane_index} #{pane_title}' \| grep ' claude$'`) |

または `claude` pane に environment variable を仕込んで `tmux show-environment` で識別する方式もあるが、pane title 方式が一番素直。

### 4.5 `cockpit/summary.sh`

cache layout を変えないので集計ロジック変更なし。ただし「claude session の数」を数える概念は **「claude pane の数」に置き換え済み** (state tracking 導入時点で)。新形式でも壊れない。

### 4.6 `cockpit/switcher.sh`

階層 fzf は既に session → window → pane の 3 段を扱う設計なので、構造変更なし。むしろ window 階層に意味のあるラベル (branch 名) が入ることで使い勝手が向上する。

`Ctrl-X` の kill 動作だけ要確認: 現行は session 単位 kill。新形式では「window kill が妥当か session kill が妥当か」を選択候補のレベルで分岐する。提案:

| 選択カーソル位置 | Ctrl-X の動作 |
|---|---|
| session 行 | session 全体を kill (= 全 worktree 削除を伴う、要確認 prompt) |
| window 行 | window + 該当 worktree のみ削除 |
| pane 行 | pane だけ kill (worktree は残す) |

### 4.7 `cockpit/next-ready.sh`

`waiting` バケット → `done` バケット の優先順で要対応 pane を巡回する仕様 (state-tracking design §6.4 の更新版)。各バケット内では session asc / window idx asc / pane idx asc の inbox 順。F-6 の repo-scoped session でも変わらず動作する (session 数が減って window 数が増えるだけ)。

### 4.8 `dot_config/tmux/conf/bindings.conf`

`claude_table` のキーセマンティクスは保持。`k` の挙動説明文を「session + worktree」から「window + worktree」に書き換えるのと、confirm-before のメッセージを更新する。

```
bind -N "window + worktree 削除 (確認あり)" \
  -T claude_table k \
  confirm-before -p "kill claude window and worktree? (y/n) " \
  "run-shell '~/.config/tmux/scripts/claude-kill-session.sh'"
```

スクリプト名 `claude-kill-session.sh` は責務とズレるが rename は別 follow-up (互換性維持優先、Q2)。

## 5. Migration & Rollback

### 5.1 Migration

破壊的変更ゼロ前提。既存 `claude-*` session に積極介入しない:

1. 新 `tmux-claude-new.sh` を deploy (`chezmoi apply`)
2. **ユーザ判断で** 既存 `claude-*` session を順次 detach → 作業終了 → 自然消滅させる
3. 新規 worktree 作成は新形式で (= 既存 session には属さない `<repo>` session を新設)
4. tmux-continuum の next save まで両者並走可能 (session 名が異なるので衝突しない)
5. 完全移行確認後、`claude-respawn-pane.sh` を新判定ロジックに切替 (旧 `claude-` prefix サポートを除去)

### 5.2 Rollback

各スクリプトの変更点はすべて関数レベルの差分 1〜数十行。`git revert` 1 発で旧形式に戻せる。cache layout は不変なので state file の整合性は維持される。

## 6. Edge Cases

| ケース | 対応 |
|---|---|
| 同 basename 異 path repo | §3.4 通り v1 では非対応。Q1 で議論 |
| basename に空白 / 特殊文字 | tmux session 名で問題 → `tr -c 'a-zA-Z0-9._-' '-'` で sanitize する関数を追加 |
| worktree なし repo (chezmoi 等で submodule のみ) | 既存挙動どおり main repo 自体の cwd で window を作る |
| claude-kill 中に他 window の作業中 process あり | window kill は他 window に影響しない (tmux 標準挙動) |
| session に手動で追加した window を持つ場合 | 影響なし。kill 対象は **現 window** のみ |
| `--from-root` の session_id picker | main repo session の `~/.claude/projects/<encoded>` から拾う。session 名が `<repo>` に簡略化されるだけで encoded path は worktree path 由来なので変更なし |
| tmux-continuum の保存ファイル | session 名スキーマが変わるため、復元時に旧 `claude-*` session が部分復活する可能性。継続発生時は `~/.local/share/tmux/resurrect/` の手動掃除を案内 |

## 7. Testing Strategy

ユニットテスト書かない (総差分 ~150 行未満)。手動スモーク 8 ステップ:

1. clean 状態で `tmux kill-server` → tmux 起動
2. chezmoi (main 居る状態) で `prefix + C → n` → `develop` 選択 → session=`chezmoi`、window=`develop` で 2 pane が立つ
3. 別タブで `prefix + C → n` → `feat/foo` 選択 → 同じ `chezmoi` session に window=`feat-foo` が増える
4. `prefix + C → s` の switcher で 3 階層 (session=chezmoi → 2 windows → 各 2 panes) が見える
5. 別 repo (例: `~/programs/data_manager`) で `prefix + C → n` → 別 session が立つ (basename 衝突なし)
6. `develop` window で `prefix + C → k` 確認 → window と worktree が消え、session は他 window が残るので生存
7. 最後の window を kill → session も自動 destroy
8. cockpit summary は `⚡ N ⏸ M ✓ K ` を新形式でも正しく集計

## 8. Error Handling

| ケース | 対応 |
|---|---|
| `git worktree list` 失敗 | `tmux-claude-new.sh` の既存 `die` で abort |
| basename が空 | `die "failed to resolve repo name"` |
| `tmux new-window -S` 失敗 | `die "failed to create or attach window"` |
| 同名 window が既にあるが pane 数が 1 | `-A` で attach した先で pane_count を確認、不足分を split で追加 |
| basename collision で別 repo の session に attach | v1 では検出しない (Q1 で対策検討) |

## 9. Open Questions

### Q1. 異 repo 同 basename の自動 disambiguation 戦略
候補:
- A) 何もしない (ユーザ規約)。**v1 採用候補**
- B) collision 検出時に `<repo>-<short-hash>` 形式に fallback (`git rev-parse --show-toplevel | sha256sum | head -c 6`)
- C) `<parent-dir>-<repo>` 形式 (`work-dotfiles` vs `code-dotfiles`)

A は最小コスト、B は再現性高いが session 名が読みにくくなる、C は人間可読だが parent 名重複も起こりうる。**recommendation: A → 衝突顕在化したら B にアップグレード**。

### Q2. `claude-kill-session.sh` の rename
責務が "kill window" になるなら `claude-kill-window.sh` が自然。ただし bindings.conf / docs / 過去 commit 言及で互換性を壊すコストあり。**recommendation: v1 では rename しない。F-X として将来別 PR**。

### Q3. status-right に session 名を出すか
複数 repo に同時 attach するケースが増えるなら、`#S #W` を status-right に出す価値が上がる。現行は `cockpit/summary.sh` だけ表示。**recommendation: v1 では追加しない。実運用してから判断**。

### Q4. `claude-respawn-pane.sh` の判定ロジック切替タイミング
旧 prefix 判定を残したまま新形式の pane title 判定を or 条件で並走させる方式と、一気に切替方式がある。**recommendation: 並走 (or 条件)**。旧 session が完全消滅するまで余裕を持たせる。

## 10. References

- 4/29 Cockpit v1: [`2026-04-29-tmux-claude-cockpit-design.md`](./2026-04-29-tmux-claude-cockpit-design.md)
- 4/30 State tracking: [`2026-04-30-claude-cockpit-state-tracking-design.md`](./2026-04-30-claude-cockpit-state-tracking-design.md)
- tmux man page: `tmux(1)` — `new-window -S` (NOT `-A`; `-A` is `new-session`-only), `kill-window`, `select-pane`, `switch-client`
