# Wired notification click actions — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Claude Code の常駐 popup を「左クリック=tmux focus / 中=closeall / 右=この popup を close」に再定義する。

**Architecture:** 既存の単一 hook スクリプト `claude-notify-sound.sh` を 3 ファイルに分割 (hook 入口 / 音再生 / popup dispatcher)。`notify-send --print-id --wait --action=default=Focus` で popup の存続中ずっとプロセスを保持し、ActionInvoked シグナルを stdout 経由で受信、左クリック時に `tmux switch-client + select-pane` を実行して `gdbus CloseNotification` で popup を明示 close。`wired.ron` の shortcuts を組み替えて中ボタンを closeall、右ボタンを単 close にマップ。

**Tech Stack:** Bash, libnotify (`notify-send`), wired-notify, gdbus, tmux, chezmoi, jq

**Spec reference:** `docs/superpowers/specs/2026-04-30-wired-click-actions-design.md` (committed in `5fa6fc0`)

---

## File structure

| Path (chezmoi source) | Deployed path | Action |
|---|---|---|
| `dot_local/bin/executable_claude-notify-hook.sh` | `~/.local/bin/claude-notify-hook.sh` | **Create** (hook 入口) |
| `dot_local/bin/executable_claude-notify-sound.sh` | `~/.local/bin/claude-notify-sound.sh` | **Modify** (sound 再生のみに縮小) |
| `dot_local/bin/executable_claude-notify-dispatch.sh` | `~/.local/bin/claude-notify-dispatch.sh` | **Create** (popup + action ループ) |
| `dot_config/claude/settings.json` | `~/.config/claude/settings.json` | **Modify** (hook command path 差し替え) |
| `dot_config/wired/wired.ron` | `~/.config/wired/wired.ron` | **Modify** (shortcuts 組み替え) |
| `docs/manage_claude.md` | — (chezmoiignore) | **Modify** (§5.7 通知表に click 行を追加) |
| `docs/claude_tmux_cheatsheet.md` | — (chezmoiignore) | **Modify** (§5 通知表に click 行を追加) |
| `docs/todos.md` | — (chezmoiignore) | **Modify** (F-3 v1 達成分を `[x]` 化、未達分を残置) |

すべてリポジトリ root: `/home/kiyama/.local/share/chezmoi/` 起点の相対パス。

---

## Task 1: dispatcher スクリプトを作成

**Files:**
- Create: `dot_local/bin/executable_claude-notify-dispatch.sh`

- [ ] **Step 1: `dot_local/bin/executable_claude-notify-dispatch.sh` を新規作成**

ファイル内容:

```bash
#!/usr/bin/env bash
# Claude Code popup dispatcher. Lives alongside the libnotify popup,
# reads `notify-send --print-id --wait` output, and on left-click
# ("default" action) focuses the originating tmux pane and dismisses
# the popup explicitly via CloseNotification.
#
# Right-click is wired to `notification_close` in wired.ron and is
# handled entirely by wired -> notify-send returns without an action
# line, and this script no-ops.
#
# Inputs (env, set by claude-notify-hook.sh):
#   CLAUDE_NOTIFY_TITLE / BODY / URGENCY
#   CLAUDE_NOTIFY_SESSION_ID
#   CLAUDE_NOTIFY_TMUX_PANE / TMUX_SESSION
set -euo pipefail

title="${CLAUDE_NOTIFY_TITLE:-Claude Code}"
body="${CLAUDE_NOTIFY_BODY:-}"
urgency="${CLAUDE_NOTIFY_URGENCY:-normal}"
session_id="${CLAUDE_NOTIFY_SESSION_ID:-}"
tmux_pane="${CLAUDE_NOTIFY_TMUX_PANE:-}"
tmux_session="${CLAUDE_NOTIFY_TMUX_SESSION:-}"

command -v notify-send >/dev/null 2>&1 || exit 0

# notify-send --print-id --wait stdout:
#   line 1: notification id (uint32, always)
#   line 2: action key when ActionInvoked fires (e.g. "default")
# Close-without-action -> only line 1 is printed.
mapfile -t lines < <(
  notify-send \
    --app-name=ClaudeCode \
    --urgency="$urgency" \
    --expire-time=0 \
    --action=default=Focus \
    --hint=string:x-claude-session:"${session_id:-unknown}" \
    --print-id \
    --wait \
    -- "$title" "$body" 2>/dev/null
) || exit 0

notif_id="${lines[0]:-}"
action_key="${lines[1]:-}"

# Right-click / closeall / timeout -> nothing more to do.
[[ "$action_key" != "default" ]] && exit 0

# === focus the originating tmux pane ===
if [[ -n "$tmux_pane" && -n "$tmux_session" ]] \
    && command -v tmux >/dev/null 2>&1 \
    && tmux has-session -t "$tmux_session" 2>/dev/null; then
  tmux switch-client -t "$tmux_session" \; select-pane -t "$tmux_pane" \
    >/dev/null 2>&1 || true
else
  # TODO(F-3): bare terminal fallback via wmctrl / swaymsg using cwd
  # TODO(F-3): auto-reopen tmux session if it was killed, then resume claude
  logger -t claude-notify-dispatch \
    "focus skipped: no tmux context (sid=${session_id:-?})" || true
fi

# === auto-dismiss popup (FDO spec doesn't auto-close on ActionInvoked) ===
if [[ -n "$notif_id" ]] && command -v gdbus >/dev/null 2>&1; then
  gdbus call --session \
    --dest=org.freedesktop.Notifications \
    --object-path=/org/freedesktop/Notifications \
    --method=org.freedesktop.Notifications.CloseNotification "$notif_id" \
    >/dev/null 2>&1 || true
fi

exit 0
```

- [ ] **Step 2: 作成された差分を確認**

Run: `cd /home/kiyama/.local/share/chezmoi && git status dot_local/bin/`
Expected:
```
On branch main
Untracked files:
	dot_local/bin/executable_claude-notify-dispatch.sh
```

- [ ] **Step 3: コミット**

```bash
cd /home/kiyama/.local/share/chezmoi
git add dot_local/bin/executable_claude-notify-dispatch.sh
git commit -m "feat(notify): add claude-notify-dispatch.sh (popup action loop)

popup の生存期間中ずっと張り付き、notify-send --print-id --wait の
stdout から notification_id と action_key を読み、default action 受領時に
tmux switch-client + select-pane で focus、その後 gdbus CloseNotification
で popup を明示 close する dispatcher。

bare terminal fallback とセッション消失時の自動再オープンは F-3 の
follow-up TODO として inline コメントで明示。"
```

---

## Task 2: sound スクリプトを sound 再生専用にスリム化

**Files:**
- Modify: `dot_local/bin/executable_claude-notify-sound.sh`

- [ ] **Step 1: `dot_local/bin/executable_claude-notify-sound.sh` の中身を全面置換**

新しい内容:

```bash
#!/usr/bin/env bash
# Claude Code sound player. Plays the sound file at $1 using the first
# available backend: pipewire -> pulseaudio -> ffmpeg.
set -euo pipefail

sound="${1:-}"
[[ -z "$sound" || ! -r "$sound" ]] && exit 0

if command -v pw-play >/dev/null 2>&1; then
  exec pw-play --volume=0.6 "$sound"
elif command -v paplay >/dev/null 2>&1; then
  exec paplay --volume=39322 "$sound"
elif command -v ffplay >/dev/null 2>&1; then
  exec ffplay -nodisp -autoexit -loglevel quiet -volume 60 "$sound"
fi

exit 0
```

- [ ] **Step 2: 差分を確認**

Run: `cd /home/kiyama/.local/share/chezmoi && git diff dot_local/bin/executable_claude-notify-sound.sh | head -80`
Expected: 旧 hook ロジック (case 文 / payload parse / notify-send) が削除され、`pw-play / paplay / ffplay` の fallback だけが残った diff。

- [ ] **Step 3: smoke test (任意・音が鳴るので注意)**

Run: `bash dot_local/bin/executable_claude-notify-sound.sh /usr/share/sounds/freedesktop/stereo/message.oga`
Expected: 短い通知音が鳴って exit 0。

引数なし / 不在ファイルの場合の no-op 確認:

Run: `bash dot_local/bin/executable_claude-notify-sound.sh; echo "exit=$?"`
Expected: `exit=0`

Run: `bash dot_local/bin/executable_claude-notify-sound.sh /nonexistent.oga; echo "exit=$?"`
Expected: `exit=0`

- [ ] **Step 4: コミット**

```bash
cd /home/kiyama/.local/share/chezmoi
git add dot_local/bin/executable_claude-notify-sound.sh
git commit -m "refactor(notify): slim claude-notify-sound.sh to sound-only worker

hook 入口・payload parse・notify-send 呼び出しを後続の hook.sh /
dispatch.sh に切り出し、本ファイルは sound パスを 1 つ受け取って
pw-play -> paplay -> ffplay でフォールバック再生するだけに縮小する。"
```

---

## Task 3: hook 入口スクリプトを作成

**Files:**
- Create: `dot_local/bin/executable_claude-notify-hook.sh`

- [ ] **Step 1: `dot_local/bin/executable_claude-notify-hook.sh` を新規作成**

ファイル内容:

```bash
#!/usr/bin/env bash
# Claude Code hook entry. Parses event/payload, forks the sound player
# and the popup dispatcher, then exits immediately so the hook caller
# (claude) is not blocked.
#
# Argument $1: event key (notification | stop | subagent-stop | error).
# Stdin: Claude Code hook payload (JSON).
set -euo pipefail

event="${1:-notification}"
sound_dir="${CLAUDE_NOTIFY_SOUND_DIR:-/usr/share/sounds/freedesktop/stereo}"

payload=""
if [[ ! -t 0 ]]; then
  payload="$(cat || true)"
fi

case "$event" in
  notification)  sound="$sound_dir/message.oga";      title="Claude Code"; default_body="Awaiting input";    urgency="normal"   ;;
  stop)          sound="$sound_dir/complete.oga";     title="Claude Code"; default_body="Turn complete";     urgency="normal"   ;;
  subagent-stop) sound="$sound_dir/bell.oga";         title="Claude Code"; default_body="Subagent finished"; urgency="low"      ;;
  error)         sound="$sound_dir/dialog-error.oga"; title="Claude Code"; default_body="Error";             urgency="critical" ;;
  *)             sound="$sound_dir/message.oga";      title="Claude Code"; default_body="$event";            urgency="normal"   ;;
esac

# === payload extraction ===
body="$default_body"
session_id=""
if [[ -n "$payload" ]] && command -v jq >/dev/null 2>&1; then
  parsed_msg="$(printf '%s' "$payload" | jq -r '.message // empty' 2>/dev/null || true)"
  [[ -n "$parsed_msg" ]] && body="$parsed_msg"
  session_id="$(printf '%s' "$payload" | jq -r '.session_id // empty' 2>/dev/null || true)"
fi

# === tmux context (empty -> bare terminal) ===
tmux_pane="${TMUX_PANE:-}"
tmux_session=""
if [[ -n "$tmux_pane" ]] && command -v tmux >/dev/null 2>&1; then
  tmux_session="$(tmux display-message -p -t "$tmux_pane" '#{session_name}' 2>/dev/null || true)"
fi

# === fire & forget: sound ===
sound_bin="${CLAUDE_NOTIFY_SOUND_BIN:-$HOME/.local/bin/claude-notify-sound.sh}"
if [[ -x "$sound_bin" ]]; then
  "$sound_bin" "$sound" >/dev/null 2>&1 &
  disown 2>/dev/null || true
fi

# === fork dispatcher (popup + action loop) ===
dispatch_bin="${CLAUDE_NOTIFY_DISPATCH:-$HOME/.local/bin/claude-notify-dispatch.sh}"
if [[ -x "$dispatch_bin" ]] && command -v notify-send >/dev/null 2>&1; then
  CLAUDE_NOTIFY_TITLE="$title" \
  CLAUDE_NOTIFY_BODY="$body" \
  CLAUDE_NOTIFY_URGENCY="$urgency" \
  CLAUDE_NOTIFY_SESSION_ID="$session_id" \
  CLAUDE_NOTIFY_TMUX_PANE="$tmux_pane" \
  CLAUDE_NOTIFY_TMUX_SESSION="$tmux_session" \
    setsid "$dispatch_bin" </dev/null >/dev/null 2>&1 &
  disown 2>/dev/null || true
elif command -v notify-send >/dev/null 2>&1; then
  # fallback: dispatcher missing -> still show a (non-interactive) popup
  notify-send --app-name=ClaudeCode --urgency="$urgency" --expire-time=0 \
    -- "$title" "$body" >/dev/null 2>&1 || true
fi

exit 0
```

- [ ] **Step 2: 差分を確認**

Run: `cd /home/kiyama/.local/share/chezmoi && git status dot_local/bin/`
Expected: `Untracked files: dot_local/bin/executable_claude-notify-hook.sh` のみ。

- [ ] **Step 3: コミット**

```bash
cd /home/kiyama/.local/share/chezmoi
git add dot_local/bin/executable_claude-notify-hook.sh
git commit -m "feat(notify): add claude-notify-hook.sh (entry orchestrator)

Claude Code hook の新しい入口。event 引数から sound/title/urgency を
解決し、stdin payload から session_id と message を取り出した上で、
TMUX_PANE / tmux session 名をキャプチャし、claude-notify-sound.sh と
claude-notify-dispatch.sh を背景化 (setsid) で fork して即 exit 0 する。

dispatcher が deploy されていない時の fallback として、従来通り
notify-send で非対話 popup を出すパスも残す。"
```

---

## Task 4: chezmoi apply で 3 スクリプトを deploy + 配置確認

**Files:** (実行のみ。リポジトリへの編集は無し)

- [ ] **Step 1: chezmoi diff で deploy 内容を確認**

Run: `chezmoi diff ~/.local/bin/claude-notify-hook.sh ~/.local/bin/claude-notify-sound.sh ~/.local/bin/claude-notify-dispatch.sh`
Expected: hook と dispatch は新規作成 (`+` 行のみ)、sound は旧内容 → スリム版への置換 diff が表示される。

- [ ] **Step 2: chezmoi apply で deploy**

Run: `chezmoi apply ~/.local/bin/claude-notify-hook.sh ~/.local/bin/claude-notify-sound.sh ~/.local/bin/claude-notify-dispatch.sh`
Expected: 出力なしまたは "Applied" 程度。

- [ ] **Step 3: deploy 結果を検証**

Run:
```bash
ls -l ~/.local/bin/claude-notify-hook.sh ~/.local/bin/claude-notify-sound.sh ~/.local/bin/claude-notify-dispatch.sh
```
Expected: 3 ファイルとも存在し `-rwx` (実行ビット) が立っている。

Run: `chezmoi diff ~/.local/bin/claude-notify-hook.sh ~/.local/bin/claude-notify-sound.sh ~/.local/bin/claude-notify-dispatch.sh`
Expected: 出力なし (diff ゼロ)。

(コミットは Task 1〜3 で済んでいるためここでは無し)

---

## Task 5: settings.json の hook command path を差し替え

**Files:**
- Modify: `dot_config/claude/settings.json`

- [ ] **Step 1: 該当箇所 2 行を編集**

`Notification` ブロックと `Stop` ブロックの `command` を `claude-notify-sound.sh` から `claude-notify-hook.sh` に書き換える。

差し替え 2 箇所:

old: `"command": "/home/kiyama/.local/bin/claude-notify-sound.sh notification"`
new: `"command": "/home/kiyama/.local/bin/claude-notify-hook.sh notification"`

old: `"command": "/home/kiyama/.local/bin/claude-notify-sound.sh stop"`
new: `"command": "/home/kiyama/.local/bin/claude-notify-hook.sh stop"`

- [ ] **Step 2: 差分を確認**

Run: `cd /home/kiyama/.local/share/chezmoi && git diff dot_config/claude/settings.json`
Expected:
```diff
-              "command": "/home/kiyama/.local/bin/claude-notify-sound.sh notification"
+              "command": "/home/kiyama/.local/bin/claude-notify-hook.sh notification"
...
-              "command": "/home/kiyama/.local/bin/claude-notify-sound.sh stop"
+              "command": "/home/kiyama/.local/bin/claude-notify-hook.sh stop"
```

- [ ] **Step 3: chezmoi apply**

Run: `chezmoi apply ~/.config/claude/settings.json`
Expected: 出力なし。

- [ ] **Step 4: コミット**

```bash
cd /home/kiyama/.local/share/chezmoi
git add dot_config/claude/settings.json
git commit -m "feat(claude): point Notification/Stop hooks to claude-notify-hook.sh

新設の orchestrator (hook.sh) に通知 hook の起動先を切り替える。
sound.sh は sound 再生専用 worker に縮小されたため、直接 hook で
呼ぶのは不正。"
```

---

## Task 6: wired.ron の shortcuts を組み替え + wired を reload

**Files:**
- Modify: `dot_config/wired/wired.ron`

- [ ] **Step 1: shortcuts ブロックを編集**

`shortcuts: ShortcutsConfig (` 内の以下を変更:

old:
```ron
        notification_interact: 1,
        // notification_close: 2,
        notification_closeall: 3,
        // notification_pause: 99,

        notification_action1: 2,
```

new:
```ron
        notification_interact: 1,
        notification_close: 3,
        notification_closeall: 2,
        // notification_pause: 99,

        // notification_action1: 99,
```

- [ ] **Step 2: 差分を確認**

Run: `cd /home/kiyama/.local/share/chezmoi && git diff dot_config/wired/wired.ron`
Expected:
```diff
     notification_interact: 1,
-    // notification_close: 2,
-    notification_closeall: 3,
+    notification_close: 3,
+    notification_closeall: 2,
     // notification_pause: 99,

-    notification_action1: 2,
+    // notification_action1: 99,
```

- [ ] **Step 3: chezmoi apply**

Run: `chezmoi apply ~/.config/wired/wired.ron`
Expected: 出力なし。

- [ ] **Step 4: wired を reload**

Run: `pkill -USR1 wired || systemctl --user restart wired.service`
Expected: 出力なし、終了コード 0。

確認:

Run: `systemctl --user is-active wired.service`
Expected: `active`

- [ ] **Step 5: コミット**

```bash
cd /home/kiyama/.local/share/chezmoi
git add dot_config/wired/wired.ron
git commit -m "feat(wired): remap mouse shortcuts to left=focus / mid=closeall / right=close

左クリック (= notification_interact) は引き続き default action を
発火させ、新設の dispatcher が tmux focus を実行する。
右クリックは notification_close (この popup だけ消す) に変更し、
中クリックを notification_closeall にずらして滞留時の一掃手段として
温存。中クリック専用の action1 は廃止 (コメントアウト)。"
```

---

## Task 7: smoke test シナリオ 3 件を実行

**Files:** (実行のみ)

- [ ] **Step 1: シナリオ A — 左クリック focus**

事前準備: tmux 内の任意のペインで以下を実行 (claude pane でなくてよい)。

Run:
```bash
printf '{"message":"smoke test left-click"}' | ~/.local/bin/claude-notify-hook.sh notification
```

Expected:
- 通知音が鳴る
- 画面右上に「Claude Code: smoke test left-click」の popup が出る
- popup を **左クリック** すると、現在の tmux client が「コマンドを叩いたペインがある session/pane」に switch される (今回は同じ pane なので画面変化は無いが、popup が消えれば成功)

検証ログ:

Run: `journalctl --user -t claude-notify-dispatch --since "1 minute ago" 2>/dev/null | tail -3`
Expected: 警告ログ無し (tmux 内なので "focus skipped" は出ない)。

- [ ] **Step 2: シナリオ B — 右クリック close**

Run: `printf '{"message":"smoke test right-click"}' | ~/.local/bin/claude-notify-hook.sh notification`
Expected: popup 表示。

- popup を **右クリック** すると、その popup だけ消える。session 切替などの副作用無し。

- [ ] **Step 3: シナリオ C — 中クリック closeall**

Run:
```bash
for i in 1 2 3; do
  printf '{"message":"closeall test '$i'"}' | ~/.local/bin/claude-notify-hook.sh notification
  sleep 0.3
done
```
Expected: 3 個 popup が積み上がる。

- いずれかの popup を **中クリック** で全部消える。

- [ ] **Step 4: 失敗ケースの確認 (optional)**

dispatcher 不在時の fallback:

```bash
chmod -x ~/.local/bin/claude-notify-dispatch.sh
printf '{"message":"fallback test"}' | ~/.local/bin/claude-notify-hook.sh notification
```
Expected: 旧挙動の popup が出る。クリックしても何も起きない。

復元:
```bash
chmod +x ~/.local/bin/claude-notify-dispatch.sh
```

bare terminal:

tmux 外 (素の ghostty/kitty ターミナル) で:

```bash
printf '{"message":"bare terminal test"}' | ~/.local/bin/claude-notify-hook.sh notification
```
左クリック後:

Run: `journalctl --user -t claude-notify-dispatch --since "1 minute ago" | tail -3`
Expected: `focus skipped: no tmux context (sid=...)` のログ行が含まれる。

(本タスクではコード変更なし、コミット不要)

---

## Task 8: 既存ドキュメントを更新

**Files:**
- Modify: `docs/manage_claude.md` (§5.7 通知表)
- Modify: `docs/claude_tmux_cheatsheet.md` (§5 通知表 / §7 ファイル位置クイックリンク)

- [ ] **Step 1: `docs/manage_claude.md` §5.7 にクリックアクション表を追加**

§5.7 のサウンド表の直後 (「ポイント:」段落の前) に挿入:

```markdown
クリックアクション (`dot_config/wired/wired.ron` の `shortcuts` で定義):

| ボタン | 動作 |
|---|---|
| 左クリック | 発火元の tmux session/pane に focus を戻し、popup は自動で消える (`claude-notify-dispatch.sh` が `--action=default=Focus` を介して受信 → `tmux switch-client` + `select-pane` → `gdbus CloseNotification`) |
| 中クリック | 滞留した popup を一括 close (`notification_closeall`) |
| 右クリック | この popup だけ close (`notification_close`)。focus 等の副作用なし |

tmux 外で起動された Claude (素の terminal) は左クリック時 `journalctl --user -t claude-notify-dispatch` に "focus skipped" を出して no-op (将来 `wmctrl` / `swaymsg` ベースの WM-level focus を追加予定 — `docs/todos.md` F-3 follow-up)。
```

§5.7 の「フック登録は ... `hooks.Stop`」記述を `hooks.Notification` / `hooks.Stop` 双方とも `claude-notify-hook.sh` に切り替わった旨に書き換え。

§7 ファイル位置クイックリンク表の `dot_local/bin/executable_claude-notify-sound.sh` 行を 3 行に展開:

```markdown
| `dot_local/bin/executable_claude-notify-hook.sh` | hook 入口 (orchestrator)。event/payload を解析し sound と dispatcher を fork |
| `dot_local/bin/executable_claude-notify-sound.sh` | sound 再生 worker (pw-play→paplay→ffplay) |
| `dot_local/bin/executable_claude-notify-dispatch.sh` | popup の生存期間中張り付き、ActionInvoked を待って tmux focus + CloseNotification |
```

- [ ] **Step 2: `docs/claude_tmux_cheatsheet.md` §5 にクリックアクション表を追加**

§5 のサウンド表の直後 (「ポイント:」段落の前) に挿入:

```markdown
クリックアクション:

| ボタン | 動作 |
|---|---|
| 左クリック | 発火元の tmux pane に focus を戻して popup を auto-dismiss |
| 中クリック | 滞留 popup を一括 close |
| 右クリック | この popup だけ close |

詳細は [`docs/manage_claude.md`](./manage_claude.md) §5.7 / `dot_config/wired/wired.ron`。
```

§7 ファイル位置クイックリンク表の `dot_local/bin/executable_claude-notify-sound.sh` 行を 3 行に展開:

```markdown
| `dot_local/bin/executable_claude-notify-hook.sh` | Claude Code hook 入口 |
| `dot_local/bin/executable_claude-notify-sound.sh` | sound 再生 worker |
| `dot_local/bin/executable_claude-notify-dispatch.sh` | popup + click action ハンドラ |
```

- [ ] **Step 3: 差分を確認**

Run: `cd /home/kiyama/.local/share/chezmoi && git diff docs/manage_claude.md docs/claude_tmux_cheatsheet.md | head -120`
Expected: §5.7 / §5 / §7 の追記が見える。

- [ ] **Step 4: コミット**

```bash
cd /home/kiyama/.local/share/chezmoi
git add docs/manage_claude.md docs/claude_tmux_cheatsheet.md
git commit -m "docs(notify): describe left/mid/right click actions and 3-file split

manage_claude.md §5.7 と claude_tmux_cheatsheet.md §5 にクリックアクション
表を追加。§7 ファイル位置クイックリンクの sound.sh 行を hook.sh /
sound.sh / dispatch.sh の 3 行に展開する。"
```

---

## Task 9: `docs/todos.md` の F-3 を更新

**Files:**
- Modify: `docs/todos.md`

- [ ] **Step 1: F-3 セクションを v1 完了 + F-3.next に再構成**

旧 F-3 セクション全体 (背景 / 該当 / 設計メモ / 対応 / 注意 のサブブロック) を以下に置換:

```markdown
### F-3. wired 通知の左クリック / 右クリックアクション実装 (v1 完了 / follow-up あり)
- 背景: B 案で Claude Code → wired のデスクトップ通知が復活し、`--expire-time=0` で自動消去されなくなった (`dot_local/bin/executable_claude-notify-sound.sh`)。次のステップとして popup を **左クリックで発信元セッションへフォーカス** / **右クリックで個別 close**。設計ドキュメントは [`superpowers/specs/2026-04-30-wired-click-actions-design.md`](superpowers/specs/2026-04-30-wired-click-actions-design.md)、実装計画は [`superpowers/plans/2026-04-30-wired-click-actions.md`](superpowers/plans/2026-04-30-wired-click-actions.md) を参照。

#### v1 (実装済み, 2026-04-30)
- [x] hook を `claude-notify-hook.sh` (orchestrator) / `claude-notify-sound.sh` (sound worker) / `claude-notify-dispatch.sh` (popup + action loop) の 3 ファイルに分割
- [x] hook payload (`session_id`, `message`) と env (`TMUX_PANE`, tmux session 名) を環境変数で dispatcher に受け渡し
- [x] `notify-send --print-id --wait --action=default=Focus` で popup を保持し、ActionInvoked 受領時に tmux focus + `gdbus CloseNotification` で auto-dismiss
- [x] `wired.ron` shortcuts を `notification_interact: 1` (左) / `notification_close: 3` (右) / `notification_closeall: 2` (中) に組み替え
- [x] dispatcher を `setsid` で hook 親 (claude) から分離し、hook は即 exit 0
- [x] `docs/manage_claude.md` §5.7 と `docs/claude_tmux_cheatsheet.md` §5 にクリックアクション表を追記

#### F-3.next (follow-up, 未着手)
- [ ] 同一 `session_id` の通知が積み重なった場合の **`replace-id` ベース de-dup**。state ファイル or libnotify hint で session→notif_id を覚えて上書き
- [ ] **bare terminal fallback** — tmux 外で起動された Claude セッションを左クリックした時に `wmctrl` (X11) / `swaymsg` (Wayland) で cwd を持つ window を focus、または `transcript_path` を `$EDITOR` で開く
- [ ] **セッション消失時の自動再オープン** — kill されたあと残った popup を左クリック → `tmux_claude_new` 相当のロジックで tmux session を再生成 + `claude --resume <session_id>` で claude を復元
- [ ] **右クリックの拡張アクション** — 単純 close 以外に「transcript を開く」など二重アクションを検討 (要 wired/notify-send の追加 action 設計)
- [ ] **dispatcher を 1 本の常駐 helper daemon に集約** — D-Bus signal を直接 listen する案 (ブレストの案 B)。多重 popup 時の状態管理が綺麗になる代わりに systemd unit が増える
```

冒頭の「最終更新: 2026-04-30」は当該日付のため変更不要。

- [ ] **Step 2: 差分を確認**

Run: `cd /home/kiyama/.local/share/chezmoi && git diff docs/todos.md | head -100`
Expected: 旧 F-3 が削除され、新 F-3 (v1 完了 + F-3.next) に置換された diff。

- [ ] **Step 3: コミット**

```bash
cd /home/kiyama/.local/share/chezmoi
git add docs/todos.md
git commit -m "docs(todos): mark F-3 wired click actions v1 done, split out F-3.next

実装済みチェックボックスを v1 サブヘッダに集約、未実装項目 (replace-id
de-dup / bare terminal fallback / セッション消失時の自動再オープン /
右クリック拡張アクション / 常駐 daemon 化) を F-3.next として明示。

設計書 (superpowers/specs) と実装計画 (superpowers/plans) へのリンクも
追加。"
```

---

## Self-review

- **Spec coverage**: Spec §1 の左/中/右ボタン挙動は Task 6 (wired.ron) + Task 1 (dispatcher) で対応。§3 の各ファイル内容は Task 1 (dispatch) / Task 2 (sound) / Task 3 (hook) / Task 5 (settings.json) / Task 6 (wired.ron) で 1:1 対応。§4 failure modes は dispatcher 内の `command -v` ガード / fallback パス / silent fail 設計で実装済み。§5 test plan のシナリオ 1〜3 は Task 7 で smoke test 化、bare terminal / 失敗ケースは Task 7 step 4 (optional) でカバー。§6 rollout 手順は Task 4 (apply) → Task 6 step 4 (wired reload) → Task 7 (動作確認) → Task 8/9 (docs) で網羅。§7 follow-ups は Task 9 で F-3.next として residual 化。✓
- **Placeholder scan**: 「TBD」「TODO: implement later」「fill in details」相当なし。Task 7 step 4 の (optional) は手動 smoke test の補強であり必須ステップではない。コード内の `# TODO(F-3): ...` コメントは spec §7 の意図的 follow-up であり placeholder ではない。✓
- **Type consistency**: 環境変数名 (`CLAUDE_NOTIFY_TITLE` / `BODY` / `URGENCY` / `SESSION_ID` / `TMUX_PANE` / `TMUX_SESSION`) は Task 1 (dispatch 受領) と Task 3 (hook 送出) で完全一致。コマンドパス (`claude-notify-hook.sh` / `claude-notify-sound.sh` / `claude-notify-dispatch.sh`) は Task 3 / Task 5 / spec §3 で一貫。✓
