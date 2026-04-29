# tmux Claude Cockpit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild tmux as a Claude Code cockpit with worktree-bound sessions, TPM-managed plugins, dynamic pane state visualization, and a key-table for Claude operations.

**Architecture:** Modularize `tmux.conf` into `conf/*.conf` and add helper shell scripts under `scripts/`. Use TPM (`tmux-plugins/tpm`) for `tmux-resurrect` / `tmux-continuum` / `tmux-yank`. The new-claude-session logic lives in a portable shell script `tmux-claude-new.sh`, with a 1-line zsh wrapper for interactive use and the same script reused by the popup picker. Bootstrap TPM via `chezmoi run_once`.

**Tech Stack:** tmux 3.x, TPM, bash/zsh, fzf, chezmoi, git worktrees.

**Refinement vs spec:** Spec § 5.8 puts `tmux_claude_new` purely in `tmux.zsh`. To enable reuse from tmux popup scripts, the core logic moves to `dot_config/tmux/scripts/tmux-claude-new.sh` and `tmux_claude_new()` becomes a thin zsh wrapper. Same end-user behavior.

**Source repo:** `/home/kiyama/.local/share/chezmoi`. All paths below are repo-relative unless prefixed `~/`.

**Verification convention:**
- `chezmoi diff` after every task touching dotfiles to preview render
- `tmux source-file ~/.config/tmux/tmux.conf` to check syntax (run only after `chezmoi apply` or directly against staged file)
- `chezmoi apply` is **NOT** run from this plan automatically — user runs it when comfortable

---

## File Structure (target)

```
dot_config/tmux/
├── tmux.conf                              # MODIFY: source-file declarations only
├── conf/
│   ├── options.conf                       # CREATE: set -g options
│   ├── bindings.conf                      # CREATE: bind directives (existing + new prefix+s/g)
│   ├── status.conf                        # CREATE: status-left/right + interval
│   ├── claude.conf                        # CREATE: pane-border-format + claude_table bindings
│   └── plugins.conf                       # CREATE: TPM @plugin declarations + plugin opts
└── scripts/                               # all files use `executable_` prefix
    ├── executable_tmux-claude-new.sh      # CREATE: core "new claude session" logic
    ├── executable_claude-status-count.sh  # CREATE: pgrep claude → status-right token
    ├── executable_claude-branch.sh        # CREATE: git branch → status-right token
    ├── executable_claude-pick-branch.sh   # CREATE: fzf popup → tmux-claude-new
    ├── executable_claude-pick-session.sh  # CREATE: fzf popup → switch-client (claude-* only)
    ├── executable_claude-respawn-pane.sh  # CREATE: kill+restart claude in current pane
    ├── executable_claude-kill-session.sh  # CREATE: kill session + git worktree remove
    └── executable_tpm-bootstrap.sh        # CREATE: idempotent TPM clone + install_plugins

dot_config/zsh/rc/my_plugins/tmux.zsh      # MODIFY: append tmux_claude_new wrapper
.chezmoiscripts/run_once_all_os.sh.cmd.tmpl # MODIFY: add tmux + fzf to paru packages, call tpm-bootstrap.sh
.chezmoiignore                              # MODIFY: add .local/share/tmux/resurrect/
```

**Why this split:**
- `tmux.conf` becomes a 6-line declaration file; future growth lands in topic-specific files (each <200 lines).
- `claude.conf` and `claude-*.sh` co-locate everything Claude-specific so future tweaks touch one folder.
- Helper scripts in `scripts/` are reused by both bindings (popups) and `tmux_claude_new` (core), avoiding duplicated logic.

---

## Task 1: Modularize tmux.conf skeleton (no behavior change yet)

Create the new directory structure and slim `tmux.conf` to a source-only shell. All existing options/bindings copied verbatim into `conf/options.conf` and `conf/bindings.conf` so behavior is preserved.

**Files:**
- Create: `dot_config/tmux/conf/options.conf`
- Create: `dot_config/tmux/conf/bindings.conf`
- Create: `dot_config/tmux/conf/status.conf`
- Create: `dot_config/tmux/conf/claude.conf` (empty for now)
- Create: `dot_config/tmux/conf/plugins.conf` (empty for now)
- Modify: `dot_config/tmux/tmux.conf`

- [ ] **Step 1: Create `dot_config/tmux/conf/options.conf` with all `set/setw` directives from current `tmux.conf`**

```tmux
# tmux options (general). Sourced from tmux.conf.

# prefix を C-t に変更
set -g prefix C-t
unbind C-b

# マウス操作を有効にする
set-option -g mouse on
set -g mouse on

# tmux を true color で表示
set-option -g default-terminal "screen-256color"
set-option -ga terminal-overrides ",xterm-256color:Tc"

# esc 遅延をなくす
set-option -s escape-time 0

# ウィンドウ番号を 1 始まりに
set-option -g base-index 1

# pane-border-status を上に
set -g pane-border-status top

# focus events: pane border 動的色や resurrect が利用
set -g focus-events on

# vi mode keys for copy mode
setw -g mode-keys vi
```

- [ ] **Step 2: Create `dot_config/tmux/conf/bindings.conf` with all existing `bind` directives**

```tmux
# tmux key bindings. Sourced from tmux.conf.

# マウスホイールでコピーモード（既存）
bind -n WheelUpPane if-shell -F -t = "#{mouse_any_flag}" "send-keys -M" "if -Ft= '#{pane_in_mode}' 'send-keys -M' 'copy-mode -e'"
bind -n WheelDownPane select-pane -t= \; send-keys -M

# vim 風 pane 移動
bind h select-pane -L
bind j select-pane -D
bind k select-pane -U
bind l select-pane -R

# window 作成・移動
bind  M-c new-window -c "#{pane_current_path}"
bind  J next-window
bind  K previous-window

# session 作成・移動
bind  M-C new-session
bind  L switch-client -n
bind  H switch-client -p

# pane 分割（カレント PWD を継承）
bind | split-window -h -c "#{pane_current_path}"
bind _ split-window -v -c "#{pane_current_path}"

# pane kill
bind q kill-pane

# copy mode (vi)
bind -T copy-mode-vi v send -X begin-selection
bind -T copy-mode-vi V send -X select-line
bind -T copy-mode-vi C-v send -X rectangle-toggle
bind -T copy-mode-vi y send -X copy-selection
bind -T copy-mode-vi Y send -X copy-line
```

- [ ] **Step 3: Create `dot_config/tmux/conf/status.conf` with existing status-left only**

```tmux
# tmux status bar config. Sourced from tmux.conf.

set -g status-left "#[fg=colour108,bg=colour237,bold] [#S:#I:#P] "
```

- [ ] **Step 4: Create empty `dot_config/tmux/conf/claude.conf` and `dot_config/tmux/conf/plugins.conf` placeholders**

`dot_config/tmux/conf/claude.conf`:
```tmux
# Claude Code 専用設定（pane border 動的色、claude_table 切替）。
# 後続タスクで埋める。
```

`dot_config/tmux/conf/plugins.conf`:
```tmux
# TPM プラグイン宣言。tmux.conf の最後に source-file される必要がある。
# 後続タスクで埋める。
```

- [ ] **Step 5: Replace `dot_config/tmux/tmux.conf` with source-only declarations**

Full new content:
```tmux
# tmux entrypoint: モジュール化された設定を順に source する。
# プラグインは末尾で TPM が読み込む。
source-file ~/.config/tmux/conf/options.conf
source-file ~/.config/tmux/conf/bindings.conf
source-file ~/.config/tmux/conf/status.conf
source-file ~/.config/tmux/conf/claude.conf
source-file ~/.config/tmux/conf/plugins.conf

# TPM (tmux-plugins/tpm) — 末尾必須
if "test ! -d ~/.tmux/plugins/tpm" \
   "display 'TPM not installed; run ~/.config/tmux/scripts/tpm-bootstrap.sh'"
run -b '~/.tmux/plugins/tpm/tpm'
```

- [ ] **Step 6: Verify chezmoi can render the templates**

Run: `chezmoi diff dot_config/tmux/`
Expected: shows the new `conf/*.conf` files as additions and `tmux.conf` shrunk to declarations. No template errors.

- [ ] **Step 7: Verify tmux can parse the new conf chain**

Run (read-only check, does not affect running server):
```bash
tmux -L plan-test -f /home/kiyama/.local/share/chezmoi/dot_config/tmux/tmux.conf new-session -d ';' kill-server 2>&1
```
Expected: empty output or only `[exited]`. **No** `error in config file` lines.

If this fails, fix syntax before committing.

- [ ] **Step 8: Commit**

```bash
git add dot_config/tmux/
git commit -m "refactor(tmux): split tmux.conf into modular conf/* files"
```

---

## Task 2: Add status helper scripts (claude count + branch)

Two pure-bash scripts that emit short tokens for `status-right`.

**Files:**
- Create: `dot_config/tmux/scripts/executable_claude-status-count.sh`
- Create: `dot_config/tmux/scripts/executable_claude-branch.sh`

- [ ] **Step 1: Create `executable_claude-status-count.sh`**

```bash
#!/usr/bin/env bash
# stdout: "[claude:N] " when N >= 1, else empty.
n=$(pgrep -c -x claude 2>/dev/null || echo 0)
[ "$n" -gt 0 ] && printf "[claude:%d] " "$n"
exit 0
```

Note: `-x` matches exact command name `claude`, avoiding partial matches like `claude-helper`.

- [ ] **Step 2: Create `executable_claude-branch.sh`**

```bash
#!/usr/bin/env bash
# usage: claude-branch.sh <pane-current-path>
# stdout: "[<branch>] " when inside a git repo with a current branch, else empty.
[ -z "$1" ] && exit 0
b=$(git -C "$1" branch --show-current 2>/dev/null)
[ -n "$b" ] && printf "[%s] " "$b"
exit 0
```

- [ ] **Step 3: Verify chezmoi renders with executable bit**

Quick sanity check: `chezmoi diff dot_config/tmux/scripts/` should show the two new files; the rendered target paths drop the `executable_` prefix → `~/.config/tmux/scripts/claude-status-count.sh` and `~/.config/tmux/scripts/claude-branch.sh`, mode 0755.

- [ ] **Step 4: Manual smoke test**

Run the source-form scripts directly:
```bash
bash dot_config/tmux/scripts/executable_claude-status-count.sh
# Expected: empty (no claude running) OR "[claude:1] " if a claude session is up

bash dot_config/tmux/scripts/executable_claude-branch.sh "$(pwd)"
# Expected: "[main] " (or whatever branch you're on)
```

- [ ] **Step 5: Commit**

```bash
git add dot_config/tmux/scripts/executable_claude-status-count.sh dot_config/tmux/scripts/executable_claude-branch.sh
git commit -m "feat(tmux): add claude-status-count and claude-branch helpers"
```

---

## Task 3: Wire status-right to use helper scripts

Update `status.conf` to call the helpers and bump status interval to 5 s.

**Files:**
- Modify: `dot_config/tmux/conf/status.conf`

- [ ] **Step 1: Replace `status.conf` content**

Full new content:
```tmux
# tmux status bar config. Sourced from tmux.conf.

# 5 秒間隔（claude プロセス監視のため少し短め）
set -g status-interval 5

# 左: セッション識別子 + プレフィックス入力中インジケータ
set -g status-left "#[fg=colour108,bg=colour237,bold] [#S:#I:#P] #{?client_prefix,⌘ ,}"

# 右: claude プロセス数 + 現ペインの git branch + 時刻
set -g status-right "#(~/.config/tmux/scripts/claude-status-count.sh)#(~/.config/tmux/scripts/claude-branch.sh #{pane_current_path})%H:%M "
set -g status-right-length 80
```

- [ ] **Step 2: Verify syntax with the same dry-run pattern as Task 1**

Run:
```bash
tmux -L plan-test -f /home/kiyama/.local/share/chezmoi/dot_config/tmux/tmux.conf new-session -d ';' kill-server 2>&1
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add dot_config/tmux/conf/status.conf
git commit -m "feat(tmux): wire status-right with claude count, git branch, time"
```

---

## Task 4: Add Claude pane-border dynamic color in claude.conf

`claude` を実行中のペインだけ枠を黄色にする。

**Files:**
- Modify: `dot_config/tmux/conf/claude.conf`

- [ ] **Step 1: Replace `claude.conf` content**

Full new content:
```tmux
# Claude Code 専用設定（pane border 動的色、claude_table 切替の宣言）

# pane title を表示しつつ、claude プロセス実行中は黄色枠で目立たせる
set -g pane-border-format "#[fg=#{?#{==:#{pane_current_command},claude},yellow,colour244}] #{pane_title} [#{pane_current_command}]"

# claude_table 切替バインドは bindings.conf 側で定義（責務分離）。
# このファイルは「Claude 関連の見た目と key-table 設定」を集約する。
```

- [ ] **Step 2: Syntax dry-run**

Run:
```bash
tmux -L plan-test -f /home/kiyama/.local/share/chezmoi/dot_config/tmux/tmux.conf new-session -d ';' kill-server 2>&1
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add dot_config/tmux/conf/claude.conf
git commit -m "feat(tmux): highlight claude panes with yellow border"
```

---

## Task 5: Add core `tmux-claude-new.sh` script

Single source of truth for "create worktree + tmux session + start claude". Reused by zsh function and popup picker.

**Files:**
- Create: `dot_config/tmux/scripts/executable_tmux-claude-new.sh`

- [ ] **Step 1: Create the script**

```bash
#!/usr/bin/env bash
# usage: tmux-claude-new.sh <branch>
# - normalizes branch name to a session name "claude-<safe>"
# - creates git worktree at <repo-root>-<safe> if missing
# - creates 2-pane tmux session if missing (left: shell, right: claude --continue --fork-session)
# - attaches or switch-clients

set -euo pipefail

branch="${1:-}"
if [ -z "$branch" ]; then
  echo "usage: tmux-claude-new.sh <branch>" >&2
  exit 1
fi

safe="${branch//\//-}"
session="claude-${safe}"

repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || {
  echo "tmux-claude-new: not inside a git repo" >&2
  exit 1
}
worktree="${repo_root}-${safe}"

# Ensure worktree exists
if [ ! -d "$worktree" ]; then
  git worktree add "$worktree" "$branch" || {
    echo "tmux-claude-new: failed to create worktree at $worktree" >&2
    exit 1
  }
fi

# Ensure session exists
if ! tmux has-session -t "$session" 2>/dev/null; then
  tmux new-session -d -s "$session" -c "$worktree"
  tmux split-window -h -t "$session" -c "$worktree"
  tmux select-pane -t "${session}.0" -T work
  tmux select-pane -t "${session}.1" -T claude
  tmux send-keys -t "${session}.1" "claude --continue --fork-session" Enter
fi

# Attach or switch
if [ -n "${TMUX:-}" ]; then
  tmux switch-client -t "$session"
else
  tmux attach-session -t "$session"
fi
```

- [ ] **Step 2: Manual smoke test (dry-form)**

Without applying chezmoi (script lives in source tree):
```bash
bash dot_config/tmux/scripts/executable_tmux-claude-new.sh
# Expected: usage message on stderr, exit 1
```

- [ ] **Step 3: Commit**

```bash
git add dot_config/tmux/scripts/executable_tmux-claude-new.sh
git commit -m "feat(tmux): add tmux-claude-new.sh core launcher"
```

---

## Task 6: Add zsh wrapper `tmux_claude_new`

The wrapper is a 3-line function that delegates to the script. Keep existing `tmux` and `tmux_claude` untouched.

**Files:**
- Modify: `dot_config/zsh/rc/my_plugins/tmux.zsh` (append at end)

- [ ] **Step 1: Append to `dot_config/zsh/rc/my_plugins/tmux.zsh`**

Add at the end of the file (after the existing `tmux_claude` function):

```zsh

tmux_claude_new() {
  # Thin wrapper around the portable shell script so tmux popups can call the
  # same logic. Keeps tmux / tmux_claude untouched.
  ~/.config/tmux/scripts/tmux-claude-new.sh "$@"
}
```

- [ ] **Step 2: Verify zsh syntax**

Run:
```bash
zsh -n dot_config/zsh/rc/my_plugins/tmux.zsh && echo OK
```
Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add dot_config/zsh/rc/my_plugins/tmux.zsh
git commit -m "feat(zsh): add tmux_claude_new wrapper for worktree-bound sessions"
```

---

## Task 7: Add Claude popup helper scripts

Four small bash scripts used by `claude_table` bindings.

**Files:**
- Create: `dot_config/tmux/scripts/executable_claude-pick-branch.sh`
- Create: `dot_config/tmux/scripts/executable_claude-pick-session.sh`
- Create: `dot_config/tmux/scripts/executable_claude-respawn-pane.sh`
- Create: `dot_config/tmux/scripts/executable_claude-kill-session.sh`

- [ ] **Step 1: Create `executable_claude-pick-branch.sh`**

```bash
#!/usr/bin/env bash
# tmux popup: pick a git branch via fzf, then call tmux-claude-new.sh.
set -euo pipefail

if ! command -v fzf >/dev/null 2>&1; then
  echo "fzf is required (install via paru -S fzf)" >&2
  read -r -p "Press Enter to close..."
  exit 1
fi

branch=$(git for-each-ref --format='%(refname:short)' refs/heads | fzf --prompt='claude branch> ' --height=100%) || exit 0
[ -z "$branch" ] && exit 0

exec ~/.config/tmux/scripts/tmux-claude-new.sh "$branch"
```

- [ ] **Step 2: Create `executable_claude-pick-session.sh`**

```bash
#!/usr/bin/env bash
# tmux popup: pick a session whose name matches "claude-*" and switch-client to it.
set -euo pipefail

if ! command -v fzf >/dev/null 2>&1; then
  echo "fzf is required (install via paru -S fzf)" >&2
  read -r -p "Press Enter to close..."
  exit 1
fi

target=$(tmux list-sessions -F '#S' 2>/dev/null | grep '^claude-' | fzf --prompt='claude session> ' --height=100%) || exit 0
[ -z "$target" ] && exit 0

tmux switch-client -t "$target"
```

- [ ] **Step 3: Create `executable_claude-respawn-pane.sh`**

```bash
#!/usr/bin/env bash
# Restart claude in the current session's claude pane.
# Strategy: find a pane in the current session whose pane_current_command is
# 'claude'; if found, respawn-pane -k and start a fresh claude --continue.
# If none found, do it in the current pane.
set -euo pipefail

session=$(tmux display-message -p '#S')
target=$(tmux list-panes -t "$session" -F '#{pane_id} #{pane_current_command}' \
  | awk '$2 == "claude" {print $1; exit}')

if [ -z "$target" ]; then
  target=$(tmux display-message -p '#{pane_id}')
fi

tmux respawn-pane -k -t "$target"
tmux send-keys -t "$target" "claude --continue" Enter
```

- [ ] **Step 4: Create `executable_claude-kill-session.sh`**

```bash
#!/usr/bin/env bash
# Kill the current "claude-*" session and remove its corresponding worktree.
# Caller (binding) wraps this in confirm-before, so we proceed unconditionally.
set -euo pipefail

session=$(tmux display-message -p '#S')

case "$session" in
  claude-*) ;;
  *)
    tmux display-message "claude-kill-session: refusing on non-claude session ($session)"
    exit 1
    ;;
esac

# Worktree path is computed the same way tmux-claude-new.sh did:
# repo_root + "-" + session_suffix
suffix="${session#claude-}"
pane_path=$(tmux display-message -p '#{pane_current_path}')
repo_root=$(git -C "$pane_path" rev-parse --show-toplevel 2>/dev/null || true)

# Detach clients on this session, then kill it, then remove worktree if found.
tmux switch-client -n 2>/dev/null || true
tmux kill-session -t "$session"

if [ -n "$repo_root" ]; then
  worktree="${repo_root}-${suffix}"
  if [ -d "$worktree" ]; then
    git -C "$repo_root" worktree remove "$worktree" --force 2>/dev/null \
      || tmux display-message "kept worktree $worktree (remove manually)"
  fi
fi
```

- [ ] **Step 5: Smoke-test the scripts that don't require a live tmux server**

```bash
bash dot_config/tmux/scripts/executable_claude-pick-branch.sh < /dev/null
# Expected: fzf opens, you press Esc → exits cleanly with no error.
# (If fzf is missing, you see the install hint.)
```

- [ ] **Step 6: Commit**

```bash
git add dot_config/tmux/scripts/executable_claude-pick-branch.sh \
        dot_config/tmux/scripts/executable_claude-pick-session.sh \
        dot_config/tmux/scripts/executable_claude-respawn-pane.sh \
        dot_config/tmux/scripts/executable_claude-kill-session.sh
git commit -m "feat(tmux): add Claude popup helpers (pick/respawn/kill)"
```

---

## Task 8: Wire `claude_table` key-table into bindings.conf

Add `prefix + C` to enter `claude_table`, and the 5 sub-keys (c/n/r/s/k). Also add `prefix + s` (session picker) and `prefix + g` (worktree picker).

**Files:**
- Modify: `dot_config/tmux/conf/bindings.conf` (append)

- [ ] **Step 1: Append to `dot_config/tmux/conf/bindings.conf`**

Add after existing bindings:

```tmux

# --- セッション / worktree ピッカー（直下） ---

# prefix + s : claude-* に絞らない全 session ピッカー
bind s display-popup -E "tmux list-sessions -F '#S' | fzf --prompt='session> ' --height=100% | xargs -r tmux switch-client -t"

# prefix + g : 現リポジトリの git worktree から選んで cd（新ペインで作業継続）
bind g display-popup -E "git worktree list --porcelain | awk '/^worktree /{print \$2}' | fzf --prompt='worktree> ' --height=100%"

# --- claude_table（二段プレフィックス: prefix + C, <key>） ---

bind C switch-client -T claude_table

# c: 現ペインで claude --continue を流す
bind -T claude_table c send-keys "claude --continue" Enter

# n: ブランチ選択 → tmux-claude-new
bind -T claude_table n display-popup -E "~/.config/tmux/scripts/claude-pick-branch.sh"

# r: アクティブ claude ペインを kill → 再起動
bind -T claude_table r run-shell "~/.config/tmux/scripts/claude-respawn-pane.sh"

# s: claude-* セッション ピッカー
bind -T claude_table s display-popup -E "~/.config/tmux/scripts/claude-pick-session.sh"

# k: 現セッション + worktree を削除（要確認）
bind -T claude_table k confirm-before -p "kill claude session and worktree? (y/n) " "run-shell '~/.config/tmux/scripts/claude-kill-session.sh'"
```

- [ ] **Step 2: Syntax dry-run**

Run:
```bash
tmux -L plan-test -f /home/kiyama/.local/share/chezmoi/dot_config/tmux/tmux.conf new-session -d ';' kill-server 2>&1
```
Expected: no errors.

- [ ] **Step 3: Verify the key-table is registered (server-aware check)**

Spin up a throw-away server, source the conf, list keys:
```bash
tmux -L plan-test -f /home/kiyama/.local/share/chezmoi/dot_config/tmux/tmux.conf new-session -d \
  ';' list-keys -T claude_table \
  ';' kill-server 2>&1
```
Expected: 5 lines listing the `c`, `n`, `r`, `s`, `k` bindings under `claude_table`.

- [ ] **Step 4: Commit**

```bash
git add dot_config/tmux/conf/bindings.conf
git commit -m "feat(tmux): wire prefix+s/g pickers and claude_table key-table"
```

---

## Task 9: Add TPM bootstrap script + plugins.conf

Idempotent TPM clone + `install_plugins`, then declare the 4 plugins.

**Files:**
- Create: `dot_config/tmux/scripts/executable_tpm-bootstrap.sh`
- Modify: `dot_config/tmux/conf/plugins.conf`

- [ ] **Step 1: Create `executable_tpm-bootstrap.sh`**

```bash
#!/usr/bin/env bash
# Idempotent TPM bootstrap. Safe to call from chezmoi run_once or by hand.
set -euo pipefail

TPM_DIR="$HOME/.tmux/plugins/tpm"

if [ ! -d "$TPM_DIR" ]; then
  echo "[tpm-bootstrap] cloning TPM into $TPM_DIR"
  git clone --depth 1 https://github.com/tmux-plugins/tpm "$TPM_DIR"
fi

if [ -x "$TPM_DIR/bin/install_plugins" ]; then
  echo "[tpm-bootstrap] running install_plugins"
  "$TPM_DIR/bin/install_plugins"
fi
```

- [ ] **Step 2: Replace `dot_config/tmux/conf/plugins.conf`**

Full new content:
```tmux
# TPM プラグイン宣言。
# 注: tmux.conf 末尾で source されること。最後に `run -b ~/.tmux/plugins/tpm/tpm` が走る。

set -g @plugin 'tmux-plugins/tpm'
set -g @plugin 'tmux-plugins/tmux-resurrect'
set -g @plugin 'tmux-plugins/tmux-continuum'
set -g @plugin 'tmux-plugins/tmux-yank'

# resurrect: pane 内容まで含めて保存
set -g @resurrect-capture-pane-contents 'on'
set -g @resurrect-strategy-nvim 'session'

# continuum: 15 分間隔の自動保存 + 起動時自動復元
set -g @continuum-save-interval '15'
set -g @continuum-restore 'on'
```

- [ ] **Step 3: Smoke test the bootstrap script (it is idempotent)**

```bash
bash dot_config/tmux/scripts/executable_tpm-bootstrap.sh
# Expected: clones TPM if missing, then runs install_plugins.
# If TPM already present, skips clone but re-runs install_plugins.
ls ~/.tmux/plugins/
# Expected: contains tpm, tmux-resurrect, tmux-continuum, tmux-yank
```

If you do not want this side effect during plan execution, skip step 3 and rely on Task 10's chezmoi run_once integration.

- [ ] **Step 4: Commit**

```bash
git add dot_config/tmux/scripts/executable_tpm-bootstrap.sh dot_config/tmux/conf/plugins.conf
git commit -m "feat(tmux): add TPM bootstrap and plugin declarations"
```

---

## Task 10: Integrate TPM bootstrap into chezmoi run_once + add tmux/fzf to paru packages

**Files:**
- Modify: `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl`

- [ ] **Step 1: Add `tmux` and `fzf` to the paru `PACKAGES` heredoc**

Inside the existing `read -r -d '' PACKAGES <<EOF` block (currently ending around line 79), insert two lines (alphabetical-ish placement):

Find this block:
```sh
read -r -d '' PACKAGES <<EOF
ttf-plemoljp-bin
fcitx5
neovim
rofi
wezterm
bat
fd
lsd
skim
navi
onefetch
ripgrep
tealdeer
pueue
zoxide
EOF
```

Replace with:
```sh
read -r -d '' PACKAGES <<EOF
ttf-plemoljp-bin
fcitx5
neovim
rofi
wezterm
bat
fd
lsd
skim
navi
onefetch
ripgrep
tealdeer
pueue
zoxide
tmux
fzf
EOF
```

- [ ] **Step 2: Append TPM bootstrap call at the end of the file**

Append after the `for package in ...` loop that installs paru packages:

```sh

# tmux: TPM bootstrap (idempotent)
if [ -x "$HOME/.config/tmux/scripts/tpm-bootstrap.sh" ]; then
  "$HOME/.config/tmux/scripts/tpm-bootstrap.sh"
fi
```

- [ ] **Step 3: Verify chezmoi can render the template**

```bash
chezmoi diff .chezmoiscripts/run_once_all_os.sh.cmd.tmpl
```
Expected: rendered diff shows `tmux` and `fzf` added to PACKAGES, plus the TPM bootstrap call appended at the end. No template errors.

If you are not on Manjaro, the rendered script short-circuits with `exit 0`; the diff still shows the source-side change.

- [ ] **Step 4: Commit**

```bash
git add .chezmoiscripts/run_once_all_os.sh.cmd.tmpl
git commit -m "chore(chezmoi): bootstrap TPM and install tmux/fzf via run_once"
```

---

## Task 11: Update `.chezmoiignore` for resurrect data + final integration check

resurrect/continuum write to `~/.local/share/tmux/resurrect/`. Mark it as ignored so chezmoi doesn't try to manage generated state.

**Files:**
- Modify: `.chezmoiignore`

- [ ] **Step 1: Append the ignore rule**

Append to `.chezmoiignore` (after the `.local/share/rye/*` block, alphabetical-ish):

```
# tmux-resurrect / continuum 自動保存（生成物 — chezmoi 管理対象外）
.local/share/tmux/*
```

- [ ] **Step 2: Verify chezmoi recognizes the ignore**

```bash
chezmoi managed | grep 'tmux/resurrect' && echo 'still managed (BAD)' || echo 'not managed (correct)'
```
Expected: `not managed (correct)` (since we never sourced it).

`chezmoi diff` should not contain any `.local/share/tmux/` entries.

- [ ] **Step 3: Final pre-apply review**

```bash
chezmoi diff | head -200
```
Expected: a clean diff showing:
- `dot_config/tmux/tmux.conf` shrunk to source declarations
- `dot_config/tmux/conf/*.conf` added (5 files)
- `dot_config/tmux/scripts/*.sh` added (8 executable files, mode 0755)
- `dot_config/zsh/rc/my_plugins/tmux.zsh` gains `tmux_claude_new`
- `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` adds tmux/fzf packages and TPM bootstrap call
- `.chezmoiignore` adds tmux state ignore

- [ ] **Step 4: Commit (.chezmoiignore only)**

```bash
git add .chezmoiignore
git commit -m "chore(chezmoi): ignore tmux state directory (resurrect/continuum output)"
```

- [ ] **Step 5: Run apply and verify (USER-CONTROLLED — pause for confirmation)**

This is the only step that mutates `~/`. Run it manually when you're ready:

```bash
chezmoi apply
~/.config/tmux/scripts/tpm-bootstrap.sh   # in case run_once has already executed
tmux kill-server 2>/dev/null || true
tmux                                       # main session should come up as before
```

Acceptance criteria from the spec (§ 9.1) — check each:

- [ ] `tmux source ~/.config/tmux/tmux.conf` (inside a running tmux) prints no errors
- [ ] `tmux list-keys -T claude_table` lists 5 entries
- [ ] `prefix + s` opens an fzf session picker
- [ ] `prefix + C, n` opens an fzf branch picker
- [ ] Running `tmux_claude_new feat/test` (after creating branch `feat/test`) creates `claude-feat-test` session and starts `claude --continue --fork-session` in the right pane
- [ ] The right pane's border turns yellow (claude is the current command)
- [ ] `status-right` shows `[claude:1] [<branch>] HH:MM`
- [ ] `tmux kill-server && tmux` restores the previous layout (continuum)
- [ ] Existing `tmux` and `tmux_claude` shell functions still work unchanged

If anything fails, do **not** force-fix in this final task — open a follow-up task or revert the offending earlier commit.

- [ ] **Step 6: Cleanup test worktree (optional)**

If you ran the acceptance test with `feat/test`:

```bash
tmux kill-session -t claude-feat-test 2>/dev/null || true
git worktree remove "$(git rev-parse --show-toplevel)-feat-test" 2>/dev/null || true
git branch -D feat/test 2>/dev/null || true
```

---

## Self-Review Notes

Spec coverage map:
- § 3.1 ハイブリッド粒度 → Task 5/6 (`tmux-claude-new.sh` + zsh wrapper, leaves `tmux`/`tmux_claude` untouched)
- § 3.2 プラグイン管理 → Task 9/10 (TPM bootstrap + plugins.conf + run_once)
- § 4.1 ファイルレイアウト → Task 1 (skeleton) + subsequent tasks fill it in
- § 4.2 設定モジュール化 → Task 1 (slim tmux.conf to source declarations)
- § 5.1 options.conf → Task 1
- § 5.2 bindings.conf → Task 1 (existing) + Task 8 (new prefix+s/g + claude_table)
- § 5.3 plugins.conf → Task 9
- § 5.4 status.conf → Task 3
- § 5.5 claude.conf → Task 4 (border) + Task 8 (key-table; lives in bindings.conf for responsibility separation, see note below)
- § 5.6 claude-status-count.sh → Task 2
- § 5.7 claude-branch.sh → Task 2
- § 5.8 tmux_claude_new → Task 5 (script) + Task 6 (zsh wrapper)
- § 5.9 run_once → Task 10
- § 6.1 data flow → Task 5/6 implements; Task 11 verifies
- § 6.2 永続化復元 → Task 9 + Task 11 verification
- § 6.3 ペイン状態可視化 → Task 4 + Task 3
- § 7 error handling → covered by `set -euo pipefail` in scripts and explicit checks
- § 8 ブートストラップ → Task 10 + Task 11 step 5
- § 9 verification → Task 11 step 5

Spec deviation acknowledged: claude key-table bindings live in `bindings.conf`, not `claude.conf`. Reason: `bindings.conf` is the single home for `bind` directives, so future tab-completion / grep-by-key works predictably. `claude.conf` keeps only the visual side (pane-border-format). This is a more readable split than the spec sketched.

Open questions from spec § 10 are unchanged:
- fzf availability on non-Manjaro: the popup scripts emit an install hint and exit cleanly; non-Manjaro users see this as a discoverable failure mode rather than a silent break.
- `tmux-yank` clipboard backend: defer to verification in Task 11 step 5.
