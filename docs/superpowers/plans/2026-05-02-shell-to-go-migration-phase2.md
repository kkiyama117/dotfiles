# Shell → Go Migration (Phase 2: A Subsystem — Notify Pipeline) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Phase 1 で確立した `programs/claude-tools/` 上に、A サブシステム notify pipeline 4 binary (`claude-notify-{cleanup,sound,hook,dispatch}`) を Go で 1:1 置換する。Phase 1 と同じく **PR 1 本につき shell 1 本 → Go 1 本の atomic 切替**で進め、cache / state file のパス・フォーマットは shell 時代と完全互換を維持する。

**Architecture:** Phase 1 で構築済みの `internal/{xdg,atomicfile,proc,obslog}` をそのまま再利用し、新規に `internal/notify` パッケージを追加。`internal/notify` には replace-id state I/O / popup struct / D-Bus action loop 用 API を集約する。

**Tech Stack:**
- Go 1.22+ (Phase 1 と同じ、mise install 済み)
- `log/slog` + `logger -t` for observability (`internal/obslog` 既存)
- `encoding/json` for hook payload
- **新規外部依存**: `github.com/godbus/dbus/v5` (PR-9 の `notify-dispatch` のみ使用)。Phase 2 開始時に `go get` する。

**Spec:** [`../specs/2026-05-01-shell-to-go-migration-design.md`](../specs/2026-05-01-shell-to-go-migration-design.md) §3.1 / §4.3 / §8

**Phase 1 plan (前提):** [`./2026-05-01-shell-to-go-migration.md`](./2026-05-01-shell-to-go-migration.md)

---

## File Structure

### 新規作成 (本 plan 内で生成)

| パス | 役割 |
|---|---|
| `programs/claude-tools/internal/notify/notify.go` | `Popup` struct / replace-id state I/O / `SafeSessionID` filename helper |
| `programs/claude-tools/internal/notify/notify_test.go` | replace-id round-trip / 不正フォーマット時の 0 fallback / SafeSessionID sanitize |
| `programs/claude-tools/internal/notify/dispatch.go` | (PR-9) D-Bus action listener / popup orchestration |
| `programs/claude-tools/internal/notify/dispatch_test.go` | (PR-9) FakeBus でのアクション受領テスト |
| `programs/claude-tools/cmd/claude-notify-cleanup/main.go` | (PR-6) mtime TTL 剪定 + base_dir suffix guard |
| `programs/claude-tools/cmd/claude-notify-cleanup/main_test.go` | (PR-6) 削除/保持/guard の table-driven |
| `programs/claude-tools/cmd/claude-notify-sound/main.go` | (PR-7) pw-play / paplay / ffplay フォールバック |
| `programs/claude-tools/cmd/claude-notify-sound/main_test.go` | (PR-7) backend 選択 / 引数組み立てを FakeRunner で検証 |
| `programs/claude-tools/cmd/claude-notify-hook/main.go` | (PR-8) Claude hook entry: payload parse / context / sound + dispatch fork (setsid) |
| `programs/claude-tools/cmd/claude-notify-hook/main_test.go` | (PR-8) event→sound/title/body/urgency マッピング、payload extract、env composition |
| `programs/claude-tools/cmd/claude-notify-dispatch/main.go` | (PR-9) popup + action loop entry point |
| `programs/claude-tools/cmd/claude-notify-dispatch/main_test.go` | (PR-9) state file load/save、focus dispatch 経路の選択 |
| `docs/superpowers/smoke/2026-05-02-go-notify-smoke.md` | Phase 2 完走時の smoke 結果記録 |

### 変更 (本 plan 内)

| パス | 変更内容 |
|---|---|
| `programs/claude-tools/go.mod` | (PR-9) `github.com/godbus/dbus/v5` を追加 |
| `programs/claude-tools/go.sum` | (PR-9) 同上 |
| `programs/claude-tools/README.md` | `internal/notify` の forward-looking 表記 → 実装済みに更新 |
| `dot_config/claude/settings.json` | (PR-8) hook command 4 箇所を `claude-notify-hook.sh` → `claude-notify-hook` に書き換え |
| `dot_config/systemd/user/claude-notify-cleanup.service` | (PR-6) `ExecStart=%h/.local/bin/claude-notify-cleanup.sh` → `claude-notify-cleanup` |
| `.chezmoiscripts/run_onchange_after_enable-claude-notify-cleanup.sh.tmpl` | (PR-6) helper sha256 行を撤去 (旧 shell 削除に伴う include エラー回避)、Go binary path に concept 切替 |
| `docs/todos.md` G-1 | PR-6 〜 PR-9 のチェックボックスを `[x]` に更新 (各 PR 完了時) |

### 削除 (本 plan 内、各 binary task で `git rm`)

| パス | 削除タイミング |
|---|---|
| `dot_local/bin/executable_claude-notify-cleanup.sh` | PR-6 |
| `dot_local/bin/executable_claude-notify-sound.sh` | PR-7 |
| `dot_local/bin/executable_claude-notify-hook.sh` | PR-8 |
| `dot_local/bin/executable_claude-notify-dispatch.sh` | PR-9 |

---

## Tasks

### Task 0: Prerequisites

- [ ] **Step 0.1: Working tree clean**: `git status` で `nothing to commit` を確認。HEAD は develop の `05d6132` (Phase 1 + Phase 1.5 merged) 以降。
- [ ] **Step 0.2: Phase 1 baseline 緑**: `cd programs/claude-tools && go test ./...` で全 ok。
- [ ] **Step 0.3: 環境**: D-Bus session bus が動いていること (`gdbus introspect --session --dest=org.freedesktop.Notifications --object-path=/org/freedesktop/Notifications | head -5` が出力を返す)。PR-9 smoke で必要。

### Task 1: PR-6 — `claude-notify-cleanup` (T1)

**Files:**
- Create: `programs/claude-tools/internal/notify/notify.go` (skeleton: package宣言 + `StateDir()` helper のみ)
- Create: `programs/claude-tools/internal/notify/notify_test.go`
- Create: `programs/claude-tools/cmd/claude-notify-cleanup/main.go`
- Create: `programs/claude-tools/cmd/claude-notify-cleanup/main_test.go`
- Modify: `dot_config/systemd/user/claude-notify-cleanup.service` (`ExecStart` のパスを `.sh` 抜きに)
- Modify: `.chezmoiscripts/run_onchange_after_enable-claude-notify-cleanup.sh.tmpl` (helper sha256 の include 行を撤去 / Go binary path 化)
- Delete: `dot_local/bin/executable_claude-notify-cleanup.sh`

**TDD steps:**

- [ ] **1.1 `internal/notify` を作成、`StateDir()` だけ移植**
  - `notify.StateDir() string` = `xdg.ClaudeNotifyStateDir()` の re-export (notify ドメインからの import を意味的に統一)
  - test: `xdg.RuntimeDir()` 経由のパス組み立て確認
- [ ] **1.2 `cmd/claude-notify-cleanup/main_test.go` を書く (RED)**
  - case A: 7 日 TTL, 8 日前 mtime の `*.id` は削除、3 日前は保持
  - case B: 60 分 TTL, 90 分前 mtime の `.tmp.foo` は削除、30 分前は保持
  - case C: `XDG_RUNTIME_DIR=/tmp/evil-xxx` (suffix が `claude-notify/sessions` でない) → 何も削除せず exit 0
  - case D: `base_dir` 不存在 → exit 0
  - case E: `CLAUDE_NOTIFY_CLEANUP_TTL_DAYS=invalid` → 既定 7 にフォールバック
  - case F: 削除発生時に obslog の Info ログが出る (FakeRunner で `logger -t` 観測)
- [ ] **1.3 `cmd/claude-notify-cleanup/main.go` を実装 (GREEN)**
  - `mtime` ベースの TTL 判定: `os.Stat` → `info.ModTime().Before(time.Now().Add(-ttl))`
  - `os.Remove` で削除、エラーは Warn ログ
  - shell 時代の `find ... -mtime +N` は >= N 日「より古い」(strictly older) 意味だったので、Go では `time.Now().Sub(mt) > time.Duration(ttlDays)*24*time.Hour` で同義になるよう確認
- [ ] **1.4 systemd unit / bootstrap 更新**
  - `dot_config/systemd/user/claude-notify-cleanup.service` の `ExecStart` から `.sh` を削る
  - `.chezmoiscripts/run_onchange_after_enable-claude-notify-cleanup.sh.tmpl`: `helper:` の sha256 行を撤去 (旧 shell が消えると `include` がエラーになるため)。代わりに `dot_config/systemd/user/claude-notify-cleanup.service` の hash + Go binary パス (`programs/claude-tools/cmd/claude-notify-cleanup/main.go` の hash) を埋め込む
- [ ] **1.5 git rm 旧 shell + commit**
  - `git rm dot_local/bin/executable_claude-notify-cleanup.sh`
  - commit message: `feat(g1): PR-6 replace claude-notify-cleanup.sh with Go binary`
- [ ] **1.6 smoke**
  - 合成 `XDG_RUNTIME_DIR` (mktemp -d) で `claude-notify/sessions/` を作り、stale `.id` × 2 + fresh `.id` × 4 + 古い `.tmp.X` × 1 を撒く
  - `claude-notify-cleanup` を実行、stale 2 + 古い tmp 1 のみ消えることを確認
  - 怪しい base_dir (例: `/tmp/evil`) を渡して何もしないことを確認
  - 結果を `docs/superpowers/smoke/2026-05-02-go-notify-smoke.md` に追記

### Task 2: PR-7 — `claude-notify-sound` (T1)

**Files:**
- Create: `programs/claude-tools/cmd/claude-notify-sound/main.go`
- Create: `programs/claude-tools/cmd/claude-notify-sound/main_test.go`
- Delete: `dot_local/bin/executable_claude-notify-sound.sh`
- Modify (任意): `claude-notify-hook` のデフォルト sound binary path 参照箇所 (PR-8 で同時更新する案も可)

**TDD steps:**

- [ ] **2.1 `main_test.go` を書く (RED)**
  - case A: `pw-play` 存在 → `pw-play --volume=0.6 <sound>` で起動 (FakeRunner で `Run` ではなく `LookPath` 相当を抽象化する必要 — `proc.Runner` に `LookPath` を増やすかは judgment)
  - case B: `pw-play` 不在 / `paplay` 存在 → `paplay --volume=39322 <sound>`
  - case C: 全部不在 → exit 0 (no error)
  - case D: 引数不在 / ファイル non-readable → exit 0
- [ ] **2.2 設計判断: `proc.Runner` を拡張 vs `exec.LookPath` 直叩き**
  - 既存 `proc.Runner` には `LookPath` がない。Phase 1 で `cockpit-prune` などは直接 `exec.LookPath` を使っていたか確認 → 同じパターンを踏襲
  - もし test のために抽象化が必要なら、`proc.Runner` に `LookPath(name string) (string, error)` を追加
- [ ] **2.3 実装 (GREEN)**: 17 行の shell をそのまま port
- [ ] **2.4 git rm + commit**: `feat(g1): PR-7 replace claude-notify-sound.sh with Go binary`
- [ ] **2.5 smoke**: `claude-notify-sound /usr/share/sounds/freedesktop/stereo/message.oga` で音が鳴ることを確認

### Task 3: PR-8 — `claude-notify-hook` (T4)

**Files:**
- Create: `programs/claude-tools/cmd/claude-notify-hook/main.go`
- Create: `programs/claude-tools/cmd/claude-notify-hook/main_test.go`
- Modify: `dot_config/claude/settings.json` (4 箇所の hook command を新 binary 名に)
- Delete: `dot_local/bin/executable_claude-notify-hook.sh`

**TDD steps:**

- [ ] **3.1 hook event → 通知マッピングのテーブル化**
  - 既存 shell の `case` 4 + default → Go の構造体テーブル
  - `eventConfig{ sound, title, defaultBody, urgency }` 形式
- [ ] **3.2 `main_test.go` を書く (RED)**
  - case A: `notification` event → `message.oga` / urgency=normal
  - case B: `stop` event → `complete.oga`
  - case C: `subagent-stop` → `bell.oga` / urgency=low
  - case D: `error` → `dialog-error.oga` / urgency=critical
  - case E: stdin payload `{"message":"foo","session_id":"abc","cwd":"/tmp"}` → body=foo, sid=abc を dispatch env に乗せる
  - case F: payload 空 → defaultBody そのまま
  - case G: cwd が git worktree → title に `· <main_repo>/<branch>` を append
  - case H: `TMUX_PANE` set + tmux 経由で session 名取得 → dispatch env に CLAUDE_NOTIFY_TMUX_{PANE,SESSION} 設定
  - case I: 致命エラーが起きても exit 0 (panic も recover 経由で握りつぶす)
- [ ] **3.3 実装 (GREEN)**
  - JSON parse: `encoding/json` で `struct{ Message string; SessionID string; Cwd string }` (json tag で `session_id` / `cwd` にマッピング)
  - git context: `proc.Runner.Run(ctx, "git", "-C", cwd, "worktree", "list", "--porcelain")` → 1 行目 worktree path → basename
  - tmux session 取得: `tmux display-message -p -t $TMUX_PANE '#{session_name}'`
  - sound fork: `proc.Runner.Start` (fire-and-forget)
  - dispatch fork: **`syscall.SysProcAttr.Setsid: true`** で hook 親と切り離す。Stdin/Stdout/Stderr は `/dev/null` にリダイレクト
  - `defer func() { recover(); os.Exit(0) }()` を冒頭に
- [ ] **3.4 settings.json 更新 + git rm**
  - 4 箇所の hook command (`notification` / `stop` / `subagent-stop` / `error` のような ON_* なら 4 箇所すべて) を `~/.local/bin/claude-notify-hook.sh <event>` → `~/.local/bin/claude-notify-hook <event>` に書き換え
  - `git rm dot_local/bin/executable_claude-notify-hook.sh`
- [ ] **3.5 commit**: `feat(g1): PR-8 replace claude-notify-hook.sh with Go binary`
- [ ] **3.6 smoke**: 実機 Claude Code に `Stop` イベントを発火させ (テストプロンプト → 完了)、popup + sound + tmux focus が動作することを確認

### Task 4: PR-9 — `claude-notify-dispatch` (T5, 最難所)

**Files:**
- Create: `programs/claude-tools/internal/notify/dispatch.go` (D-Bus action loop)
- Create: `programs/claude-tools/internal/notify/dispatch_test.go`
- Create: `programs/claude-tools/cmd/claude-notify-dispatch/main.go`
- Create: `programs/claude-tools/cmd/claude-notify-dispatch/main_test.go`
- Modify: `programs/claude-tools/go.mod` / `go.sum` (`github.com/godbus/dbus/v5` 追加)
- Delete: `dot_local/bin/executable_claude-notify-dispatch.sh`

**TDD steps:**

- [ ] **4.1 設計判断: `notify-send --print-id --wait` 維持 vs D-Bus 直接呼び出し**
  - 候補 A: `notify-send --print-id --wait` の stdout 行 1 = id / 行 2 = action key を Go から `proc.Runner` で読む — shell と最も近い
  - 候補 B: `dbus.Notify(...)` で id を取得 → 同じ bus connection で `ActionInvoked` / `NotificationClosed` signal を listen — shell の `gdbus call CloseNotification` も同じ bus で発行できて 1 接続で完結。**こちらを採用**
  - 理由: `--wait` の stderr/exit handling が shell でも fragile。Go 側で signal を直接 listen するほうが reproducibility が高い (G3 目標)
- [ ] **4.2 `internal/notify/dispatch.go` を実装**
  - `Popup` struct: SessionID / Title / Body / Urgency / Hints / Actions
  - `Dispatch(ctx, popup, prevID)` flow:
    1. `bus, err := dbus.SessionBus()` → 失敗時 fallback (notify-send 実行のみ、id=0)
    2. `bus.AddMatchSignal(...)` で `ActionInvoked` / `NotificationClosed` を listen
    3. `bus.Object("org.freedesktop.Notifications").Call("Notify", ...)` で id 取得
    4. signal channel から該当 id の event を待つ (timeout 24h or ctx cancel)
    5. action key を返却 (caller が tmux focus 等を実行)
  - `LoadReplaceID(stateDir, sid) uint32` / `SaveReplaceID(stateDir, sid, id) error` (atomicfile.Write 経由)
- [ ] **4.3 `dispatch_test.go`**
  - replace-id round-trip (read → write → read)
  - 不正フォーマット時 (`abc` / 負数 / 0) → 0 を返す
  - SafeSessionID 関数: `[^a-zA-Z0-9_-]` を `_` に置換、空なら `unknown`
  - D-Bus 経路は signal channel 注入できるよう `bus` インターフェース化、FakeBus で `Notify(...)` → id 返却 / `Subscribe(...)` → 事前登録 signal を流す test
- [ ] **4.4 `cmd/claude-notify-dispatch/main.go` 実装**
  - env から `Popup` を組み立て
  - `LoadReplaceID` → `notify.Dispatch(ctx, p, prev)` → action 受領
  - action == "default" なら tmux switch-client + select-pane + WM focus (xdotool/swaymsg) + `CloseNotification`
  - `SaveReplaceID` で新 id を永続化
- [ ] **4.5 git rm + go.mod 追加 + commit**
- [ ] **4.6 smoke (real D-Bus)**
  - 実機 wired-notify が動いている状態で manual で env を設定して dispatch を起動 → popup が出る、左クリックで focus、右クリックで close、replace-id で同 sid の連発が in-place 更新されることを確認

### Task 5: Phase 2 完走チェックポイント

- [ ] **5.1 `go test ./...` 全 PASS**
- [ ] **5.2 `chezmoi diff` の `~/.local/bin/claude-notify-*` 4 binary 差分確認**
- [ ] **5.3 `chezmoi apply` で実機反映**
- [ ] **5.4 `docs/manage_claude.md` §5.7 notify smoke を 1 周通す**
  - 通知発火 (Stop event) → popup 出現 → 左クリック focus → tmux pane に切替 / WM が前面化
  - 同 session 連発で popup が in-place 更新される (replace-id)
  - 右クリックで個別 close
- [ ] **5.5 `docs/todos.md` G-1 セクションのチェックを `[x]` に更新**
- [ ] **5.6 `docs/superpowers/smoke/2026-05-02-go-notify-smoke.md` に通し記録**
- [ ] **5.7 旧 shell 4 本が repo から消えていることを `git ls-files | rg notify` で確認**

---

## Risks / Mitigations

| リスク | 影響 | 緩和策 |
|---|---|---|
| `godbus/dbus/v5` の signal listen で deadlock | popup が永遠に閉じない | ctx timeout (24h) + `notify-send --print-id --wait` 経由の fallback path を残す |
| setsid 経由の dispatcher fork で stdin/out が claude hook に逆流 | hook が hang | `Stdin: nil` + 出力を `/dev/null` にリダイレクト |
| settings.json hook command の書き換え漏れ | claude が古い `.sh` を呼んで not found | PR-8 commit 内で `rg "claude-notify-hook.sh" dot_config/claude/` で 0 件を確認 |
| systemd unit 更新後の reload 漏れ | timer が新 binary を呼ばない | `run_onchange_after_enable-claude-notify-cleanup.sh.tmpl` に systemd unit の sha256 を埋めてあるため、`chezmoi apply` で `daemon-reload` が走る |
| D-Bus 不在環境 (sshd セッション直など) | dispatch がクラッシュ | `dbus.SessionBus()` 失敗時は notify-send fallback (action listener なし) で graceful degrade |

## Acceptance Criteria

- [ ] `programs/claude-tools/cmd/claude-notify-{cleanup,sound,hook,dispatch}/` の 4 binary が `go test ./...` 全 pass で build できる
- [ ] `chezmoi apply` で `~/.local/bin/claude-notify-{cleanup,sound,hook,dispatch}` が生える
- [ ] 旧 shell 4 本が repo から消えている
- [ ] `claude-notify-cleanup.timer` が新 binary を ExecStart している (`systemctl --user cat claude-notify-cleanup.service` で確認)
- [ ] Claude Code → wired popup → 左クリック focus → tmux 切替が一連で動作する
- [ ] `journalctl --user | grep claude-notify-` で hook / dispatch のログが観測できる
