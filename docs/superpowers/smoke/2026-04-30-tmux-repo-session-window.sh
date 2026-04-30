#!/usr/bin/env bash
# F-6 smoke test runbook — interactive checklist.
#
# Plan: docs/superpowers/plans/2026-04-30-tmux-repo-session-window.md (Task 6)
# Spec: docs/superpowers/specs/2026-04-30-tmux-repo-session-window-design.md
#
# Run this from a terminal of your choice. The script prompts you to perform
# each manual key chord / fzf selection, then auto-verifies post-conditions.
#
# Usage:
#   bash docs/superpowers/smoke/2026-04-30-tmux-repo-session-window.sh
#   REPO=/path/to/chezmoi  OTHER_REPO=/path/to/another/repo  bash ./...
#
# Notes:
#   - Step 1 calls `tmux kill-server` which terminates ALL existing tmux
#     sessions, including any Claude Code running inside tmux. Run this script
#     from a terminal that is OUTSIDE the tmux you intend to test (or accept
#     that the script's own tmux session will end at Step 1).
#   - The script is idempotent in spirit: each step prompts before acting and
#     skips destructive operations on user 'N' response.

set -uo pipefail

REPO="${REPO:-$HOME/.local/share/chezmoi}"
OTHER_REPO="${OTHER_REPO:-}"
PASS=0
FAIL=0

hr()    { printf '\n%s\n' '────────────────────────────────────────'; }
step()  { hr; printf '▶ %s\n' "$*"; }
note()  { printf '  • %s\n' "$*"; }
ok()    { printf '  ✓ %s\n' "$*"; PASS=$((PASS+1)); }
ng()    { printf '  ✗ %s\n' "$*"; FAIL=$((FAIL+1)); }
ask()   { printf '\n  ? %s\n  > Press Enter when done (Ctrl-C to abort): ' "$*"; read -r _; }
yesno() { local p="$1" r; read -rp "  ? $p (y/N): " r; [[ "$r" =~ ^[Yy] ]]; }

# Pre-flight ------------------------------------------------------------------
step "Pre-flight"
[ -d "$REPO" ] || { ng "REPO not found: $REPO"; exit 1; }
command -v tmux >/dev/null || { ng "tmux not installed"; exit 1; }
command -v chezmoi >/dev/null || { ng "chezmoi not installed"; exit 1; }
ok "REPO=$REPO"
ok "tmux=$(tmux -V)"
ok "chezmoi present"

# Step 0: chezmoi apply -------------------------------------------------------
step "Step 0: chezmoi apply (live system update)"
cd "$REPO"
note "F-6 affected files:"
note "  ~/.config/tmux/scripts/tmux-claude-new.sh"
note "  ~/.config/tmux/scripts/claude-kill-session.sh"
note "  ~/.config/tmux/conf/bindings.conf"
echo
note "Showing chezmoi diff (truncated to 80 lines):"
chezmoi diff \
  ~/.config/tmux/scripts/tmux-claude-new.sh \
  ~/.config/tmux/scripts/claude-kill-session.sh \
  ~/.config/tmux/conf/bindings.conf 2>&1 | head -80 || true
echo
if yesno "Proceed with chezmoi apply for those 3 paths?"; then
  if chezmoi apply \
       ~/.config/tmux/scripts/tmux-claude-new.sh \
       ~/.config/tmux/scripts/claude-kill-session.sh \
       ~/.config/tmux/conf/bindings.conf
  then
    ok "chezmoi apply ok"
  else
    ng "chezmoi apply failed"; exit 1
  fi
else
  ng "chezmoi apply skipped — aborting"; exit 1
fi

# Step 1: tmux clean restart --------------------------------------------------
step "Step 1: tmux clean restart"
note "WARNING: this terminates ALL existing tmux sessions (kill-server)."
note ""
note "Choose ONE option:"
note "  A) Script runs kill-server + creates a fresh 'scratch' session"
note "  B) You handle restart manually, then resume this script"
echo
if yesno "Option A — let the script kill-server + create scratch?"; then
  tmux kill-server 2>/dev/null || true
  sleep 1
  if tmux new-session -d -s scratch -c "$REPO"; then
    ok "scratch session created"
  else
    ng "failed to create scratch session"; exit 1
  fi
  if tmux source-file ~/.config/tmux/tmux.conf 2>/dev/null; then
    ok "tmux config sourced"
  else
    ng "source-file failed (check ~/.config/tmux/tmux.conf for syntax errors)"
  fi
  note ""
  note "Now in another terminal: tmux attach -t scratch"
  ask "Attached to scratch session?"
else
  note "Run yourself:"
  note "  tmux kill-server"
  note "  tmux new-session -d -s scratch -c $REPO"
  note "  tmux source-file ~/.config/tmux/tmux.conf"
  note "  tmux attach -t scratch"
  ask "Done? scratch session attached?"
fi

# Step 2: first window --------------------------------------------------------
step "Step 2: First window via prefix + C → n"
note "Inside the scratch tmux pane, ensure cwd is $REPO:"
note "  cd $REPO"
note ""
note "Press the chord:  prefix + C → n"
note "fzf popup appears. Select 'develop' (or any existing branch)."
note ""
note "Expected: new session 'chezmoi' with window named after the branch,"
note "          left pane = work shell, right pane = claude."
ask "Done?"
sessions=$(tmux list-sessions -F '#S' 2>/dev/null || true)
note "Active tmux sessions:"
printf '%s\n' "$sessions" | sed 's/^/      /'
if printf '%s\n' "$sessions" | grep -Fxq chezmoi; then
  ok "session 'chezmoi' exists"
else
  ng "session 'chezmoi' missing"
fi
windows=$(tmux list-windows -t chezmoi -F '#W' 2>/dev/null || true)
note "Windows in chezmoi:"
printf '%s\n' "$windows" | sed 's/^/      /'
[ -n "$windows" ] && ok "≥1 window present in chezmoi" || ng "no windows in chezmoi"
managed=$(tmux show-options -w -t chezmoi: -v '@claude-managed' 2>/dev/null || echo)
[ "$managed" = yes ] && ok "@claude-managed=yes set on the window" \
                     || ng "@claude-managed not yes (got: '$managed')"

# Step 3: second window -------------------------------------------------------
step "Step 3: Second window in same session"
note "Press: prefix + C → n → select a different branch (e.g. feat/foo, or"
note "       enter a new branch name in fzf to create one)."
ask "Done?"
windows=$(tmux list-windows -t chezmoi -F '#W' 2>/dev/null || true)
n=$(printf '%s\n' "$windows" | grep -c .)
note "Windows in chezmoi ($n total):"
printf '%s\n' "$windows" | sed 's/^/      /'
[ "$n" -ge 2 ] && ok "≥2 windows present" || ng "expected ≥2 windows, got $n"

# Step 4: switcher hierarchy --------------------------------------------------
step "Step 4: cockpit switcher hierarchy"
note "Press: prefix + C → s   (cockpit switcher popup opens)"
note "Each sub-check is asked separately — answer based on what you actually see."
note "Press Ctrl-C / N at any kill prompt — DO NOT actually kill anything yet."
echo
yesno "4a) Does 'chezmoi' show ≥2 windows in the tree?" \
  && ok "4a chezmoi window list" \
  || ng "4a chezmoi windows not visible"
yesno "4b) Is 'scratch' session also visible in the tree?" \
  && ok "4b scratch visible" \
  || ng "4b scratch missing"
yesno "4c) Selecting a chezmoi:<branch> window + Ctrl-X — prompt says 'kill claude window ... and worktree?'" \
  && ok "4c claude-managed window prompt" \
  || ng "4c managed window prompt wrong"
yesno "4d) Selecting 'scratch' (session row) + Ctrl-X — prompt says 'kill session scratch? (worktrees kept)'" \
  && ok "4d non-claude session prompt" \
  || ng "4d session prompt wrong"

# Step 5: cross-repo isolation ------------------------------------------------
step "Step 5: Cross-repo session isolation"
if [ -z "$OTHER_REPO" ]; then
  read -rp "  Path to another git repo for testing (Enter to skip): " OTHER_REPO
fi
if [ -n "$OTHER_REPO" ] && [ -d "$OTHER_REPO" ]; then
  base=$(basename "$OTHER_REPO")
  note "In a tmux pane: cd $OTHER_REPO; press prefix + C → n; select a branch."
  ask "Done?"
  sessions=$(tmux list-sessions -F '#S' 2>/dev/null || true)
  note "Sessions now:"
  printf '%s\n' "$sessions" | sed 's/^/      /'
  printf '%s\n' "$sessions" | grep -Fxq "$base" && ok "session '$base' exists" \
                                                 || ng "session '$base' missing"
  printf '%s\n' "$sessions" | grep -Fxq chezmoi && ok "chezmoi session still independent" \
                                                || ng "chezmoi session affected"
else
  note "Skipped (no OTHER_REPO provided)"
fi

# Step 6: window-level kill ---------------------------------------------------
step "Step 6: Window-level kill"
note "Switch to chezmoi:develop window in your tmux client."
note "Press: prefix + C → k → y"
ask "Done?"
windows=$(tmux list-windows -t chezmoi -F '#W' 2>/dev/null || true)
note "Remaining chezmoi windows:"
printf '%s\n' "$windows" | sed 's/^/      /'
printf '%s\n' "$windows" | grep -Fxq develop && ng "develop window still present" \
                                              || ok "develop window gone"
worktrees=$(git -C "$REPO" worktree list 2>/dev/null || true)
echo "$worktrees" | grep -Fq "chezmoi-develop" \
  && ng "worktree chezmoi-develop still present" \
  || ok "worktree chezmoi-develop removed"

# Step 7: last-window kill destroys session -----------------------------------
step "Step 7: Last-window kill auto-destroys session"
note "On the last remaining chezmoi window, press: prefix + C → k → y"
ask "Done?"
sessions=$(tmux list-sessions -F '#S' 2>/dev/null || true)
note "Sessions now:"
printf '%s\n' "$sessions" | sed 's/^/      /'
printf '%s\n' "$sessions" | grep -Fxq chezmoi && ng "chezmoi session still present" \
                                              || ok "chezmoi session auto-destroyed"

# Step 8: cockpit summary -----------------------------------------------------
step "Step 8: cockpit summary still works"
note "Direct check: run summary.sh and confirm exit 0."
direct=$(~/.config/tmux/scripts/cockpit/summary.sh 2>&1) && summary_rc=0 || summary_rc=$?
note "summary.sh stdout: '${direct}'"
if [ "$summary_rc" -eq 0 ]; then
  ok "summary.sh exits 0"
else
  ng "summary.sh exit=$summary_rc — script-level regression"
fi

# Auto-detect: are there any *.status files in the cockpit cache? If not,
# claude isn't currently being tracked anywhere, so an empty summary is the
# CORRECT behavior — no visual confirmation needed.
cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
if ls "$cache_dir"/*.status >/dev/null 2>&1; then
  note ""
  note "Cache has .status files — visual check applies:"
  note "tmux status-right should include  ⚡ N ⏸ M ✓ K  (counts)."
  if yesno "status-right shows the ⚡/⏸/✓ counts?"; then
    ok "cockpit summary visual ok"
  else
    ng "cockpit summary regression (cache present but counts not displayed)"
  fi
else
  ok "no claude tracked in cache — empty summary is correct (visual N/A)"
fi

# Summary --------------------------------------------------------------------
hr
printf '  PASS: %d    FAIL: %d\n' "$PASS" "$FAIL"
hr
if [ "$FAIL" -eq 0 ]; then
  cat <<EOM

🎉 ALL CHECKS PASSED

Next: mark docs/todos.md F-6 task 5 as [x] and commit.

  cd $REPO
  # Edit docs/todos.md inside the F-6 block:
  #   - [ ] Task 5: 8-step 手動スモークの実機通し
  # becomes:
  #   - [x] Task 5: 8-step 手動スモークの実機通し (完了 $(date +%Y-%m-%d))
  git add docs/todos.md
  git commit -m 'docs(todos): F-6 tmux session/window redesign complete'

Optional cleanup (after smoke confirms green):
  git worktree remove $REPO/.worktrees/tmux-session-window-redesign
  git branch -d tmux-session-window-redesign

EOM
else
  cat <<EOM

⚠️  SOME CHECKS FAILED

Diagnostics:
  - /tmp/tmux-claude-new.log  (script execution log)
  - tmux show-options -w '@claude-managed'  (per-window flag)
  - tmux display-message -p '#W / #{pane_current_command}'  (state inspection)
  - git worktree list  (worktree state)

Failed step → likely culprit:
  Step 2/3/5 → Task 1 (tmux-claude-new.sh)
  Step 6/7   → Task 2 (claude-kill-session.sh)
  Step 4     → Task 3 (bindings.conf) or cockpit/switcher.sh
  Step 8     → cockpit/summary.sh (cache layout regression)

Append the failure summary + reproduction notes to the F-6 block in
docs/todos.md so the failure context is preserved.

EOM
fi
