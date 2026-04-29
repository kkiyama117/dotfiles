# Wired notification click actions — design

作成日: 2026-04-30
status: draft (awaiting user review before writing-plans)
related TODO: `docs/todos.md` §F-3
related docs: `docs/manage_claude.md` §5.7, `docs/claude_tmux_cheatsheet.md` §5

---

## 1. Goal / motivation

`dot_local/bin/executable_claude-notify-sound.sh` が `--expire-time=0` で出している Claude Code の常駐 popup に対して、クリックアクションを以下のように再定義する。

| ボタン | 期待動作 |
|---|---|
| **左クリック** | 通知を発行した Claude セッション (= tmux pane) に focus を戻す。focus 後、popup は自動で閉じる |
| **中クリック** | 滞留している popup を一括で消す (緊急避難用) |
| **右クリック** | この popup **だけ** 閉じる。focus などの副作用なし |

Out of scope (v1):
- 同一 session_id の連続通知に対する `replace-id` ベースの de-dup
- tmux 外で起動された Claude (素の terminal) への WM 経由の window focus
- セッション消失時の自動再オープン (kill されたあと残った popup を左クリック → tmux session を再生成して claude を resume)
- 上記 3 つは `docs/todos.md` F-3 の追加チェックボックスとして残す。

---

## 2. Architecture

### Components

| ファイル | 既存/新規 | 役割 | 寿命 |
|---|---|---|---|
| `dot_local/bin/executable_claude-notify-hook.sh` | 新規 | hook 入口。event → title/sound/urgency 解決、payload parse、env キャプチャ、子プロセス 2 つを fork | 短命 (Claude を待たせない) |
| `dot_local/bin/executable_claude-notify-sound.sh` | 既存 (スリム化) | sound ファイルパスを 1 個受け取って再生 (pw-play→paplay→ffplay) | 短命 |
| `dot_local/bin/executable_claude-notify-dispatch.sh` | 新規 | `notify-send --print-id --wait` で popup を保持し、ActionInvoked を待つ。`default` 受領時に tmux focus + CloseNotification | popup の寿命 |
| `dot_config/claude/settings.json` | 既存 (修正) | hook command を `claude-notify-sound.sh` → `claude-notify-hook.sh` に差し替え | — |
| `dot_config/wired/wired.ron` | 既存 (修正) | クリック → action マッピングの組み替え | — |

### Data flow

```
Claude Code (Notification | Stop)
   │ stdin = JSON payload
   ▼
claude-notify-hook.sh                            ← hook 親 (claude) は即解放
   ├─ jq でフィールド抽出: session_id, message
   ├─ env キャプチャ: TMUX_PANE, tmux_session
   ├─ claude-notify-sound.sh "$sound" &          ← 音は非同期
   ├─ setsid claude-notify-dispatch.sh &         ← popup + action ループ
   └─ exit 0

claude-notify-dispatch.sh (背景, popup と同寿命)
   ├─ notify-send --print-id --wait \
   │     --action=default=Focus \
   │     --hint=string:x-claude-session:"$SID" \
   │     --expire-time=0 \
   │     -- "$TITLE" "$BODY"
   │
   ├─ stdout を mapfile で 2 行読む
   │   line 1 = notification id
   │   line 2 = action key (close されただけなら欠落)
   │
   ├─ action == "default" (左クリック):
   │   ├─ tmux focus: switch-client + select-pane (失敗時は logger で warn)
   │   └─ gdbus call CloseNotification "$id"     ← FDO 仕様上 auto-close されないので明示
   │
   └─ それ以外 (右クリック / 中クリック / timeout): no-op exit
```

### Design judgments

- **`setsid` で session 切り離し**: hook 親 (claude) が exit/死んでも dispatcher は生き残り ActionInvoked を確実に拾える。
- **責務を 3 ファイルに分離**: `hook` = 入口 / `sound` = 再生 / `dispatch` = action ループ。混ぜずに済むので将来 (de-dup や常駐 daemon 化に進化させる時) も dispatch だけ差し替えで済む。
- **環境変数で受け渡し**: コマンドライン引数より env の方が改行 / 引用符の扱いで事故りにくい。`CLAUDE_NOTIFY_*` プレフィックスで衝突回避。
- **session_id は hint で運ぶ**: 将来 `replace-id` ベースで de-dup する時の足場。v1 では伝送のみ、利用しない。
- **音再生は hook 側で fire-and-forget**: dispatcher の寿命に音を引きずらせるとレイテンシが悪化する。
- **`tmux_session` は hook 起動時に解決**: TMUX_PANE が確実に有効なうちに `tmux display-message` でセッション名を確定。dispatcher 側で再解決するより堅牢。

---

## 3. File contents

### 3.1 `dot_local/bin/executable_claude-notify-hook.sh` (new)

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

### 3.2 `dot_local/bin/executable_claude-notify-sound.sh` (slimmed down)

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

### 3.3 `dot_local/bin/executable_claude-notify-dispatch.sh` (new)

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

### 3.4 `dot_config/claude/settings.json` diff

```diff
       "Notification": [
         {
           "hooks": [
             {
               "type": "command",
-              "command": "/home/kiyama/.local/bin/claude-notify-sound.sh notification"
+              "command": "/home/kiyama/.local/bin/claude-notify-hook.sh notification"
             }
           ]
         }
       ],
       "Stop": [
         {
           "hooks": [
             {
               "type": "command",
-              "command": "/home/kiyama/.local/bin/claude-notify-sound.sh stop"
+              "command": "/home/kiyama/.local/bin/claude-notify-hook.sh stop"
             }
           ]
         }
       ]
```

### 3.5 `dot_config/wired/wired.ron` diff

```diff
 shortcuts: ShortcutsConfig (
     notification_interact: 1,
-    // notification_close: 2,
-    notification_closeall: 3,
+    notification_close: 3,
+    notification_closeall: 2,
     // notification_pause: 99,

-    notification_action1: 2,
+    // notification_action1: 99,
     // notification_action2: 99,
     // notification_action3: 99,
     // notification_action4: 99,
 ),
```

| ボタン | 動作 (wired 側) | 結果 (UX) |
|---|---|---|
| **左 (1)** | `notification_interact` → ActionInvoked("default") | dispatcher が受信 → tmux focus → CloseNotification で auto-dismiss |
| **中 (2)** | `notification_closeall` | 滞留 popup を一括で流す |
| **右 (3)** | `notification_close` | この popup だけ消す。focus 等の副作用なし |

---

## 4. Failure modes

| 失敗ケース | v1 挙動 | 将来 (TODO) |
|---|---|---|
| `dispatcher` バイナリ不在 (apply 漏れ) | `hook.sh` が直接 `notify-send` で popup を出す。アクションは無いが通知は出る | — |
| `notify-send` 不在 | hook は静かに exit 0 (Claude をブロックしない) | — |
| `jq` 不在 | payload parse をスキップ。body は default、session_id は空 | — |
| `tmux` 不在 / `TMUX_PANE` 空 (bare terminal) | `logger` で警告だけ出して no-op | WM-level focus (`wmctrl` / `swaymsg`) を cwd ベースで実装 |
| `tmux has-session` が false (セッション削除済み) | 同上、no-op + log | **`session_id` が記録に残っていれば `claude --resume` で session を自動再オープン** (F-3 追加 TODO) |
| `gdbus` 不在 | popup の auto-dismiss だけ失敗。focus 自体は完了。手動で右クリックで消せる | — |
| `notify-send --wait` が予期せず空出力 | `mapfile` が空配列 → action_key 空 → 無動作 exit | — |
| 連続発火 | de-dup なし。各 popup が個別に dispatcher を持ち独立に閉じる | `replace-id` ベースで同一 `session_id` の通知を上書き |

すべて **silent fail** 方針 (Claude プロセスを巻き込まない)。

---

## 5. Test plan

ユニットテストは無し (Bash + libnotify + tmux の組み合わせは shell 側で担保しにくい)。手動シナリオで確認する:

| シナリオ | 手順 | 期待 |
|---|---|---|
| **基本: 左クリック focus** | tmux session A の claude pane で `printf '{"message":"test"}' \| ~/.local/bin/claude-notify-hook.sh notification` を実行 → 別 tmux session B に switch → popup を左クリック | session A の claude pane に戻り、popup が消える |
| **右クリック close** | 同上 → popup を右クリック | この popup だけ消える。session 切替は起きない |
| **中クリック closeall** | 通知を 2-3 個積む → 中クリック | すべて消える |
| **dispatcher 欠落 fallback** | `chmod -x ~/.local/bin/claude-notify-dispatch.sh` → hook を叩く | popup は出る (旧挙動)。click では何もしない |
| **bare terminal** | tmux 外で `claude-notify-hook.sh notification` | popup 表示 → 左クリック → `journalctl --user -t claude-notify-dispatch` に "focus skipped" が出る |
| **session 消失** | session A で hook → A を `tmux kill-session -t A` → 残った popup を左クリック | log 警告のみ、何も起こらない (将来 TODO で再オープン) |

---

## 6. Rollout

1. 3 ファイルを `dot_local/bin/` に配置 (`executable_*` prefix)
2. `dot_config/claude/settings.json` の hook command を `claude-notify-hook.sh` に書き換え
3. `dot_config/wired/wired.ron` の shortcuts を編集
4. `chezmoi diff` で確認 → `chezmoi apply`
5. wired を再読込: `pkill -USR1 wired` (or `systemctl --user restart wired.service`)
6. 動作確認 (上のシナリオ 1 / 2 / 3 を最低)
7. `docs/manage_claude.md` §5.7 と `docs/claude_tmux_cheatsheet.md` §5 の通知表に「左 / 中 / 右クリック」の挙動を追記
8. `docs/todos.md` F-3 の v1 達成分を `[x]` 化、未達分は **F-3.next** として残置

---

## 7. Open questions / future follow-ups (TODO)

v1 では実装しないが、F-3 のフォローアップとして残す:

- [ ] **同一 `session_id` の通知の de-dup** (`replace-id` で上書き)
- [ ] **bare terminal fallback** (`wmctrl` / `swaymsg` で cwd を持つ window を focus、または `transcript_path` を `$EDITOR` で開く)
- [ ] **セッション消失時の自動再オープン** — kill されたあと残った popup を左クリック → `tmux_claude_new` 相当のロジックで tmux session を再生成 + `claude --resume <session_id>` で claude を復元
- [ ] **右クリックの拡張アクション** — 単純 close だけでなく「transcript を開く」など二重アクションを検討
- [ ] **dispatcher を 1 本の常駐 helper daemon に集約** (D-Bus signal を直接 listen する案 = ブレストの案 B)
