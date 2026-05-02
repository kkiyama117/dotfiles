# `claude-notifyd` Daemon Implementation Plan (G-1.next #3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `docs/superpowers/specs/2026-05-02-notify-dispatch-daemon-design.md` で確定した設計を 4 PR (D-1 〜 D-4) で段階投入する。各 PR は build/test 緑で独立 mergeable、全 PR merge 後に socket activation で daemon が常駐するようになる。**legacy `claude-notify-dispatch` は本 plan で削除しない** (fallback として温存)。

**Spec:** `../specs/2026-05-02-notify-dispatch-daemon-design.md`

**Tech Stack:**
- Go 1.22+ (Phase 1/2 と同じ)
- 新規依存: `github.com/godbus/dbus/v5` (D-Bus session bus client)
- `log/slog` + `internal/obslog` (既存)
- `net` (stdlib Unix socket)
- systemd socket activation (`coreos/go-systemd/v22/activation` を導入するか stdlib `net.FileListener` で済ますかは PR-D1 内で判定)

---

## File Structure

### 新規

| パス | PR | 役割 |
|---|---|---|
| `programs/claude-tools/internal/notifyd/protocol.go` | D-1 | wire JSON 型 + Marshal/Unmarshal + version 検証 |
| `programs/claude-tools/internal/notifyd/protocol_test.go` | D-1 | round-trip / unknown v / malformed |
| `programs/claude-tools/internal/notify/focus.go` | D-1 | dispatch 側から `focusTmux` / `focusWM` / `closeNotification` を切り出し共有化 |
| `programs/claude-tools/internal/notify/focus_test.go` | D-1 | 既存 dispatch のテストを移植 |
| `programs/claude-tools/internal/notifyd/state.go` | D-2 | sid→id table 管理 / disk warm-start / atomicfile sync |
| `programs/claude-tools/internal/notifyd/state_test.go` | D-2 | warm-start / concurrent update (race) |
| `programs/claude-tools/internal/notifyd/dbus.go` | D-2 | godbus 抽象 (interface) + 実装 + reconnect backoff |
| `programs/claude-tools/internal/notifyd/dbus_test.go` | D-2 | FakeBus で Notify/signal/reconnect |
| `programs/claude-tools/internal/notifyd/server.go` | D-3 | listener / accept loop / connection → state → dbus 結線 |
| `programs/claude-tools/internal/notifyd/server_test.go` | D-3 | end-to-end (in-process socket) / concurrent client |
| `programs/claude-tools/cmd/claude-notifyd/main.go` | D-3 | flags / systemd LISTEN_FDS / signal handling |
| `programs/claude-tools/cmd/claude-notifyd/main_test.go` | D-3 | flag parsing / lifecycle |
| `dot_config/systemd/user/claude-notifyd.socket` | D-4 | `ListenStream=%t/claude-notify/sock` + perm 0600 |
| `dot_config/systemd/user/claude-notifyd.service` | D-4 | `Type=notify` + sandboxing + Restart |
| `.chezmoiscripts/run_onchange_after_enable-claude-notifyd.sh.tmpl` | D-4 | daemon-reload + enable --now (hash 連動) |
| `docs/superpowers/smoke/2026-05-02-notifyd-smoke.md` | D-4 | smoke ログ |

### 変更

| パス | PR | 変更内容 |
|---|---|---|
| `programs/claude-tools/go.mod` / `go.sum` | D-2 | `github.com/godbus/dbus/v5` 追加 |
| `programs/claude-tools/cmd/claude-notify-dispatch/main.go` | D-1 | `focusTmux` / `focusWM` / `closeNotification` を `internal/notify/focus.go` から呼ぶ薄い entry に縮約 |
| `programs/claude-tools/cmd/claude-notify-dispatch/main_test.go` | D-1 | 切り出し後の test を `internal/notify/focus_test.go` に移管、main_test は dispatch flow のみ |
| `programs/claude-tools/internal/notify/state.go` | D-2 | `LoadAllReplaceIDs(stateDir) (map[string]uint32, error)` 追加 (warm start) |
| `programs/claude-tools/internal/notify/state_test.go` | D-2 | 新 export のテスト |
| `programs/claude-tools/cmd/claude-notify-hook/main.go` | D-4 | `forkDispatch` の前に `dialDaemon` を試行 / 失敗時 fallback |
| `programs/claude-tools/cmd/claude-notify-hook/main_test.go` | D-4 | socket 経路 / dial timeout / connection refused 各テスト |
| `programs/claude-tools/README.md` | D-4 | `claude-notifyd` セクション + arch 図 |
| `docs/manage_claude.md` | D-4 | §5 notify pipeline 図 update |
| `docs/keybinds.md` | D-4 | §通知 daemon 動線 update |
| `docs/todos.md` | D-4 | G-1.next #3 / F-3.next L33 を `[x]` |

### 削除

なし

---

## Tasks

### Task 0: Prerequisites

- [ ] **Step 0.1: Working tree clean**: `git status` で `nothing to commit`。HEAD は `feat/notify-dispatch-daemon` の `8489f47` (PR-9 完了点)。
- [ ] **Step 0.2: G-1 baseline 緑**: `cd programs/claude-tools && go test -race ./...` で全 ok。
- [ ] **Step 0.3: D-Bus session bus が動いている**: `gdbus introspect --session --dest=org.freedesktop.DBus --object-path=/org/freedesktop/DBus | head -3` が出力を返す。
- [ ] **Step 0.4: spec confirm**: `docs/superpowers/specs/2026-05-02-notify-dispatch-daemon-design.md` §3 / §4 の判断 (案 B 採用 / godbus 採用 / Unix socket / fallback 温存) をユーザーが承認。

### Task 1: PR-D1 — Protocol + focus 共有化 (基盤、依存追加なし)

**Goal:** daemon 実装の最小依存基盤。godbus 追加前に commit 可能。

**Files:**
- Create: `programs/claude-tools/internal/notifyd/protocol.go`
- Create: `programs/claude-tools/internal/notifyd/protocol_test.go`
- Create: `programs/claude-tools/internal/notify/focus.go`
- Create: `programs/claude-tools/internal/notify/focus_test.go`
- Modify: `programs/claude-tools/cmd/claude-notify-dispatch/main.go` (focus 系を呼び出し側に変更)
- Modify: `programs/claude-tools/cmd/claude-notify-dispatch/main_test.go` (該当テストを focus_test.go に移管)

**TDD steps:**

- [ ] **D1.1 `internal/notifyd/protocol_test.go` (RED)**
  - case A: 正常な `Show` (v=1) を Marshal → Unmarshal で round-trip 一致
  - case B: `v=2` (未知) を Unmarshal → `ErrUnsupportedVersion`
  - case C: `op="bogus"` → `ErrUnknownOp`
  - case D: malformed JSON (truncated) → error
  - case E: `Show.SID` が 256 文字超 → `ErrFieldTooLong` (DoS 抑止)
  - case F: optional field 全欠落でも Unmarshal 成功 (空文字)
- [ ] **D1.2 `internal/notifyd/protocol.go` (GREEN)**
  - `type Frame struct { V uint8; Op string; SID, Title, Body, Urgency, TmuxPane, TmuxSession string }` (flat struct + Op で switch)
  - `Marshal(Frame) ([]byte, error)` / `Unmarshal(line []byte) (Frame, error)`
  - 定数 `MaxFrameBytes = 8192` で受信側 LimitReader 上限
- [ ] **D1.3 `internal/notify/focus_test.go` (RED → GREEN)**
  - 既存 `cmd/claude-notify-dispatch/main_test.go` の `focusTmux` / `focusWM` / `closeNotification` テストを移植
  - 移植先で同じ FakeRunner で全 PASS
- [ ] **D1.4 `internal/notify/focus.go` 実装**
  - `FocusTmux(ctx, runner, popup PopupContext) error` (Popup 構造体は notify package に export)
  - `FocusWM(ctx, runner, lookPath, getEnv) error`
  - `CloseNotification(ctx, runner, id uint32) error`
  - 既存 dispatch の挙動と byte-equivalent (terminal class list / ログ key 全て同一)
- [ ] **D1.5 dispatch を薄い entry に縮約**
  - `cmd/claude-notify-dispatch/main.go` の `focusTmux` / `focusWM` / `closeNotification` を削除し `notify.FocusTmux(...)` などに差し替え
  - dispatch 側の test は flow only に縮約 (focus の詳細テストは移管済)
  - `go test -race ./...` 全緑
- [ ] **D1.6 commit**
  - message: `refactor(g1): D-1 extract notify focus helpers + add notifyd protocol`

### Task 2: PR-D2 — godbus + state + dbus wrapper

**Goal:** daemon が呼ぶ D-Bus / state 層を独立に追加。socket / hook 改修は次 PR。

**Files:**
- Modify: `programs/claude-tools/go.mod` / `go.sum` (godbus 追加)
- Create: `programs/claude-tools/internal/notifyd/state.go` / `state_test.go`
- Create: `programs/claude-tools/internal/notifyd/dbus.go` / `dbus_test.go`
- Modify: `programs/claude-tools/internal/notify/state.go` (`LoadAllReplaceIDs` 追加)
- Modify: `programs/claude-tools/internal/notify/state_test.go` (新 export のテスト)

**TDD steps:**

- [ ] **D2.1 `internal/notify/state_test.go` に `TestLoadAllReplaceIDs`**
  - tempdir に 3 件の `.id` (有効 / 0 始まり / 文字列) → 有効 1 件のみ map に載る
  - `LoadAllReplaceIDs(emptyDir) → empty map, nil`
  - `LoadAllReplaceIDs(missingDir) → empty map, nil`
- [ ] **D2.2 `internal/notify/state.go` に `LoadAllReplaceIDs` 実装**
  - `os.ReadDir(stateDir)` → 各エントリで `LoadReplaceID` 再利用
- [ ] **D2.3 `internal/notifyd/state_test.go` (RED)**
  - case A: `Set/Get` round-trip + disk への atomicfile.Write が起きる (tempdir 観測)
  - case B: 100 goroutine concurrent `Set` → race detector で no race
  - case C: warm start: tempdir に既存 `.id` を撒く → `NewState(dir)` 直後に `Get(sid)` でヒット
  - case D: `Delete(notifID)` で in-flight ctx が消える (replace-id は disk に残す)
- [ ] **D2.4 `internal/notifyd/state.go` 実装**
  - `type State struct { mu sync.RWMutex; replace map[string]uint32; inflight map[uint32]InflightCtx; dir string }`
  - `NewState(dir) (*State, error)` で warm start
  - `(*State).RegisterShow(sid string, popup PopupContext) (prevID uint32)` (D-Bus Notify 直前)
  - `(*State).RecordShown(sid string, notifID uint32) error` (Notify 戻り値受領後)
  - `(*State).LookupAction(notifID uint32) (PopupContext, bool)`
  - `(*State).Forget(notifID uint32)`
- [ ] **D2.5 `go.mod` に godbus 追加**
  - `cd programs/claude-tools && go get github.com/godbus/dbus/v5@latest`
- [ ] **D2.6 `internal/notifyd/dbus_test.go` (RED)**
  - 抽象 interface `Bus` を定義 (`Notify(...)` / `SubscribeActions() <-chan ActionEvent` / `SubscribeClosed() <-chan ClosedEvent` / `CloseNotification(id)`)
  - `FakeBus` 実装で test:
    - `Notify(replace=0)` → 任意の id 返却 → `RecordShown` 経由で table 反映
    - `Notify(replace=42)` → daemon は同 id を要求し直す
    - `actionCh <- {id: 7, key: "default"}` → handler が呼ばれて FocusTmux が走る (FakeRunner で観測)
    - reconnect: `Disconnect()` → backoff → 再 subscribe
- [ ] **D2.7 `internal/notifyd/dbus.go` 実装**
  - 実 `realBus struct { conn *dbus.Conn }` で `dbus.SessionBus()`
  - `Notify` 呼び出しは `bus.Object("org.freedesktop.Notifications").Call("Notify", 0, ...)`
  - signal は `bus.AddMatchSignal(...)` + `bus.Signal(ch)` で受領 → 内部 type に変換して `actionCh` / `closedCh` へ送出
  - reconnect: `conn.Closed()` を別 goroutine で監視 → 5 回 / 100ms→1.6s 指数 backoff
- [ ] **D2.8 `go test -race ./...` 全緑 / coverage `internal/notifyd/` ≥ 80%**
- [ ] **D2.9 commit**: `feat(g1): D-2 add notifyd state + dbus wrapper (godbus dep)`

### Task 3: PR-D3 — `claude-notifyd` daemon entry

**Goal:** D-1 / D-2 の基盤を結線して `claude-notifyd` バイナリを生やす。systemd unit / hook 改修は次 PR。

**Files:**
- Create: `programs/claude-tools/internal/notifyd/server.go` / `server_test.go`
- Create: `programs/claude-tools/cmd/claude-notifyd/main.go` / `main_test.go`

**TDD steps:**

- [ ] **D3.1 `internal/notifyd/server_test.go` (RED)**
  - case A: in-process Unix socket pair で `Show` 1 件送信 → FakeBus.Notify が 1 回呼ばれる
  - case B: 同 sid で 5 連発 → 2 件目以降は `replace_id == prev` で Notify 呼ばれる (disk に id 1 件)
  - case C: 50 connection を並列に open + 1 frame 送信 → race detector で no race
  - case D: malformed frame (`{` 単独) → daemon は connection close、daemon 自体は live
  - case E: `MaxFrameBytes` 超 → connection close、daemon live
  - case F: ActionInvoked signal を FakeBus から発火 → state.LookupAction → FakeRunner に `tmux switch-client` 観測
  - case G: NotificationClosed signal → state.Forget(id) → 以降 ActionInvoked が来ても無視
- [ ] **D3.2 `internal/notifyd/server.go` 実装**
  - `type Server struct { ln net.Listener; bus Bus; state *State; runner proc.Runner; ... }`
  - `NewServer(opts ServerOptions) *Server` (functional options pattern)
  - `(*Server).Serve(ctx context.Context) error` — Accept loop + signal goroutine
  - 各 connection は 1 frame 読んで close (LimitReader + bufio.Scanner)
  - 内部チャネル: `reqCh chan request` (Accept goroutine → state goroutine)、`actionCh / closedCh` (bus → state goroutine)
  - state goroutine 1 本に集約して mutex 削減 (or RWMutex 維持。判断は実装時)
- [ ] **D3.3 `cmd/claude-notifyd/main_test.go`**
  - flag parse: `--listen <path>` / 既定値
  - LISTEN_FDS=1 環境変数で `os.NewFile(3, ...)` 経由で listener 取得 (systemd socket activation 模擬)
  - SIGTERM 受領で grace shutdown (5s 上限)
- [ ] **D3.4 `cmd/claude-notifyd/main.go` 実装**
  - flag: `--listen <path>` (既定: `${XDG_RUNTIME_DIR}/claude-notify/sock`)
  - LISTEN_FDS が設定されていれば fd 3 を listener として使用、そうでなければ `net.Listen("unix", path)`
  - flock: `<state_dir>/notifyd.lock` を `syscall.Flock(LOCK_EX|LOCK_NB)` で取得、取れなければ exit 0 (二重起動排除)
  - `signal.Notify(SIGTERM, SIGINT)` → ctx cancel → `Server.Serve` return 待ち (5s timeout)
- [ ] **D3.5 build smoke**
  - `cd programs/claude-tools && go build ./cmd/claude-notifyd && ./claude-notifyd --listen /tmp/notifyd.sock &`
  - 別 shell から `printf '{"v":1,"op":"show","sid":"smoke","title":"T","body":"B","urgency":"normal"}\n' | nc -U /tmp/notifyd.sock`
  - popup が表示される (実機 D-Bus 必要)
  - `kill %1` で daemon 終了
- [ ] **D3.6 commit**: `feat(g1): D-3 add claude-notifyd daemon entry`

### Task 4: PR-D4 — Hook 結線 + systemd unit + docs

**Goal:** 通常運用で daemon が socket activation で立ち上がり、hook が socket dispatch を試行するエンドツーエンド完走。

**Files:**
- Modify: `programs/claude-tools/cmd/claude-notify-hook/main.go`
- Modify: `programs/claude-tools/cmd/claude-notify-hook/main_test.go`
- Create: `dot_config/systemd/user/claude-notifyd.socket`
- Create: `dot_config/systemd/user/claude-notifyd.service`
- Create: `.chezmoiscripts/run_onchange_after_enable-claude-notifyd.sh.tmpl`
- Modify: `programs/claude-tools/README.md`
- Modify: `docs/manage_claude.md`
- Modify: `docs/keybinds.md`
- Modify: `docs/todos.md`
- Create: `docs/superpowers/smoke/2026-05-02-notifyd-smoke.md`

**TDD steps:**

- [ ] **D4.1 `cmd/claude-notify-hook/main_test.go` 拡張 (RED)**
  - in-process Unix socket listener を立てて `dialDaemon(ctx, sockPath, 100ms)` が成功する
  - sock path 不在 → `ErrSocketUnavailable` を返す → fallback (既存 `forkDispatch` が呼ばれる、既存 mock 拡張)
  - timeout: 接続するが accept されない → `ErrSocketTimeout` → fallback
  - dial 成功で 1 frame 書き込んで close、`forkDispatch` は呼ばれない
- [ ] **D4.2 `cmd/claude-notify-hook/main.go` 実装**
  - `dialDaemon(ctx, sockPath string, timeout time.Duration) error`
  - 接続成功時: `protocol.Marshal(Frame{V:1, Op:"show", ...})` を 1 行書いて close
  - `forkDispatch` は dial 失敗時のみ実行
  - sockPath 既定: `notify.SocketPath()` (新規 helper、`StateDir()` と兄弟)
- [ ] **D4.3 systemd unit 作成**
  - `claude-notifyd.socket`:
    ```
    [Unit]
    Description=Claude Code notifyd socket (G-1.next #3)

    [Socket]
    ListenStream=%t/claude-notify/sock
    SocketMode=0600
    DirectoryMode=0700
    Service=claude-notifyd.service

    [Install]
    WantedBy=sockets.target
    ```
  - `claude-notifyd.service`:
    ```
    [Unit]
    Description=Claude Code notifyd (G-1.next #3)
    Documentation=https://github.com/kkiyama117/dotfiles
    Requires=claude-notifyd.socket

    [Service]
    Type=notify
    NotifyAccess=main
    ExecStart=%h/.local/bin/claude-notifyd
    Restart=on-failure
    RestartSec=10
    ProtectSystem=strict
    ProtectHome=read-only
    ReadWritePaths=%t/claude-notify
    PrivateTmp=true
    NoNewPrivileges=true

    [Install]
    WantedBy=default.target
    ```
- [ ] **D4.4 `Type=notify` 対応 (sd_notify)**
  - `coreos/go-systemd/v22/daemon` の `daemon.SdNotify(false, daemon.SdNotifyReady)` を `Server.Serve` 内で呼ぶ
  - もしくは依存追加を避けるため `NOTIFY_SOCKET` env 経由で stdlib socket 直接書きで済ませる (PR-D3 内で判定)
- [ ] **D4.5 chezmoi bootstrap**
  - `.chezmoiscripts/run_onchange_after_enable-claude-notifyd.sh.tmpl`:
    ```
    #!/usr/bin/env bash
    # unit-sha256: {{ include "<unit_path>" | sha256sum }}
    # binary-marker: {{ include "programs/claude-tools/cmd/claude-notifyd/main.go" | sha256sum }}
    set -euo pipefail
    systemctl --user daemon-reload
    systemctl --user enable --now claude-notifyd.socket
    ```
- [ ] **D4.6 docs 更新**
  - `programs/claude-tools/README.md`: `claude-notifyd` セクション + arch 図
  - `docs/manage_claude.md` §5: 新 pipeline 図 + daemon 説明
  - `docs/keybinds.md` §通知 daemon: `claude-notifyd.socket` 言及追加
  - `docs/todos.md`:
    - `G-1.next #3` を `[x]` 化 + 完了サマリ
    - `F-3.next L33` を `[x]` 化 (本 plan で吸収)
- [ ] **D4.7 smoke**
  - `chezmoi diff` で 4 binary + 2 unit + 1 script の差分を確認
  - `chezmoi apply` 実行
  - `systemctl --user is-active claude-notifyd.socket` → `active`
  - `systemctl --user is-active claude-notifyd.service` → `inactive` (まだ起動してない)
  - 別 shell で `printf '{"v":1,"op":"show","sid":"smoke","title":"NotifydSmoke","body":"socket activation","urgency":"normal"}\n' | nc -U $XDG_RUNTIME_DIR/claude-notify/sock`
  - daemon が socket activation で立ち上がり popup 表示
  - `systemctl --user is-active claude-notifyd.service` → `active`
  - 同 sid で連続発火 → in-place 更新
  - `systemctl --user kill -s SIGSEGV claude-notifyd` → daemon 落ちる → 次の hook 経由で fallback dispatch が動いて popup 表示
  - 10s 後 daemon が systemd Restart で復活 → 再度 socket dispatch
  - 結果を `docs/superpowers/smoke/2026-05-02-notifyd-smoke.md` に追記
- [ ] **D4.8 commit**: `feat(g1): D-4 wire claude-notify-hook to claude-notifyd + systemd unit`

### Task 5: 完走チェックポイント

- [ ] **5.1 `cd programs/claude-tools && go test -race -cover ./...` 全 PASS / `internal/notifyd/` coverage ≥ 80%**
- [ ] **5.2 `chezmoi diff` でゴミ差分なし**
- [ ] **5.3 `journalctl --user -t claude-notifyd --since '5 min ago'` で起動ログ + Notify ログが見える**
- [ ] **5.4 1 週間運用観察** (本 plan の merge 後): `journalctl --user -t claude-notify-dispatch --since '7 days ago'` で fallback 起動が 0 件 (= daemon が安定動作している) なら legacy dispatch deprecate を別 plan で検討
- [ ] **5.5 `docs/superpowers/smoke/2026-05-02-notifyd-smoke.md` 完走記録**

---

## Risks / Mitigations

| リスク | 影響 | 緩和策 |
|---|---|---|
| godbus 起因の goroutine leak | daemon RSS 増加 | dbus_test で 1000 cycle / `runtime.NumGoroutine()` を比較する leak test |
| socket activation の起動レース | 最初の hook で popup ロスト | `claude-notifyd.socket` を `enable --now` し socket は常時 LISTEN。daemon 起動中の 2 件目以降は systemd 側で queue |
| sd_notify 依存追加 vs stdlib | `coreos/go-systemd` 追加で deps が増える | stdlib `net.Dial("unixgram", os.Getenv("NOTIFY_SOCKET"))` で `READY=1\n` を書く方式を優先 |
| flock 残骸 | daemon 異常終了で lock が残る | `syscall.Flock` は fd close で自動解放、`Restart=on-failure` で再取得 |
| in-flight popup ロスト (daemon restart) | 再起動前 popup が click 無反応 | spec §4.7 に明記。1 popup 損失は受容可能、自動補修はしない |
| Type=notify が動作しない環境 | systemd 起動が READY 待ちで stuck | unit を `Type=simple` に降格する fallback (PR-D4 内で実機確認) |
| `notify-send` から `dbus.Notify` 直接化で hint 仕様差 | wired-notify 側で popup 形状が崩れる | spec §4.5 通り `notify-send` フラグ群と byte-equivalent な D-Bus call を生成 (`x-claude-session` hint 含む) |

## Acceptance Criteria

- [ ] `programs/claude-tools/cmd/claude-notifyd/` の build + `-race` test が PASS
- [ ] `chezmoi apply` 後 `systemctl --user is-active claude-notifyd.socket` が `active`
- [ ] hook → socket → daemon → wired-notify popup → 左クリック focus が一連で動作
- [ ] daemon kill → 次 hook で `claude-notify-dispatch` fallback が popup を出す
- [ ] disk state file `<state_dir>/sessions/<sid>.id` の format が PR-9 時代と byte-equivalent
- [ ] `docs/todos.md` G-1.next #3 / F-3.next L33 が `[x]`
- [ ] `internal/notifyd/` coverage ≥ 80%
