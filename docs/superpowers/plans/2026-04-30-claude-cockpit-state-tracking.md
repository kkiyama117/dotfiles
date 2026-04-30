# Claude Cockpit State Tracking — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** tmux + Claude のステート追跡・集約サマリ・階層 fzf スイッチャ・next-ready ジャンプを、`samleeney/tmux-agent-status` の設計を参考にしつつ自前実装で導入する。

**Architecture:** Claude hooks が `$XDG_CACHE_HOME/claude-cockpit/panes/<session>_<pane>.status` に三値ステート（`working/waiting/done`）を atomic write し、tmux side のスクリプト群が都度キャッシュを集計して status-right / fzf popup / next-ready ジャンプを駆動する。常駐 daemon を持たず、既存の通知フック（音 + popup）と並走する責務分離。

**Tech Stack:** POSIX shell (bash), fzf, tmux 3.x, Claude Code hooks (UserPromptSubmit / PreToolUse / Stop / Notification), chezmoi。

**Spec:** [`docs/superpowers/specs/2026-04-30-claude-cockpit-state-tracking-design.md`](../specs/2026-04-30-claude-cockpit-state-tracking-design.md)

**Repo conventions reminder:**
- `dot_config/foo/bar` (chezmoi src) → `~/.config/foo/bar` (target)
- `dot_local/bin/executable_xxx.sh` (chezmoi src) → `~/.local/bin/xxx.sh` (target、実行ビット付き)
- 編集後は必ず `chezmoi diff` → `chezmoi apply`（`docs/` はチェックアウト直下なので apply 不要）
- 新規ファイル名 prefix `executable_` を忘れない（実行ビットが付かない）
- テンプレートではない既存ファイル（`dot_config/claude/settings.json`）は **そのまま素の JSON 編集**

---

## File Structure

| Path (chezmoi src) | Target | Action | Responsibility |
|---|---|---|---|
| `dot_local/bin/executable_claude-cockpit-state.sh` | `~/.local/bin/claude-cockpit-state.sh` | **NEW** | Claude hook entry。`$TMUX_PANE` から session 解決 → `panes/<S>_<P>.status` に状態 atomic write |
| `dot_config/tmux/scripts/cockpit/executable_summary.sh` | `~/.config/tmux/scripts/cockpit/summary.sh` | **NEW** | status-right 用集計（`⚡ N ⏸ M ✓ K `） |
| `dot_config/tmux/scripts/cockpit/executable_prune.sh` | `~/.config/tmux/scripts/cockpit/prune.sh` | **NEW** | tmux に存在しない pane の状態ファイル削除 |
| `dot_config/tmux/scripts/cockpit/executable_switcher.sh` | `~/.config/tmux/scripts/cockpit/switcher.sh` | **NEW** | fzf 階層スイッチャ（session/window/pane） |
| `dot_config/tmux/scripts/cockpit/executable_next-ready.sh` | `~/.config/tmux/scripts/cockpit/next-ready.sh` | **NEW** | inbox 順 done pane に循環ジャンプ |
| `dot_config/claude/settings.json` | `~/.config/claude/settings.json` | **MODIFY** | 4 イベント (`UserPromptSubmit/PreToolUse/Stop/Notification`) の `hooks` 配列に state hook を 追加（既存エントリは保持） |
| `dot_config/tmux/conf/status.conf` | `~/.config/tmux/conf/status.conf` | **MODIFY** | `claude-status-count.sh` 呼び出しを `cockpit/summary.sh` に差替え |
| `dot_config/tmux/conf/bindings.conf` | `~/.config/tmux/conf/bindings.conf` | **MODIFY** | `claude_table.s` を `cockpit/switcher.sh` に差替え + `claude_table.N` 新設 |
| `dot_config/tmux/scripts/executable_claude-kill-session.sh` | `~/.config/tmux/scripts/claude-kill-session.sh` | **MODIFY** | 末尾に cache 削除 2 行追加 |
| `dot_config/tmux/tmux.conf` | `~/.config/tmux/tmux.conf` | **MODIFY** | server 起動時に `prune.sh` を 1 度実行する `run -b` を追加 |
| `dot_config/tmux/scripts/executable_claude-status-count.sh` | `~/.config/tmux/scripts/claude-status-count.sh` | **DELETE** | `cockpit/summary.sh` に置換 |
| `dot_config/tmux/scripts/executable_claude-pick-session.sh` | `~/.config/tmux/scripts/claude-pick-session.sh` | **DELETE** | `cockpit/switcher.sh` に置換 |
| `docs/manage_claude.md` | (chezmoi 管理外、リポジトリ doc) | **MODIFY** | spec §9 のスモークテスト 8 ステップを末尾に追記 |

**Cache layout (created at runtime, not chezmoi-managed):**

```
${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/
├── panes/<tmux-session>_<pane-id>.status   # 値: "working" | "waiting" | "done"
└── sessions/                                # 予約のみ。本プランの範囲外
```

---

## Task 1: Add `claude-cockpit-state.sh` (Claude hook entry)

**Files:**
- Create: `dot_local/bin/executable_claude-cockpit-state.sh`

- [ ] **Step 1: Write the script**

Create `dot_local/bin/executable_claude-cockpit-state.sh` with content:

```bash
#!/usr/bin/env bash
# Claude Code hook entry for tmux cockpit state tracking.
# Writes "working" / "waiting" / "done" to a per-pane cache file based on
# the hook event. Exits 0 unconditionally so Claude is never blocked.
#
# Usage:
#   claude-cockpit-state.sh hook <Event>
# Events recognized:
#   UserPromptSubmit -> working
#   PreToolUse       -> working
#   Stop             -> done
#   Notification     -> waiting
# Other events: ignored.
#
# Stdin: claude hook payload (JSON). Currently unused; reserved.

set -u  # NOT -e: never let a tool failure propagate to claude

mode="${1:-}"
event="${2:-}"

# Only "hook" mode is implemented for now.
[ "$mode" != "hook" ] && exit 0

case "$event" in
  UserPromptSubmit|PreToolUse) state="working" ;;
  Notification)                state="waiting" ;;
  Stop)                        state="done" ;;
  *)                           exit 0 ;;
esac

# tmux 外で動いている場合は no-op
tmux_pane="${TMUX_PANE:-}"
[ -z "$tmux_pane" ] && exit 0

command -v tmux >/dev/null 2>&1 || exit 0

tmux_session=$(tmux display-message -p -t "$tmux_pane" '#{session_name}' 2>/dev/null) || exit 0
[ -z "$tmux_session" ] && exit 0

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
mkdir -p "$cache_dir" 2>/dev/null || exit 0

file="$cache_dir/${tmux_session}_${tmux_pane}.status"
tmp="$file.$$.tmp"

# atomic write: write to tmp, then rename
if printf '%s' "$state" > "$tmp" 2>/dev/null; then
  mv "$tmp" "$file" 2>/dev/null || rm -f "$tmp"
fi

exit 0
```

- [ ] **Step 2: Apply to home dir**

Run:
```bash
chezmoi diff dot_local/bin/executable_claude-cockpit-state.sh
chezmoi apply ~/.local/bin/claude-cockpit-state.sh
ls -la ~/.local/bin/claude-cockpit-state.sh
```

Expected: ファイルが配置され `-rwxr-xr-x` で実行ビットが付いている。

- [ ] **Step 3: Manual verification (working / done / waiting transitions)**

Run:
```bash
# Spin up a throwaway tmux session
tmux new-session -d -s cockpit-test -x 80 -y 24
pane_id=$(tmux display-message -p -t cockpit-test '#{pane_id}')

# Simulate UserPromptSubmit
TMUX_PANE="$pane_id" ~/.local/bin/claude-cockpit-state.sh hook UserPromptSubmit
cat "${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes/cockpit-test_${pane_id}.status"
echo
```

Expected output: `working`

```bash
# Simulate Notification (waiting for input)
TMUX_PANE="$pane_id" ~/.local/bin/claude-cockpit-state.sh hook Notification
cat "${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes/cockpit-test_${pane_id}.status"
echo
```

Expected output: `waiting`

```bash
# Simulate Stop
TMUX_PANE="$pane_id" ~/.local/bin/claude-cockpit-state.sh hook Stop
cat "${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes/cockpit-test_${pane_id}.status"
echo
```

Expected output: `done`

```bash
# Verify outside-tmux execution is a no-op (no error)
unset TMUX_PANE
~/.local/bin/claude-cockpit-state.sh hook Stop
echo "exit=$?"
```

Expected output: `exit=0`（cache に余計なファイルが増えないこと）

- [ ] **Step 4: Cleanup**

Run:
```bash
tmux kill-session -t cockpit-test
rm -f "${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes/cockpit-test_"*.status
```

- [ ] **Step 5: Commit**

```bash
git add dot_local/bin/executable_claude-cockpit-state.sh
git -c commit.gpgsign=false commit -m "feat(claude): add cockpit state hook entry

Atomic-write working/waiting/done state to per-pane cache files based on
Claude Code hook events. No-op outside tmux. Never blocks Claude.

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §6.2"
```

---

## Task 2: Wire state hook into Claude `settings.json`

**Files:**
- Modify: `dot_config/claude/settings.json`

The existing `hooks` block already has entries on `PreToolUse` (continuous-learning observer), `Notification`, `Stop` (notify-hook). We **add** new entries that also call `claude-cockpit-state.sh`. We **never replace** existing entries.

`UserPromptSubmit` is brand-new (the file currently has no entry for it).

- [ ] **Step 1: Read the current `hooks` block**

Run:
```bash
sed -n '40,83p' dot_config/claude/settings.json
```

Note the existing structure for reference.

- [ ] **Step 2: Edit the `hooks` block**

Modify `dot_config/claude/settings.json` to:

a) **Add** a new top-level key `"UserPromptSubmit"` inside `"hooks"` (before `"PreToolUse"`):

```json
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/kiyama/.local/bin/claude-cockpit-state.sh hook UserPromptSubmit"
          }
        ]
      }
    ],
```

b) **Append** a 2nd entry to the existing `"PreToolUse"` array (alongside `observe.sh`):

```json
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/kiyama/.local/bin/claude-cockpit-state.sh hook PreToolUse"
          }
        ]
      }
```

c) **Append** a 2nd entry to the existing `"Notification"` array (alongside `claude-notify-hook.sh notification`):

```json
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/kiyama/.local/bin/claude-cockpit-state.sh hook Notification"
          }
        ]
      }
```

d) **Append** a 2nd entry to the existing `"Stop"` array (alongside `claude-notify-hook.sh stop`):

```json
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/kiyama/.local/bin/claude-cockpit-state.sh hook Stop"
          }
        ]
      }
```

`PostToolUse` is **not** modified (state hook does not need it).

The end result for the modified `hooks` block (full structure):

```json
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/kiyama/.local/bin/claude-cockpit-state.sh hook UserPromptSubmit"
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "/home/kiyama/.claude/plugins/cache/everything-claude-code/everything-claude-code/1.10.0/skills/continuous-learning-v2/hooks/observe.sh"
          }
        ]
      },
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/kiyama/.local/bin/claude-cockpit-state.sh hook PreToolUse"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "/home/kiyama/.claude/plugins/cache/everything-claude-code/everything-claude-code/1.10.0/skills/continuous-learning-v2/hooks/observe.sh"
          }
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/kiyama/.local/bin/claude-notify-hook.sh notification"
          }
        ]
      },
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/kiyama/.local/bin/claude-cockpit-state.sh hook Notification"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/kiyama/.local/bin/claude-notify-hook.sh stop"
          }
        ]
      },
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/kiyama/.local/bin/claude-cockpit-state.sh hook Stop"
          }
        ]
      }
    ]
  },
```

- [ ] **Step 3: Validate JSON**

Run:
```bash
python3 -c 'import json,sys; json.load(open("dot_config/claude/settings.json")); print("JSON OK")'
```

Expected: `JSON OK`

- [ ] **Step 4: Apply to home dir**

```bash
chezmoi diff ~/.config/claude/settings.json
chezmoi apply ~/.config/claude/settings.json
```

`~/.claude` は `~/.config/claude` への symlink なので、Claude 側は次回起動時に新 hook を読む。

- [ ] **Step 5: Verify hooks fire end-to-end**

Run:
```bash
# Start a fresh claude session in a fresh tmux pane
tmux new-session -d -s cockpit-e2e -x 120 -y 30
tmux send-keys -t cockpit-e2e "claude --print --max-turns 1 'say hello'" Enter

# Wait for claude to finish (this prints output)
sleep 6

# Check state file was written
ls "${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes/" | grep cockpit-e2e
cat "${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes/cockpit-e2e_"*.status
echo
```

Expected: 1 ファイル存在し、中身は `done`（Stop hook が最後に発火するため）。

- [ ] **Step 6: Cleanup**

```bash
tmux kill-session -t cockpit-e2e
rm -f "${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes/cockpit-e2e_"*.status
```

- [ ] **Step 7: Commit**

```bash
git add dot_config/claude/settings.json
git -c commit.gpgsign=false commit -m "feat(claude): wire cockpit state hook into 4 events

Adds UserPromptSubmit/PreToolUse/Stop/Notification entries to invoke
claude-cockpit-state.sh alongside existing observers and notify-hook.
Pre-existing hook commands are preserved unchanged.

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §6.1, §10.1"
```

---

## Task 3: Add `cockpit/summary.sh` (status-right aggregator)

**Files:**
- Create: `dot_config/tmux/scripts/cockpit/executable_summary.sh`

- [ ] **Step 1: Write the script**

Create `dot_config/tmux/scripts/cockpit/executable_summary.sh` with content:

```bash
#!/usr/bin/env bash
# Aggregate per-pane state files into a status-right summary string.
# Output examples:
#   "⚡ 3 ⏸ 1 ✓ 2 "    (mixed)
#   "✓ 2 "             (only done)
#   ""                 (no state files / all empty)
# A single trailing space is included when output is non-empty so the next
# status-right segment is separated visually.

set -u

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"

# No cache yet -> empty summary
[ -d "$cache_dir" ] || exit 0

working=0
waiting=0
done_=0

# nullglob so an empty dir doesn't iterate "*.status" literal
shopt -s nullglob
for f in "$cache_dir"/*.status; do
  state=$(cat "$f" 2>/dev/null || echo "")
  case "$state" in
    working) working=$((working + 1)) ;;
    waiting) waiting=$((waiting + 1)) ;;
    done)    done_=$((done_ + 1)) ;;
    *)       ;;
  esac
done

out=""
[ "$working" -gt 0 ] && out+="⚡ $working "
[ "$waiting" -gt 0 ] && out+="⏸ $waiting "
[ "$done_"   -gt 0 ] && out+="✓ $done_ "

printf '%s' "$out"
```

- [ ] **Step 2: Apply to home dir**

```bash
chezmoi diff ~/.config/tmux/scripts/cockpit/summary.sh
chezmoi apply ~/.config/tmux/scripts/cockpit/summary.sh
ls -la ~/.config/tmux/scripts/cockpit/summary.sh
```

Expected: 実行ビット付きで配置されている。

- [ ] **Step 3: Verify aggregation**

Run:
```bash
cache="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
mkdir -p "$cache"

# Empty case
rm -f "$cache"/summary-test_*.status
result=$(~/.config/tmux/scripts/cockpit/summary.sh)
echo "[empty] result='$result'"
```

Expected: `[empty] result=''`

```bash
# Mixed case
printf 'working' > "$cache/summary-test_%1.status"
printf 'working' > "$cache/summary-test_%2.status"
printf 'waiting' > "$cache/summary-test_%3.status"
printf 'done'    > "$cache/summary-test_%4.status"
result=$(~/.config/tmux/scripts/cockpit/summary.sh)
echo "[mixed] result='$result'"
```

Expected: `[mixed] result='⚡ 2 ⏸ 1 ✓ 1 '`

```bash
# Only done
rm -f "$cache"/summary-test_*.status
printf 'done' > "$cache/summary-test_%1.status"
result=$(~/.config/tmux/scripts/cockpit/summary.sh)
echo "[done-only] result='$result'"
```

Expected: `[done-only] result='✓ 1 '`

- [ ] **Step 4: Cleanup**

```bash
rm -f "${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes/summary-test_"*.status
```

- [ ] **Step 5: Commit**

```bash
git add dot_config/tmux/scripts/cockpit/executable_summary.sh
git -c commit.gpgsign=false commit -m "feat(tmux): add cockpit summary script for status-right

Aggregates per-pane state files into '⚡ N ⏸ M ✓ K ' string.
Output is empty when no state files exist.

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §6.5"
```

---

## Task 4: Switch `status.conf` from `claude-status-count.sh` to `cockpit/summary.sh`

**Files:**
- Modify: `dot_config/tmux/conf/status.conf:11`

- [ ] **Step 1: Show current contents**

Run:
```bash
sed -n '10,12p' dot_config/tmux/conf/status.conf
```

Expected line 11:
```
set -g status-right "#(~/.config/tmux/scripts/claude-status-count.sh)#(~/.config/tmux/scripts/claude-branch.sh '#{pane_current_path}')%H:%M "
```

- [ ] **Step 2: Edit**

Replace line 11 of `dot_config/tmux/conf/status.conf` with:

```
set -g status-right "#(~/.config/tmux/scripts/cockpit/summary.sh)#(~/.config/tmux/scripts/claude-branch.sh '#{pane_current_path}')%H:%M "
```

(Only the `claude-status-count.sh` portion changes to `cockpit/summary.sh`; `claude-branch.sh` and `%H:%M` are unchanged.)

- [ ] **Step 3: Apply and reload tmux**

```bash
chezmoi apply ~/.config/tmux/conf/status.conf
tmux source-file ~/.config/tmux/tmux.conf 2>/dev/null || true
```

- [ ] **Step 4: Verify status-right rendering**

Run inside an existing tmux session:
```bash
tmux refresh-client -S
tmux show-options -g status-right
```

Expected: 表示文字列が `cockpit/summary.sh` を呼ぶ形になっている。手元に状態ファイルが無ければ status bar には branch+時刻のみが出る。

- [ ] **Step 5: Commit**

```bash
git add dot_config/tmux/conf/status.conf
git -c commit.gpgsign=false commit -m "refactor(tmux): swap pgrep status-count for cockpit summary

status-right no longer polls 'pgrep claude' every 5 s. Reads the hook-
driven cache file aggregate instead.

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §6.5"
```

---

## Task 5: Delete `claude-status-count.sh`

**Files:**
- Delete: `dot_config/tmux/scripts/executable_claude-status-count.sh`

- [ ] **Step 1: Remove the chezmoi-managed script**

```bash
git rm dot_config/tmux/scripts/executable_claude-status-count.sh
```

- [ ] **Step 2: Apply to remove from target**

```bash
chezmoi diff
chezmoi apply
ls ~/.config/tmux/scripts/claude-status-count.sh 2>&1
```

Expected: `No such file or directory`（chezmoi が削除済み）。

- [ ] **Step 3: Commit**

```bash
git -c commit.gpgsign=false commit -m "chore(tmux): remove pgrep-based claude-status-count.sh

Replaced by cockpit/summary.sh in the previous commit.

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §4.3"
```

---

## Task 6: Add `cockpit/prune.sh` (orphan cleanup)

**Files:**
- Create: `dot_config/tmux/scripts/cockpit/executable_prune.sh`

- [ ] **Step 1: Write the script**

Create `dot_config/tmux/scripts/cockpit/executable_prune.sh` with content:

```bash
#!/usr/bin/env bash
# Remove cache files for tmux panes that no longer exist.
# Safe to run any time; idempotent.

set -u

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
[ -d "$cache_dir" ] || exit 0

command -v tmux >/dev/null 2>&1 || exit 0

# Build the set of currently-live "<session>_<pane-id>" keys
live=$(tmux list-panes -a -F '#{session_name}_#{pane_id}' 2>/dev/null) || exit 0

# Compare each cached file's basename (minus .status suffix) to live set
shopt -s nullglob
for f in "$cache_dir"/*.status; do
  base=$(basename "$f" .status)
  if ! printf '%s\n' "$live" | grep -Fxq -- "$base"; then
    rm -f -- "$f"
  fi
done

# Also clean up the (currently unused) sessions/ dir defensively
sessions_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/sessions"
if [ -d "$sessions_dir" ]; then
  live_sessions=$(tmux list-sessions -F '#{session_name}' 2>/dev/null) || exit 0
  for f in "$sessions_dir"/*.status; do
    base=$(basename "$f" .status)
    if ! printf '%s\n' "$live_sessions" | grep -Fxq -- "$base"; then
      rm -f -- "$f"
    fi
  done
fi

exit 0
```

- [ ] **Step 2: Apply**

```bash
chezmoi apply ~/.config/tmux/scripts/cockpit/prune.sh
```

- [ ] **Step 3: Verify orphan removal**

Run:
```bash
cache="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
mkdir -p "$cache"

# Create an orphan (no tmux session named "ghost-session")
printf 'working' > "$cache/ghost-session_%99.status"

# Create a real entry tied to an actual live tmux pane
tmux new-session -d -s prune-test -x 80 -y 24
real_pane=$(tmux display-message -p -t prune-test '#{pane_id}')
printf 'done' > "$cache/prune-test_${real_pane}.status"

# Run prune
~/.config/tmux/scripts/cockpit/prune.sh

# Check results
echo "--- after prune ---"
ls "$cache"
```

Expected: `ghost-session_%99.status` は削除済、`prune-test_<real_pane>.status` は残っている。

- [ ] **Step 4: Cleanup**

```bash
tmux kill-session -t prune-test
rm -f "${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes/prune-test_"*.status
```

- [ ] **Step 5: Commit**

```bash
git add dot_config/tmux/scripts/cockpit/executable_prune.sh
git -c commit.gpgsign=false commit -m "feat(tmux): add cockpit prune script for orphan cache files

Removes per-pane state files for panes that tmux no longer reports.
Defensively cleans the reserved sessions/ dir as well.

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §6.7"
```

---

## Task 7: Run `prune.sh` on tmux server start

**Files:**
- Modify: `dot_config/tmux/tmux.conf`

- [ ] **Step 1: Show current contents**

```bash
cat dot_config/tmux/tmux.conf
```

- [ ] **Step 2: Edit**

Append to `dot_config/tmux/tmux.conf` (after the existing `run -b` line for tpm, end of file):

```
# Cockpit: prune orphan state cache once per server start
run -b '~/.config/tmux/scripts/cockpit/prune.sh'
```

The full file ends up:

```tmux
# tmux entrypoint: モジュール化された設定を順に source する。
# プラグインは末尾で TPM が読み込む。
source-file ~/.config/tmux/conf/options.conf
source-file ~/.config/tmux/conf/bindings.conf
source-file ~/.config/tmux/conf/status.conf
source-file ~/.config/tmux/conf/claude.conf
source-file ~/.config/tmux/conf/plugins.conf

# TPM (tmux-plugins/tpm) — 末尾必須
if "test ! -d ~/.config/tmux/plugins/tpm" \
   "display 'TPM not installed; run ~/.config/tmux/scripts/tpm-bootstrap.sh'"
run -b '~/.config/tmux/plugins/tpm/tpm'

# Cockpit: prune orphan state cache once per server start
run -b '~/.config/tmux/scripts/cockpit/prune.sh'
```

- [ ] **Step 3: Apply and reload**

```bash
chezmoi apply ~/.config/tmux/tmux.conf
tmux source-file ~/.config/tmux/tmux.conf
```

Reload alone won't trigger `run -b` differently from a fresh server start, but tmux silently re-runs the line. No visible output is normal.

- [ ] **Step 4: Commit**

```bash
git add dot_config/tmux/tmux.conf
git -c commit.gpgsign=false commit -m "feat(tmux): run cockpit/prune.sh on server start

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §6.7"
```

---

## Task 8: Append cache cleanup to `claude-kill-session.sh`

**Files:**
- Modify: `dot_config/tmux/scripts/executable_claude-kill-session.sh`

- [ ] **Step 1: Show current end of file**

```bash
sed -n '25,40p' dot_config/tmux/scripts/executable_claude-kill-session.sh
```

Expected: ends with the worktree-removal `if` block (around line 32).

- [ ] **Step 2: Append cache cleanup**

Append these lines to the very end of `dot_config/tmux/scripts/executable_claude-kill-session.sh`:

```bash

# Cockpit: drop cached state files for the killed session
cache="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit"
rm -f "$cache/sessions/${session}.status" "$cache/panes/${session}"_*.status 2>/dev/null || true
```

The blank line above `# Cockpit:` keeps the diff self-contained.

- [ ] **Step 3: Apply**

```bash
chezmoi apply ~/.config/tmux/scripts/claude-kill-session.sh
```

- [ ] **Step 4: Verify (manual smoke)**

Run:
```bash
cache="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
mkdir -p "$cache"

# Create dummy state for a session-name-only kill scenario.
session="claude-cleanup-fake"
printf 'done' > "$cache/${session}_%1.status"

# Inline the cleanup snippet (mirroring the appended lines)
cache_root="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit"
rm -f "$cache_root/sessions/${session}.status" "$cache_root/panes/${session}"_*.status 2>/dev/null || true

ls "$cache_root/panes/" 2>/dev/null | grep -F "$session" || echo "[cleanup OK] no matching files"
```

Expected: `[cleanup OK] no matching files`

- [ ] **Step 5: Commit**

```bash
git add dot_config/tmux/scripts/executable_claude-kill-session.sh
git -c commit.gpgsign=false commit -m "feat(tmux): drop cockpit cache on claude-kill-session

Removes panes/<session>_*.status and sessions/<session>.status when
killing a claude-* session.

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §6.6"
```

---

## Task 9: Add `cockpit/switcher.sh` (hierarchical fzf)

**Files:**
- Create: `dot_config/tmux/scripts/cockpit/executable_switcher.sh`

- [ ] **Step 1: Write the script**

Create `dot_config/tmux/scripts/cockpit/executable_switcher.sh` with content:

```bash
#!/usr/bin/env bash
# Hierarchical fzf switcher: lists every tmux session/window/pane with its
# claude-cockpit state badge. Acts on the selected scope:
#   Enter   -> switch-client (+ select-window / select-pane as needed)
#   Ctrl-X  -> kill the selected scope (worktree-aware for sessions)
#   Ctrl-R  -> reload (re-runs prune + redraws)

set -u

if ! command -v fzf >/dev/null 2>&1; then
  tmux display-message "fzf required (paru -S fzf)"
  exit 1
fi

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"

# Run prune first so orphans don't show up.
~/.config/tmux/scripts/cockpit/prune.sh 2>/dev/null || true

state_for_pane() {
  local s="$1" p="$2" f
  f="$cache_dir/${s}_${p}.status"
  [ -f "$f" ] && cat "$f" 2>/dev/null || true
}

badge() {
  case "${1:-}" in
    working) printf '⚡ working' ;;
    waiting) printf '⏸ waiting' ;;
    done)    printf '✓ done'    ;;
    *)       printf ''          ;;
  esac
}

# Aggregate per-session state from its panes.
state_for_session() {
  local s="$1"
  local has_w=0 has_q=0 has_d=0
  while IFS= read -r p; do
    case "$(state_for_pane "$s" "$p")" in
      working) has_w=1 ;;
      waiting) has_q=1 ;;
      done)    has_d=1 ;;
    esac
  done < <(tmux list-panes -t "$s" -s -F '#{pane_id}' 2>/dev/null)
  if   [ "$has_w" -eq 1 ]; then echo "working"
  elif [ "$has_q" -eq 1 ]; then echo "waiting"
  elif [ "$has_d" -eq 1 ]; then echo "done"
  fi
}

# Build the rendered tree. Each output line is tab-separated with metadata
# columns hidden via fzf's --with-nth=5..  Format:
#   <kind>\t<session>\t<window-idx>\t<pane-id>\t<display>
build_lines() {
  local sname session_state w_idx w_name p_id p_path p_state
  while IFS= read -r sname; do
    [ -z "$sname" ] && continue
    session_state=$(state_for_session "$sname")
    printf 'S\t%s\t\t\t%-30s  %s\n' "$sname" "$sname" "$(badge "$session_state")"

    while IFS=$'\t' read -r w_idx w_name; do
      printf 'W\t%s\t%s\t\t  window:%s %s\n' "$sname" "$w_idx" "$w_idx" "$w_name"

      while IFS=$'\t' read -r p_id p_path; do
        p_state=$(state_for_pane "$sname" "$p_id")
        printf 'P\t%s\t%s\t%s\t    pane:%s  cwd=%s    %s\n' \
          "$sname" "$w_idx" "$p_id" "$p_id" "$p_path" "$(badge "$p_state")"
      done < <(tmux list-panes -t "${sname}:${w_idx}" -F '#{pane_id}'$'\t''#{pane_current_path}' 2>/dev/null)

    done < <(tmux list-windows -t "$sname" -F '#{window_index}'$'\t''#{window_name}' 2>/dev/null)
  done < <(tmux list-sessions -F '#{session_name}' 2>/dev/null | sort)
}

selection=$(build_lines | fzf \
  --prompt='cockpit> ' \
  --height=100% \
  --delimiter=$'\t' \
  --with-nth='5..' \
  --expect='ctrl-x,ctrl-r' \
  --header='enter=switch  ctrl-x=kill  ctrl-r=reload') || exit 0

key=$(printf '%s\n' "$selection" | sed -n '1p')
row=$(printf '%s\n' "$selection" | sed -n '2p')
[ -z "$row" ] && exit 0

kind=$(printf '%s' "$row"   | cut -f1)
sname=$(printf '%s' "$row"  | cut -f2)
w_idx=$(printf '%s' "$row"  | cut -f3)
p_id=$(printf '%s' "$row"   | cut -f4)

case "$key" in
  ctrl-r)
    exec ~/.config/tmux/scripts/cockpit/switcher.sh
    ;;
  ctrl-x)
    case "$kind" in
      P)
        tmux kill-pane -t "$p_id"
        ;;
      W)
        tmux confirm-before -p "kill window ${sname}:${w_idx}? (y/n) " \
          "kill-window -t ${sname}:${w_idx}"
        ;;
      S)
        # delegate to existing claude-kill-session.sh which removes worktree too
        case "$sname" in
          claude-*)
            tmux confirm-before -p "kill claude session and worktree? (y/n) " \
              "run-shell '~/.config/tmux/scripts/claude-kill-session.sh'"
            ;;
          *)
            tmux confirm-before -p "kill session ${sname}? (y/n) " \
              "kill-session -t ${sname}"
            ;;
        esac
        ;;
    esac
    ;;
  *)
    case "$kind" in
      S) tmux switch-client -t "$sname" ;;
      W) tmux switch-client -t "$sname" \; select-window -t "${sname}:${w_idx}" ;;
      P) tmux switch-client -t "$sname" \; select-window -t "${sname}:${w_idx}" \; select-pane -t "$p_id" ;;
    esac
    ;;
esac
```

- [ ] **Step 2: Apply**

```bash
chezmoi apply ~/.config/tmux/scripts/cockpit/switcher.sh
```

- [ ] **Step 3: Manual verification**

```bash
# Spawn a couple of sessions
tmux new-session -d -s cockpit-sw-a -x 80 -y 24
tmux new-session -d -s cockpit-sw-b -x 80 -y 24

# Seed some fake state
cache="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
mkdir -p "$cache"
pa=$(tmux display-message -p -t cockpit-sw-a '#{pane_id}')
pb=$(tmux display-message -p -t cockpit-sw-b '#{pane_id}')
printf 'working' > "$cache/cockpit-sw-a_${pa}.status"
printf 'done'    > "$cache/cockpit-sw-b_${pb}.status"

# Run switcher in the foreground (must be inside an existing tmux client)
tmux display-popup -E ~/.config/tmux/scripts/cockpit/switcher.sh
```

Expected:
- popup に session-A/B が表示され、A は `⚡ working`、B は `✓ done` の badge が出る
- Enter で対象に switch-client できる
- `Ctrl-X` で pane を kill できる（既に空の場合 session ごと閉じる）

- [ ] **Step 4: Cleanup**

```bash
tmux kill-session -t cockpit-sw-a 2>/dev/null
tmux kill-session -t cockpit-sw-b 2>/dev/null
rm -f "${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes/cockpit-sw-"*.status
```

- [ ] **Step 5: Commit**

```bash
git add dot_config/tmux/scripts/cockpit/executable_switcher.sh
git -c commit.gpgsign=false commit -m "feat(tmux): add cockpit hierarchical fzf switcher

session/window/pane tree with state badges. Enter switches, Ctrl-X
kills (worktree-aware for claude-* sessions), Ctrl-R reloads.

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §6.3"
```

---

## Task 10: Replace `claude_table.s` with switcher and add `claude_table.N`

**Files:**
- Modify: `dot_config/tmux/conf/bindings.conf:59`
- Modify: `dot_config/tmux/conf/bindings.conf` (append within the claude_table block)

- [ ] **Step 1: Show current claude_table block**

```bash
sed -n '45,63p' dot_config/tmux/conf/bindings.conf
```

Expected line 59:
```
bind -T claude_table s display-popup -E "~/.config/tmux/scripts/claude-pick-session.sh"
```

- [ ] **Step 2: Edit**

Replace the `claude_table.s` line in `dot_config/tmux/conf/bindings.conf` to call the new switcher:

```
bind -T claude_table s display-popup -E "~/.config/tmux/scripts/cockpit/switcher.sh"
```

Then append a new bind inside the same `claude_table` block (right after the existing `s` line, before the `k` line):

```
# N: done な pane に inbox 順で循環ジャンプ
bind -T claude_table N run-shell "~/.config/tmux/scripts/cockpit/next-ready.sh"
```

- [ ] **Step 3: Apply and reload**

```bash
chezmoi apply ~/.config/tmux/conf/bindings.conf
tmux source-file ~/.config/tmux/tmux.conf
```

- [ ] **Step 4: Verify the binds**

```bash
tmux list-keys -T claude_table | grep -E '^bind-key -T claude_table [sN] '
```

Expected output (order may vary):
```
bind-key    -T claude_table s          display-popup -E ~/.config/tmux/scripts/cockpit/switcher.sh
bind-key    -T claude_table N          run-shell ~/.config/tmux/scripts/cockpit/next-ready.sh
```

- [ ] **Step 5: Commit**

```bash
git add dot_config/tmux/conf/bindings.conf
git -c commit.gpgsign=false commit -m "feat(tmux): rebind claude_table.s/N to cockpit scripts

claude_table.s -> hierarchical switcher
claude_table.N -> next-ready jump (new)

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §7.2"
```

---

## Task 11: Delete `claude-pick-session.sh`

**Files:**
- Delete: `dot_config/tmux/scripts/executable_claude-pick-session.sh`

- [ ] **Step 1: Remove**

```bash
git rm dot_config/tmux/scripts/executable_claude-pick-session.sh
```

- [ ] **Step 2: Apply**

```bash
chezmoi apply
ls ~/.config/tmux/scripts/claude-pick-session.sh 2>&1
```

Expected: `No such file or directory`

- [ ] **Step 3: Commit**

```bash
git -c commit.gpgsign=false commit -m "chore(tmux): remove flat claude-pick-session.sh

Replaced by cockpit/switcher.sh in the previous commits.

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §4.3"
```

---

## Task 12: Add `cockpit/next-ready.sh`

**Files:**
- Create: `dot_config/tmux/scripts/cockpit/executable_next-ready.sh`

- [ ] **Step 1: Write the script**

Create `dot_config/tmux/scripts/cockpit/executable_next-ready.sh` with content:

```bash
#!/usr/bin/env bash
# Switch to the next pane whose cockpit state is "done", in inbox order:
#   session-name asc -> window-index asc -> pane-index asc.
# Cycles past the current pane; wraps around to the first done if needed.

set -u

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
[ -d "$cache_dir" ] || { tmux display-message "no ready claude pane"; exit 0; }

# Build inbox-ordered list of "session\twindow_idx\tpane_id\tpane_idx" rows
# whose cached state is "done".
build_done_list() {
  local sname w_idx p_id p_idx state
  while IFS= read -r sname; do
    [ -z "$sname" ] && continue
    while IFS= read -r w_idx; do
      while IFS=$'\t' read -r p_id p_idx; do
        state=$(cat "$cache_dir/${sname}_${p_id}.status" 2>/dev/null || echo "")
        [ "$state" = "done" ] && printf '%s\t%s\t%s\t%s\n' "$sname" "$w_idx" "$p_id" "$p_idx"
      done < <(tmux list-panes -t "${sname}:${w_idx}" -F '#{pane_id}'$'\t''#{pane_index}' 2>/dev/null | sort -t$'\t' -k2,2n)
    done < <(tmux list-windows -t "$sname" -F '#{window_index}' 2>/dev/null | sort -n)
  done < <(tmux list-sessions -F '#{session_name}' 2>/dev/null | sort)
}

list=$(build_done_list)
if [ -z "$list" ]; then
  tmux display-message "no ready claude pane"
  exit 0
fi

# Identify the current pane to find the "next" in the list.
cur_pane=$(tmux display-message -p '#{pane_id}')

# Find the index of cur_pane in $list; pick the line after it (cycling).
target=$(awk -v cur="$cur_pane" '
  { rows[NR] = $0; ids[NR] = $3 }
  END {
    n = NR
    if (n == 0) exit 1
    pick = 1
    for (i = 1; i <= n; i++) {
      if (ids[i] == cur) { pick = (i % n) + 1; break }
    }
    print rows[pick]
  }
' <<<"$list")

[ -z "$target" ] && exit 0

t_session=$(printf '%s' "$target" | cut -f1)
t_window=$(printf '%s'  "$target" | cut -f2)
t_pane=$(printf '%s'    "$target" | cut -f3)

tmux switch-client -t "$t_session" \
  \; select-window -t "${t_session}:${t_window}" \
  \; select-pane -t "$t_pane"
```

- [ ] **Step 2: Apply**

```bash
chezmoi apply ~/.config/tmux/scripts/cockpit/next-ready.sh
```

- [ ] **Step 3: Manual verification**

```bash
tmux new-session -d -s nr-a -x 80 -y 24
tmux new-session -d -s nr-b -x 80 -y 24
pa=$(tmux display-message -p -t nr-a '#{pane_id}')
pb=$(tmux display-message -p -t nr-b '#{pane_id}')

cache="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
mkdir -p "$cache"
printf 'done' > "$cache/nr-a_${pa}.status"
printf 'done' > "$cache/nr-b_${pb}.status"

# Inside an existing tmux client, the binding `prefix + C, N` will
# actually switch panes. Out-of-client invocation is a no-op for
# switch-client, but the script returns 0 and prints nothing.
echo "expected behavior:"
echo "  - currently in nr-a -> jump to nr-b"
echo "  - currently in nr-b -> wrap to nr-a"
echo "  - no done state    -> 'no ready claude pane' message"
```

- [ ] **Step 4: Verify "no done" branch**

```bash
rm -f "${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes/nr-"*.status
~/.config/tmux/scripts/cockpit/next-ready.sh
```

Expected: tmux の display-message に `no ready claude pane` が短時間出る（exit 0）。

- [ ] **Step 5: Cleanup**

```bash
tmux kill-session -t nr-a 2>/dev/null
tmux kill-session -t nr-b 2>/dev/null
rm -f "${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes/nr-"*.status
```

- [ ] **Step 6: Commit**

```bash
git add dot_config/tmux/scripts/cockpit/executable_next-ready.sh
git -c commit.gpgsign=false commit -m "feat(tmux): add cockpit next-ready jump script

Cycles through done-state panes in inbox order (session asc, window
idx asc, pane idx asc). Wraps around. No-op message when none are done.

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §6.4"
```

---

## Task 13: Document smoke tests in `manage_claude.md`

**Files:**
- Modify: `docs/manage_claude.md`

- [ ] **Step 1: Show end of file**

```bash
tail -20 docs/manage_claude.md
```

- [ ] **Step 2: Append smoke test section**

Append to the end of `docs/manage_claude.md`:

```markdown

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
```

- [ ] **Step 3: Commit**

```bash
git add docs/manage_claude.md
git -c commit.gpgsign=false commit -m "docs(manage_claude): add cockpit state tracking smoke tests

8-step manual verification flow matching spec §9.

Refs spec 2026-04-30-claude-cockpit-state-tracking-design.md §9"
```

---

## Task 14: End-to-end smoke test

**Files:** none — verification only.

- [ ] **Step 1: Apply everything that's pending**

```bash
chezmoi diff
chezmoi apply
tmux source-file ~/.config/tmux/tmux.conf
```

Expected: diff is empty (everything from Tasks 1–13 already applied).

- [ ] **Step 2: Walk through `manage_claude.md` smoke tests 1–8**

Open `docs/manage_claude.md` and run through the new smoke section step by step. Record any deviations.

- [ ] **Step 3: If all 8 pass, mark plan done**

No further commits required. If any step fails, file a follow-up task in `docs/todos.md` describing the failure mode and the spec section it violates.

---

## Notes for Reviewers / Future Agents

- **Spec file vs settings template**: spec §4.3 / §10.1 mention `dot_config/claude/settings.json.tmpl` 想定だったが、実環境では plain `settings.json`。プランは plain JSON を編集する前提で書いた。
- **Hook ordering**: Claude は同じイベント配列に複数 entry がある場合、登録順に直列実行する。state hook を **末尾に追加** することで既存（observe.sh / claude-notify-hook.sh）の遅延を絶対に増やさない（state hook は数 ms で完了）。
- **Cache layout 互換**: `panes/<S>_<P>.status` は上流 `tmux-agent-status` と同じ。将来上流に乗り換える場合、`~/.cache/claude-cockpit/panes` → `~/.cache/tmux-agent-status/panes` の symlink で状態を継承可能。
- **Atomic write の理由**: `summary.sh` は 5 秒ごとに status-right から発火する。書き込み中の中間状態（空ファイル等）を読まないよう `printf > tmp && mv` で隔離している。
- **`set -u` だけ採用、`set -e` は採用しない**: hook script は **絶対に Claude を止めない** 契約。`set -e` を入れると tmux 不在環境などで早期 exit してしまうので明示的に外している。
