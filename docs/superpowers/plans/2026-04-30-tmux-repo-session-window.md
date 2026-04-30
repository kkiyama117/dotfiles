# tmux Repo-Scoped Session + Branch-Scoped Window Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** flat な `claude-<branch>` session スキームを **session = main repo basename / window = branch / 各 window 2 pane (work + claude)** の階層型に再構成する。

**Architecture:** `tmux-claude-new.sh` を主要編集対象として、main repo basename 解決と `tmux new-window -S` ベースの window 作成に切替える (tmux 3.6 の `new-window` で同名 window を select する正式フラグは `-S`。`-A` は `new-session` のみ)。`claude-kill-session.sh` は kill-window + worktree 削除に縮小し、claude-managed window 判定を window option `@claude-managed` + pane_current_command の OR で行う。`claude-respawn-pane.sh` は既に command 判定なので無変更。binding と docs を新スキームに合わせて更新し、最後に 8 ステップ手動スモークで通す。

**Tech Stack:** bash / tmux 3.6 / git worktree / chezmoi (`dot_config/tmux/...` → `~/.config/tmux/...`).

**Spec:** [`docs/superpowers/specs/2026-04-30-tmux-repo-session-window-design.md`](../specs/2026-04-30-tmux-repo-session-window-design.md)

**Files Affected:**

| chezmoi source | apply target | Action |
|---|---|---|
| `dot_config/tmux/scripts/executable_tmux-claude-new.sh` | `~/.config/tmux/scripts/tmux-claude-new.sh` | **Modify** (session/window 解決ロジック差替え) |
| `dot_config/tmux/scripts/executable_claude-kill-session.sh` | `~/.config/tmux/scripts/claude-kill-session.sh` | **Modify** (kill-window + worktree remove + claude pane detection) |
| `dot_config/tmux/scripts/executable_claude-respawn-pane.sh` | (n/a) | **No change** (`pane_current_command == "claude"` 判定済み、新旧両対応) |
| `dot_config/tmux/conf/bindings.conf` | `~/.config/tmux/conf/bindings.conf` | **Modify** (`claude_table.k` の `-N` ノート + confirm-before message) |
| `docs/manage_claude.md` | (docs only) | **Modify** (新命名規約の解説) |
| `docs/keybinds.md` | (docs only) | **Modify** (`k` の挙動を「window+worktree」に書換) |
| `docs/todos.md` | (docs only) | **Modify** (アクティブタスクへ F-6 を追加) |

---

## Task 1: Refactor `tmux-claude-new.sh` to repo-session + branch-window scheme

**Files:**
- Modify: `dot_config/tmux/scripts/executable_tmux-claude-new.sh:1-187` (whole-file replace)

**目的:** `session=claude-<branch>` を `session=<repo-basename>` + `window=<branch>` に切替え、`@claude-managed` user option を window に立てて kill 側の判定材料にする。

- [ ] **Step 1: Replace whole file with new logic**

`dot_config/tmux/scripts/executable_tmux-claude-new.sh` を以下の内容に置き換える:

```bash
#!/usr/bin/env bash
# usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]] [--no-claude]
# - resolves main repo path; session name = basename of main worktree
# - window name = sanitized branch name; idempotent via `tmux new-window -S`
# - 2-pane (left: shell, right: claude) inside the window
#     default        : `claude --continue --fork-session` if prior history exists,
#                       else plain `claude`
#     --from-root    : `claude --resume <id> --fork-session` (id from main repo's
#                       claude session history; fzf picker if <session-id> omitted)
#     --no-claude    : 1-pane shell window, skips claude entirely
# - tags the window with `@claude-managed=yes` so claude-kill-session.sh can
#   safely act window-scoped without the old `claude-*` session-name prefix
# - attaches or switch-clients to the window

set -euo pipefail

log_file="/tmp/tmux-claude-new.log"
{
  echo "=== $(date -Iseconds) $$ ==="
  echo "argv: $*"
  echo "cwd: $PWD"
  echo "TMUX: ${TMUX:-(unset)}"
} >> "$log_file" 2>/dev/null || true

die() {
  local msg="tmux-claude-new: $*"
  echo "$msg" >&2
  echo "$msg" >> "$log_file" 2>/dev/null || true
  if [ -n "${TMUX:-}" ]; then
    tmux display-message "$msg" 2>/dev/null || true
  fi
  exit 1
}

# tmux session/window name sanitizer (allows alnum, dot, dash, underscore)
sanitize() {
  printf '%s' "$1" | tr -c 'a-zA-Z0-9._-' '-'
}

branch=""
from_root=0
no_claude=0
explicit_session=""

if [ $# -gt 0 ] && [[ "$1" != -* ]]; then
  branch="$1"
  shift
fi

while (( $# )); do
  case "$1" in
    --from-root)
      from_root=1
      shift
      if (( $# )) && [[ "$1" != -* ]]; then
        explicit_session="$1"
        shift
      fi
      ;;
    --no-claude) no_claude=1; shift ;;
    -h|--help)
      echo "usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]] [--no-claude]"
      exit 0
      ;;
    *) die "unknown arg: $1" ;;
  esac
done

[ -z "$branch" ] && die "usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]] [--no-claude]"
(( from_root )) && (( no_claude )) && die "--from-root and --no-claude are mutually exclusive"

safe="$(sanitize "$branch")"
window_name="$safe"

# Resolve main repo (the first worktree entry is always the main worktree).
main_repo=$(git worktree list --porcelain 2>/dev/null | awk '/^worktree / {print $2; exit}')
[ -n "$main_repo" ] || die "not inside a git repo (cwd=$PWD)"

repo_basename="$(basename "$main_repo")"
session="$(sanitize "$repo_basename")"
[ -n "$session" ] || die "failed to resolve repo basename"

# Resolve target worktree.
existing_worktree=$(git worktree list --porcelain | awk -v b="refs/heads/$branch" '
  /^worktree / { wt=$2 }
  $0 == "branch " b { print wt; exit }
')

if [ -n "$existing_worktree" ]; then
  worktree="$existing_worktree"
else
  worktree="${main_repo}-${safe}"
  if [ ! -d "$worktree" ]; then
    if git show-ref --verify --quiet "refs/heads/$branch"; then
      git worktree add "$worktree" "$branch"
    elif git show-ref --verify --quiet "refs/remotes/origin/$branch"; then
      git worktree add -b "$branch" "$worktree" "origin/$branch"
    else
      git worktree add -b "$branch" "$worktree" HEAD
    fi || die "failed to create worktree at $worktree"
  fi
fi

# Resolve root session id when --from-root.
session_id=""
if (( from_root )); then
  encoded=$(printf '%s' "$main_repo" | tr '/.' '-')
  sessions_dir="$HOME/.claude/projects/$encoded"
  [ -d "$sessions_dir" ] || die "no claude sessions at $sessions_dir"
  if [ -n "$explicit_session" ]; then
    [ -f "$sessions_dir/$explicit_session.jsonl" ] || die "session id not found: $explicit_session"
    session_id="$explicit_session"
  else
    command -v fzf >/dev/null 2>&1 || die "fzf required for --from-root without an id"
    pick=$(ls -t "$sessions_dir"/*.jsonl 2>/dev/null | \
      fzf --prompt='root session> ' --preview 'head -50 {}' --height=80%) || exit 0
    [ -z "$pick" ] && exit 0
    session_id=$(basename "$pick" .jsonl)
  fi
fi

# Detect prior claude history for THIS worktree (not the repo session).
worktree_has_history=0
worktree_encoded=$(printf '%s' "$worktree" | tr '/.' '-')
worktree_sessions_dir="$HOME/.claude/projects/$worktree_encoded"
if [ -d "$worktree_sessions_dir" ]; then
  shopt -s nullglob
  jsonl_files=("$worktree_sessions_dir"/*.jsonl)
  shopt -u nullglob
  (( ${#jsonl_files[@]} > 0 )) && worktree_has_history=1
fi

# Idempotent session create. The first window is the branch window itself
# (-n "$window_name", cwd = "$worktree"), so a freshly created session does
# not leave behind an orphan default `zsh` window. When the session already
# exists, -A attaches and -n / -c are ignored.
tmux new-session -A -d -s "$session" -n "$window_name" -c "$worktree" 2>>"$log_file" \
  || die "failed to create or attach session $session"

# Idempotent window create. tmux 3.6's new-window uses -S (NOT -A) to select
# an existing same-named window instead of creating a duplicate.
tmux new-window -S -t "${session}:" -n "$window_name" -c "$worktree" 2>>"$log_file" \
  || die "failed to create or attach window $session:$window_name"

# Mark window as claude-managed. Used by claude-kill-session.sh as a fallback
# when no claude pane is currently running (e.g., user manually closed it).
tmux set-option -w -t "${session}:${window_name}" -o '@claude-managed' yes 2>/dev/null || true

# Inspect window pane count; only set up panes on a fresh window.
pane_count=$(tmux list-panes -t "${session}:${window_name}" -F '.' 2>/dev/null | wc -l)
if [ "$pane_count" -le 1 ]; then
  if (( no_claude )); then
    tmux select-pane -t "${session}:${window_name}.0" -T work 2>/dev/null || true
  else
    tmux split-window -h -t "${session}:${window_name}" -c "$worktree" 2>>"$log_file" \
      || die "failed to split window in $session:$window_name"
    tmux select-pane -t "${session}:${window_name}.0" -T work 2>/dev/null || true
    tmux select-pane -t "${session}:${window_name}.1" -T claude 2>/dev/null || true
    if [ -n "$session_id" ]; then
      claude_cmd="claude --resume $session_id --fork-session"
    elif (( worktree_has_history )); then
      claude_cmd="claude --continue --fork-session"
    else
      claude_cmd="claude"
    fi
    tmux send-keys -t "${session}:${window_name}.1" "$claude_cmd" Enter 2>>"$log_file" \
      || die "failed to send claude command"
  fi
fi

# Switch (or attach if outside tmux).
if [ -n "${TMUX:-}" ]; then
  tmux switch-client -t "${session}:${window_name}" 2>>"$log_file" \
    || die "switch-client to ${session}:${window_name} failed"
  echo "switched to ${session}:${window_name}" >> "$log_file"
else
  tmux attach-session -t "${session}:${window_name}"
fi
```

- [ ] **Step 2: Verify chezmoi diff matches**

Run:
```bash
chezmoi diff ~/.config/tmux/scripts/tmux-claude-new.sh
```

Expected: 全文 diff が表示される。`session="claude-..."` の行が消えて `repo_basename` / `tmux new-window -S` / `@claude-managed` 関連の追加が見える。

- [ ] **Step 3: Apply and syntax-check**

```bash
chezmoi apply ~/.config/tmux/scripts/tmux-claude-new.sh
bash -n ~/.config/tmux/scripts/tmux-claude-new.sh
echo $?
```

Expected: `bash -n` exit 0、no output。

- [ ] **Step 4: Commit**

```bash
git add dot_config/tmux/scripts/executable_tmux-claude-new.sh
git commit -m "feat(tmux): repo-scoped session + branch-scoped window in tmux-claude-new

session = main repo basename, window = sanitized branch, 2 panes per window
(work + claude). Window is tagged with @claude-managed=yes so kill-session
can safely act window-scoped. New worktree path stays
\${main_repo}-<safe-branch>. Refs spec
2026-04-30-tmux-repo-session-window-design.md §3 / §4.1."
```

---

## Task 2: Window-scoped kill in `claude-kill-session.sh`

**Files:**
- Modify: `dot_config/tmux/scripts/executable_claude-kill-session.sh:1-37` (whole-file replace)

**目的:** session-level kill を window-level kill に縮小。`claude-` prefix 判定の代わりに **`@claude-managed` user option もしくは pane_current_command='claude'** を判定材料にする。

- [ ] **Step 1: Replace whole file with new logic**

`dot_config/tmux/scripts/executable_claude-kill-session.sh` を以下に置き換える:

```bash
#!/usr/bin/env bash
# Kill the current claude-managed window and remove its corresponding worktree.
# Caller (binding) wraps this in confirm-before, so we proceed unconditionally
# once the safety check passes.
#
# Safety check (in order, OR semantics):
#   1. window has user option `@claude-managed=yes` (set by tmux-claude-new.sh)
#   2. window has any pane whose pane_current_command == 'claude'
#   3. legacy: session name starts with 'claude-' (old flat scheme)
# If none match, refuse with display-message + exit 1.
#
# Worktree resolution: read pane_current_path of the active pane in the window.
# If it differs from main repo path, attempt git worktree remove.
# If only a session name is passed (legacy callers), fall back to old behavior.

set -euo pipefail

# Optional positional: explicit session-name (legacy support for switcher Ctrl-X
# called with a non-current session). When given, defaults to current window
# inside that session.
explicit_session="${1:-}"

if [ -n "$explicit_session" ]; then
  session="$explicit_session"
  # Pick the active window of the explicit session (best effort).
  window=$(tmux display-message -p -t "$session" '#W' 2>/dev/null) \
    || { tmux display-message "claude-kill-session: session not found ($session)"; exit 1; }
else
  session=$(tmux display-message -p '#S')
  window=$(tmux display-message -p '#W')
fi

target="${session}:${window}"

# --- Safety check ---
managed=$(tmux show-options -w -t "$target" -v '@claude-managed' 2>/dev/null || echo "")
has_claude=""
if [ "$managed" != "yes" ]; then
  has_claude=$(tmux list-panes -t "$target" -F '#{pane_current_command}' 2>/dev/null \
    | grep -Fx claude || true)
fi
legacy=""
case "$session" in claude-*) legacy=1 ;; esac

if [ "$managed" != "yes" ] && [ -z "$has_claude" ] && [ -z "$legacy" ]; then
  tmux display-message "claude-kill-session: refusing on non-claude window ($target)"
  exit 1
fi

# --- Worktree resolution ---
# Prefer pane_current_path of the active pane in the target window.
pane_path=$(tmux display-message -p -t "$target" '#{pane_current_path}' 2>/dev/null || true)
repo_root=""
if [ -n "$pane_path" ]; then
  repo_root=$(git -C "$pane_path" rev-parse --show-toplevel 2>/dev/null || true)
fi

# --- Pre-kill: capture pane ids for cache cleanup ---
pane_ids=$(tmux list-panes -t "$target" -F '#{pane_id}' 2>/dev/null || true)

# --- Kill the window. tmux destroys the session automatically if it was the
# last window. ---
tmux kill-window -t "$target"

# --- Worktree remove (only if pane_path is NOT the main repo) ---
if [ -n "$repo_root" ] && [ -n "$pane_path" ] && [ "$pane_path" != "$repo_root" ]; then
  # pane_path may be a sub-directory of the worktree; resolve worktree root
  # via git -C.
  wt_root=$(git -C "$pane_path" rev-parse --show-toplevel 2>/dev/null || true)
  if [ -n "$wt_root" ] && [ "$wt_root" != "$repo_root" ] && [ -d "$wt_root" ]; then
    git -C "$repo_root" worktree remove "$wt_root" --force 2>/dev/null \
      || tmux display-message "kept worktree $wt_root (remove manually)"
  fi
fi

# --- Cockpit: drop cached state for the killed panes ---
cache="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit"
for pid in $pane_ids; do
  rm -f "$cache/panes/${session}_${pid}.status" 2>/dev/null || true
done
```

- [ ] **Step 2: Verify chezmoi diff matches**

```bash
chezmoi diff ~/.config/tmux/scripts/claude-kill-session.sh
```

Expected: 全文 diff。古い `kill-session -t "$session"` は消え、`kill-window -t "$target"` と `@claude-managed` の `show-options -w` 検査が現れる。

- [ ] **Step 3: Apply and syntax-check**

```bash
chezmoi apply ~/.config/tmux/scripts/claude-kill-session.sh
bash -n ~/.config/tmux/scripts/claude-kill-session.sh
echo $?
```

Expected: exit 0、no output。

- [ ] **Step 4: Commit**

```bash
git add dot_config/tmux/scripts/executable_claude-kill-session.sh
git commit -m "feat(tmux): window-scoped claude-kill-session

Switch from session-level kill to window-level kill. Safety guard now
uses (1) @claude-managed user option, (2) pane_current_command='claude',
(3) legacy 'claude-*' session name — any one is sufficient. Worktree
remove is conditional on the active pane path differing from the main
repo. Cache cleanup is per-pane-id, matching the cockpit cache key
shape. Refs spec 2026-04-30-tmux-repo-session-window-design.md §4.2."
```

---

## Task 3: Update `bindings.conf` `-N` note + confirm-before message for `k`

**Files:**
- Modify: `dot_config/tmux/conf/bindings.conf:80-83`

- [ ] **Step 1: Edit the `claude_table.k` block**

現行の (line 80-83):

```
# k: 現セッション + worktree を削除（要確認）
bind -N "session + worktree 一括削除 (確認あり)" \
  -T claude_table k \
  confirm-before -p "kill claude session and worktree? (y/n) " "run-shell '~/.config/tmux/scripts/claude-kill-session.sh'"
```

を以下に書き換え:

```
# k: 現 window + worktree を削除（要確認）
bind -N "window + worktree 削除 (確認あり、最後の window なら session も destroy)" \
  -T claude_table k \
  confirm-before -p "kill claude window and worktree? (y/n) " "run-shell '~/.config/tmux/scripts/claude-kill-session.sh'"
```

- [ ] **Step 2: Verify chezmoi diff**

```bash
chezmoi diff ~/.config/tmux/conf/bindings.conf
```

Expected: 上記 4 行ぶんの差分のみ。

- [ ] **Step 3: Apply and reload tmux**

```bash
chezmoi apply ~/.config/tmux/conf/bindings.conf
tmux source-file ~/.config/tmux/tmux.conf
```

- [ ] **Step 4: Verify the binding note**

```bash
tmux list-keys -T claude_table -N | grep -F ' k '
```

Expected: `(window + worktree 削除 ...)` の note が表示される。

- [ ] **Step 5: Commit**

```bash
git add dot_config/tmux/conf/bindings.conf
git commit -m "docs(tmux): retitle claude_table.k for window-scoped kill

Reflect new semantics: kill targets the current window + its worktree,
not the entire session. Last-window-of-session still destroys the
session because tmux does it automatically."
```

---

## Task 4: Update `docs/manage_claude.md` and `docs/keybinds.md`

**Files:**
- Modify: `docs/manage_claude.md` (sections describing tmux-claude-new.sh and claude_table)
- Modify: `docs/keybinds.md` (claude_table table row for `k`)

- [ ] **Step 1: Locate the relevant sections**

```bash
rg -n 'claude-<branch>|claude_table\.k|tmux-claude-new' docs/manage_claude.md docs/keybinds.md
```

Expected: いくつかの行ヒット。少なくとも `manage_claude.md` の §5 周辺と `keybinds.md` の §2.2 周辺。

- [ ] **Step 2: Update `docs/manage_claude.md` — naming convention**

`tmux-claude-new.sh` の説明節で `session=claude-<branch>` と書かれている箇所を以下のように書換:

```markdown
- **session 名**: `<main worktree の basename>` (例: `chezmoi`, `data_manager`)
- **window 名**: `<sanitize した branch 名>` (例: `feat-foo`, `develop`)
- **pane 構成**: 各 window 内に左 = work、右 = `claude --continue --fork-session`
- **冪等性**: `tmux new-session -A` と `tmux new-window -S` の組合せで、同じ branch を 2 度叩いても既存の session/window に attach するだけ
- **window 識別**: `@claude-managed=yes` を window option として set。`claude-kill-session.sh` の安全判定で参照される
```

`claude_table.k` の説明:

```markdown
- `k`: 現 window と対応 worktree を削除 (`confirm-before` で確認あり)。
  - session 内に他の window があれば session 自体は残る
  - 最後の window だった場合 tmux が自動的に session を destroy する
  - 安全判定: window option `@claude-managed=yes` / pane の `claude` プロセス / 旧 `claude-*` session 名 のいずれか 1 つを満たす場合のみ実行
```

- [ ] **Step 3: Update `docs/keybinds.md` — claude_table table**

§2.2 (claude_table の表) で `k` 行を:

```markdown
| `k` | 現 window と対応 worktree を削除 (確認あり、最後の window なら session も destroy) | `claude-kill-session.sh` |
```

に書換 (元は「session + worktree」)。

- [ ] **Step 4: Verify diff**

```bash
git diff docs/manage_claude.md docs/keybinds.md
```

Expected: 上記の修正のみ。

- [ ] **Step 5: Commit**

```bash
git add docs/manage_claude.md docs/keybinds.md
git commit -m "docs: describe repo-session + branch-window scheme

Update tmux-claude-new.sh and claude_table.k descriptions to reflect
the new naming convention (session = repo basename, window = branch)
and the window-scoped kill behavior with @claude-managed safety guard."
```

---

## Task 5: Add F-6 entry to `docs/todos.md`

**Files:**
- Modify: `docs/todos.md` (append new active task before deferred section)

- [ ] **Step 1: Insert new entry after F-5 block**

`docs/todos.md` の F-5 ブロック直後 (デファード節の前) に以下を挿入:

```markdown
### F-6. tmux session/window 階層再構成 (実装中)
- 背景: flat な `claude-<branch>` session スキームを **session = main repo basename / window = branch / 各 window 2 pane** に再構成。複数 repo で同名 branch (`develop` 等) を持つ際の session 名衝突を解消し、cockpit 階層 fzf スイッチャの 3 段表現を活かす。設計は [`superpowers/specs/2026-04-30-tmux-repo-session-window-design.md`](superpowers/specs/2026-04-30-tmux-repo-session-window-design.md)、実装計画は [`superpowers/plans/2026-04-30-tmux-repo-session-window.md`](superpowers/plans/2026-04-30-tmux-repo-session-window.md)。
- 該当: `tmux-claude-new.sh` / `claude-kill-session.sh` / `bindings.conf` / `manage_claude.md` / `keybinds.md`
- 対応:
  - [ ] Task 1: tmux-claude-new.sh を repo-session + branch-window scheme に refactor
  - [ ] Task 2: claude-kill-session.sh を window-scoped kill に縮小、`@claude-managed` 判定を導入
  - [ ] Task 3: bindings.conf の `claude_table.k` の note と confirm message を更新
  - [ ] Task 4: docs/manage_claude.md と docs/keybinds.md を新スキーマに更新
  - [ ] Task 5: 8-step 手動スモークの実機通し
- 注意:
  - 既存 `claude-*` session には介入しない (自然消滅させる migration)
  - tmux-continuum の resurrect で旧 session 名が部分復活する可能性あり (要 follow-up)
  - 異 repo + 同 basename の collision は v1 では非対応 (spec §3.4 / Q1)
```

最終更新行を更新:

```markdown
最終更新: 2026-04-30 (F-6 tmux session/window 階層再構成 plan 着手)
```

- [ ] **Step 2: Commit**

```bash
git add docs/todos.md
git commit -m "docs(todos): add F-6 tmux session/window redesign active task"
```

---

## Task 6: Manual smoke test (8 steps)

**Files:** None (validation only)

**Note:** これは tty / 実 tmux client が必要なので **ユーザ手動実行**。失敗時は該当 Task に差戻し。

- [ ] **Step 1: tmux clean restart**

```bash
tmux kill-server 2>/dev/null || true
tmux new-session -d -s scratch
tmux source-file ~/.config/tmux/tmux.conf
tmux attach
```

Expected: tmux に attach、`scratch` session に居る。

- [ ] **Step 2: Create first window via `prefix + C → n`**

chezmoi リポジトリ (`~/.local/share/chezmoi`) の pane に居て:
1. `prefix + C` → `n` で fzf popup
2. `develop` を選択 (なければ任意の既存 branch)

Expected:
- session=`chezmoi` が新規作成される (もしくは attach)
- window=`develop` が立ち、左 pane=`work`、右 pane で `claude` が起動
- status-line が `chezmoi:develop` を示す

- [ ] **Step 3: Create second window in same session**

別の branch (例: `feat/foo`、無ければ `prefix + C → n` 後に新 branch を入力) を選択。

Expected:
- session=`chezmoi` 内に window=`feat-foo` が増える (合計 2 windows)
- session 名は変わらず `chezmoi`

- [ ] **Step 4: Verify hierarchy in cockpit switcher**

`prefix + C → s` で switcher popup を開く。

Expected:
- session=`chezmoi` の下に 2 windows (`develop`, `feat-foo`)
- 各 window の下に 2 panes
- session=`scratch` も別ツリーで見える (claude-managed ではないので Ctrl-X で kill 拒否されることも確認)

- [ ] **Step 5: Cross-repo session isolation**

別の repo (例: `~/programs/data_manager` がなければ任意の git リポ) で `prefix + C → n` → 任意の branch 選択。

Expected:
- 新 session が `data_manager` (または該当 repo basename) として作成される
- `chezmoi` session とは独立

- [ ] **Step 6: Window kill via `prefix + C → k`**

`develop` window に居る状態で `prefix + C → k` → `y` で確認。

Expected:
- window `develop` が消える
- worktree `~/.local/share/chezmoi-develop` が消える (`git worktree list` で確認)
- session=`chezmoi` は `feat-foo` が残るので生存

- [ ] **Step 7: Last-window kill destroys session**

残った `feat-foo` window で再度 `prefix + C → k` → `y`。

Expected:
- window が消えると同時に session=`chezmoi` も自動 destroy
- attach 中のクライアントは別 session (例: `scratch` か `data_manager`) に切替わる

- [ ] **Step 8: Cockpit summary correctness**

`data_manager` 等で適当に Claude を立ち上げ → status-right を確認。

Expected: `⚡ N ⏸ M ✓ K ` の集計が新 schema 下でも壊れず動く (cache layout 不変のため)。

- [ ] **Step 9: Mark all F-6 tasks done in todos.md and commit**

8 ステップが全部通ったら docs/todos.md の F-6 内のチェックボックスを `[x]` に書換 + 末尾に「完了 (YYYY-MM-DD)」追記:

```bash
git add docs/todos.md
git commit -m "docs(todos): F-6 tmux session/window redesign complete"
```

失敗があった場合は該当 Task に差戻し、原因 commit を `docs/todos.md` の F-6 ブロックに追記。

---

## Self-Review Notes

- **Spec coverage**: §3 (naming) → Task 1。§4.1 (tmux-claude-new) → Task 1。§4.2 (kill-session) → Task 2。§4.3 (pick-branch) → no change required。§4.4 (respawn-pane) → **既に command 判定で動作するため変更不要、spec §4.4 の現状記述は誤り** (実装は `pane_current_command` を見ている)。§4.5–4.7 (cockpit) → cache layout 不変のため無変更。§4.8 (bindings) → Task 3。§5 (migration) → §5.1 通り破壊的変更ゼロ。§7 (testing) → Task 6。§8 (error handling) → Task 1, 2 の `die` で吸収。§9 Q1–Q4 → 全て recommendation 採用 (Q4 は実装側で既に解決済み判明)。
- **Placeholder scan**: なし。すべて具体コードで埋めた。
- **Type/identifier consistency**: `session` / `window_name` / `target="${session}:${window_name}"` を Task 1, 2, 3 で一貫使用。`@claude-managed` window option 名も Task 1 (set) と Task 2 (read) で同じ。
