# `claude-notifyd` Resident Helper Daemon Design (G-1.next #3)

**Status:** Draft (2026-05-02)
**Branch:** `feat/notify-dispatch-daemon`
**Phase:** G-1.next #3 (post Shell→Go Phase 2)

**Parent items:**
- `docs/todos.md` L33 (F-3.next: dispatcher を 1 本の常駐 helper daemon に集約 / 案 B)
- `docs/todos.md` L236 (G-1.next #3: notify-dispatch daemon 化 — 別 spec に切り出す)
- `docs/superpowers/specs/2026-04-30-wired-click-actions-design.md` §74 / §359
- `docs/superpowers/plans/2026-05-02-shell-to-go-migration-phase2.md` (1:1 Go 置換は完了済)

**Related implementation plan:** `docs/superpowers/plans/2026-05-02-notify-dispatch-daemon.md`

---

## 1. Problem

現状 (G-1 Phase 2 完了後 / `8489f47` 時点) の notify pipeline は次の形:

```
Claude Code ─┐ (event)
             ▼
   claude-notify-hook (Go, exit 0 fast)
             │
             ├── fork: claude-notify-sound (5s 程度)
             │
             └── setsid fork: claude-notify-dispatch
                        │
                        ▼
                 notify-send --print-id --wait  ←─ blocks for minutes/hours
                        │
                        ▼
              wired-notify (D-Bus daemon, 既存)
                        │
                        ▼
                 ActionInvoked / NotificationClosed
                        │
                        ▼
               tmux focus / WM focus / gdbus CloseNotification
```

これにより以下の摩擦が積もっている:

1. **N popup = N プロセス**: 同時に開いている session 数だけ `notify-send --wait` が常駐し、各々 dispatcher プロセスを抱え込む。長時間の作業 (6h/日) で `ps -ef | grep notify-` が 10 行以上になることがある。
2. **状態が 2 系列で分散**: replace-id は `<state_dir>/sessions/<sid>.id` (disk) に永続するが、in-flight popup の notif_id ↔ session_id 対応はプロセスローカル。daemon が落ちると Track 不能。
3. **D-Bus 接続が popup 数だけ作られる**: 各 dispatcher が個別に session bus に繋ぎ ActionInvoked を listen している (gdbus subprocess 経由のため正確にはコネクションは短命だが、`gdbus monitor` で観測すると "open / close" がノイズになる)。
4. **session 横断のロジック追加が困難**: 例えば「同 git project の popup を group 化」「3 件以上溜まったら要約 popup に置換」など F-3.next の伸張アイデアは、すべて `dispatch` がプロセス局所な現状では実装できない。
5. **観測性の限界**: `journalctl --user -t claude-notify-dispatch` が popup 単位のログしか出さず、pipeline 全体のヘルスを 1 ファイルで追えない。

## 2. Goal

- popup state (replace-id table / in-flight notif_id ↔ session_id ↔ tmux ctx) を **1 プロセスに集約**する `claude-notifyd` を導入。
- D-Bus session bus に **1 接続だけ** 張り、`ActionInvoked` / `NotificationClosed` を全 popup について受ける。
- `claude-notify-hook` は **fire-and-forget で 1 IPC** だけ送って即 exit 0 (現行と同じ Hook 契約を維持)。
- daemon 死亡時は **既存の `claude-notify-dispatch` 経路にフォールバック** し、Hook → popup の到達は劣化しない。
- 既存 disk 形式 (`<state_dir>/sessions/<sid>.id`) は **完全互換**。`claude-notify-cleanup.timer` をそのまま流用。

### Non-goals (本 spec の範囲外)

- 複数 host / 複数ユーザー対応 (per-UID instance のみ)
- popup の集計 / 要約 / グルーピング (上層 feature。socket protocol の余白だけ確保)
- D-Bus 経由で daemon を直接呼び出す API (hook ↔ daemon は Unix socket。daemon ↔ wired は既存 D-Bus を使うのみ)
- Wayland / X11 共存以外の WM (現行 dispatch と同じ)

## 3. Decision: 案 B (常駐 daemon + Unix socket) を採用

ブレスト時に検討した 2 案 (`docs/superpowers/specs/2026-04-30-wired-click-actions-design.md` §6 follow-ups) のうち、案 B (常駐 daemon に集約) を以下の理由で採用する。

| 観点 | 案 A (現行: per-popup dispatcher) | 案 B (本案: 常駐 daemon) |
|---|---|---|
| プロセス数 | popup 数 + sound 一瞬 | 常駐 1 + sound 一瞬 |
| 状態の集約性 | disk のみ / プロセス局所 in-flight | in-memory + disk (永続は同形式) |
| F-3.next 伸張余地 | 困難 (cross-popup ロジックが書けない) | 容易 (daemon 内で table 操作) |
| Hook 経路の劣化 | なし | socket 接続失敗時 fallback で同等 |
| 新規依存 | 0 | `net` (stdlib) + 任意で `godbus` |
| 起動コスト | popup ごとに dispatch fork | systemd socket activation で常駐 |
| 観測性 | `-t claude-notify-dispatch` 個別 | `-t claude-notifyd` 単一線 |
| systemd unit 数 | 0 (timer 1 のみ) | +2 (`.service` + `.socket`) |
| 失敗時の劣化幅 | 1 popup ロスト | daemon down → fallback で popup 1 個遅延無し |

**判断**: トレードオフは systemd unit が 2 増えること / godbus 依存追加 (任意) に集中するが、いずれも本 repo の既存パターン (`wired.service` / `claude-notify-cleanup.{service,timer}`) と同種でコストが低い。一方で daemon 化で得られる observability と将来拡張余地は質的に大きい。

## 4. Architecture

```
Claude Code ─┐ (event)
             ▼
  claude-notify-hook (Go, exit 0 fast)
             │
             ├── fork: claude-notify-sound
             │
             └── short-write to ${XDG_RUNTIME_DIR}/claude-notify/sock
                        │
                        ▼  (newline-delimited JSON, single-shot)
              ┌─────────────────────────────┐
              │  claude-notifyd (常駐, 1 ps) │
              │   ┌──────────────────────┐  │
              │   │ Listener Goroutine   │  │
              │   │ (UnixListener.Accept)│  │
              │   └──────────┬───────────┘  │
              │              │ chan req     │
              │   ┌──────────▼───────────┐  │
              │   │ State Goroutine      │  │
              │   │  - sid → notif_id    │  │
              │   │  - notif_id → ctx    │  │
              │   │  - replace-id disk   │  │
              │   └──────────┬───────────┘  │
              │              │              │
              │   ┌──────────▼───────────┐  │
              │   │ D-Bus Conn (godbus)  │  │
              │   │  Notify(replace_id)  │  │
              │   │  Listen ActionInvoked│  │
              │   │  Listen Closed       │  │
              │   └──────────┬───────────┘  │
              │              │ chan signal  │
              │   ┌──────────▼───────────┐  │
              │   │ Action Goroutine     │  │
              │   │ tmux switch / focus  │  │
              │   │ WM focus (xdotool)   │  │
              │   │ gdbus CloseNotification│
              │   └──────────────────────┘  │
              └─────────────────────────────┘
                            │
                            ▼ (fallback path: socket connect failed)
              setsid fork: claude-notify-dispatch (legacy, 既存 Go binary)
```

### 4.1 Process model

- **`claude-notifyd`** (新規 long-lived daemon)
  - 起動: systemd socket activation (`claude-notifyd.socket` LISTEN_FDS=1)。手動起動も可 (`claude-notifyd --listen <path>`)
  - 終了: SIGTERM 受領で listener close → 既存 popup の signal 待ちを最大 N 秒 grace、超過で強制 exit
  - 並行性: 1 process / per-UID。`flock` で `<state_dir>/notifyd.lock` を取り、二重起動を排除
  - 失敗時挙動: `Restart=on-failure` + `RestartSec=10s`。socket activation は上流に socket を残す
- **`claude-notify-hook`** (既存 Go binary、改修)
  - **新規ロジック**: `forkDispatch` の前に `dialDaemon(socketPath, timeout=100ms)` を試行
    - 成功: protocol frame を 1 行書いて即 close、return (sound fork のみ並走)
    - 失敗: 既存 `setsid fork claude-notify-dispatch` を実行
  - hook 契約 (always exit 0, recover panic) は維持
- **`claude-notify-dispatch`** (既存 Go binary、改修なし)
  - daemon が落ちている / socket がない緊急 fallback として温存
  - 将来 daemon が安定したら deprecate を検討 (本 spec の対象外)
- **`claude-notify-sound`** / **`claude-notify-cleanup`**: 改修なし

### 4.2 Socket layout

| 項目 | 値 |
|---|---|
| パス | `${XDG_RUNTIME_DIR}/claude-notify/sock` (絶対パス、`StateDir()` と同階層) |
| 種別 | `SOCK_STREAM` Unix socket |
| 権限 | mode `0600` / owner UID のみ |
| activation | systemd `.socket` unit で `ListenStream=` |
| protocol | newline-delimited JSON (1 frame / connection、双方向待ち合わせなし) |

### 4.3 Wire protocol (v1)

#### Hook → Daemon: `Show`

```json
{
  "v": 1,
  "op": "show",
  "sid": "abc-123-def",
  "title": "Claude Code · chezmoi/feat-notify-dispatch-daemon",
  "body": "Turn complete",
  "urgency": "normal",
  "tmux_pane": "%42",
  "tmux_session": "chezmoi"
}
```

- すべて UTF-8、1 行で送出 (区切り `\n`)
- `v` は protocol version。daemon が知らない `v` は ack を返さず close (hook 側 fallback 起動)
- daemon は受領後 ack を **返さない** (一方向 fire-and-forget)。失敗観測は journal 経由で行う
- daemon 内部の処理が遅延しても hook はブロックしない (socket buffer に乗ったら close)

#### Daemon → Hook: なし

- D-Bus signal 経由で action を直接拾うため、hook へ返す情報はない
- 将来 (`v >= 2`) で `query` op など追加するなら同 protocol を流用予定

### 4.4 State management

| 状態 | 場所 | 形式 | 永続化 |
|---|---|---|---|
| `sid → notif_id` (replace-id) | in-memory `map[string]uint32` + 既存 disk file | `<state_dir>/sessions/<sid>.id` | atomicfile.Write (既存) |
| `notif_id → notif_ctx` (in-flight popup の tmux ctx 等) | in-memory `map[uint32]notifContext` | -- | 揮発 (daemon 再起動でロスト) |
| daemon PID lock | `<state_dir>/notifyd.lock` | flock | 永続 |

- daemon 起動時に `<state_dir>/sessions/*.id` をスキャンしてメモリへロード (warm start)
- `Show` 受領のたびに in-memory replace-id を引き、`Notify(replace_id=...)` 呼び出し → 戻り値 `id` を `sid → id` で更新 + disk へ atomic write
- in-flight ctx (notif_id → tmux pane / session) は `Notify` 呼び出し時にメモリ追加、`ActionInvoked` / `NotificationClosed` 受領時に削除
- daemon 再起動: in-flight ctx は失われるが、ユーザー視点では「再起動前の popup を click しても何も起きない」程度。次の popup から正常動作

### 4.5 D-Bus integration

候補は 2 つ:

- **(a) godbus/dbus/v5 直接利用** — Phase 2 plan で一度検討して採用見送りした依存。daemon 化時は採用余地が再浮上 (常駐なら 1 接続コストが希薄)
- **(b) `notify-send --print-id --wait` を daemon 内から長期 spawn** — shell parity を維持。subprocess 群の管理が daemon の責務になる

**判断: (a) godbus 採用**。理由:

1. daemon 内では D-Bus 接続が **1 本** で全 popup を捌けるため、subprocess 群を抱える (b) より資源効率が良い
2. signal listen を `dbus.AddMatchSignal` で 1 度だけ登録する設計は、(b) より bug が少ない (subprocess fork の race / SIGCHLD 取り扱いが消える)
3. Phase 2 plan の `godbus` 見送り理由 (per-popup プロセス × 個人 dotfile YAGNI) は daemon 化で前提が変わる
4. fallback path として既存 dispatch (subprocess 経由) が残るため、godbus 採用が daemon 死亡時の劣化を増やさない

#### 4.5.1 接続の resilience

- `dbus.SessionBus()` 失敗時は ConnectivityError で daemon 起動失敗 (systemd Restart に委ねる)
- 接続途絶 (`Disconnect` signal) を監視し、最大 5 回 / 指数 backoff で reconnect
- reconnect 成功時に in-memory state を保ったまま継続。in-flight notif_id は孤児になる可能性があるため、`NotificationClosed` の取りこぼしに対しては「30 分後に強制 GC」を設ける

### 4.6 Action handling

ActionInvoked が来たら:

1. notif_id → notif_ctx を引く (なければ無視)
2. `ctx.action_key == "default"` のとき:
   - `tmux switch-client -t <session>` + `select-pane -t <pane>` (`tmux has-session` で守る)
   - WM focus (xdotool / swaymsg) — 既存 `focusWM` ロジックを `internal/notify` に切り出して daemon と dispatch で共有
   - `gdbus CloseNotification` (or `bus.Object(...).Call("CloseNotification", id)`)
3. それ以外の action key は v1 では未使用 (将来拡張)

NotificationClosed が来たら:

1. notif_id を notif_ctx map から削除
2. 同 sid の `replace-id` は disk に残す (次の popup で再利用可能なため)

### 4.7 Failure modes & fallback

| シナリオ | 検知 | 挙動 |
|---|---|---|
| daemon 起動失敗 (D-Bus 不在) | systemd Restart 5 連敗 | hook → socket connect refused → 既存 dispatch fallback |
| daemon クラッシュ in-flight | systemd Restart=10s | 落ちた瞬間の popup は notif_id ロスト (click しても無反応)。次以降は復活 |
| socket 接続 timeout | hook 100ms |  fallback dispatch |
| socket activation 競合 | systemd | systemd 側で serialize |
| disk write 失敗 | atomicfile error | Warn ログ + skip (次回 click が --replace-id なし popup を出す) |
| godbus reconnect 失敗 5 回 | daemon 内 backoff | exit 1 → systemd Restart |

## 5. Files

### 新規 (本 spec 由来 / 別 plan で実装)

| パス | 役割 |
|---|---|
| `programs/claude-tools/cmd/claude-notifyd/main.go` | daemon entry: flags / signal handling / lifecycle |
| `programs/claude-tools/cmd/claude-notifyd/main_test.go` | flag parsing / lifecycle smoke (FakeRunner) |
| `programs/claude-tools/internal/notifyd/server.go` | listener loop / connection handling / state goroutine |
| `programs/claude-tools/internal/notifyd/server_test.go` | protocol round-trip / replace-id load/save / concurrent connections |
| `programs/claude-tools/internal/notifyd/dbus.go` | godbus wrapper: Notify / signal subscribe / reconnect (interface 化で test 可) |
| `programs/claude-tools/internal/notifyd/dbus_test.go` | FakeBus で `Notify` round-trip / signal dispatch |
| `programs/claude-tools/internal/notifyd/protocol.go` | wire protocol type / Marshal/Unmarshal / version negotiation |
| `programs/claude-tools/internal/notifyd/protocol_test.go` | json round-trip / 未知 version 拒否 / malformed input |
| `dot_config/systemd/user/claude-notifyd.service` | `Type=notify` + `Restart=on-failure` |
| `dot_config/systemd/user/claude-notifyd.socket` | `ListenStream=%t/claude-notify/sock` + `SocketMode=0600` |
| `.chezmoiscripts/run_onchange_after_enable-claude-notifyd.sh.tmpl` | `daemon-reload` + `enable --now claude-notifyd.socket` (idempotent / hash-tracked) |
| `docs/superpowers/plans/2026-05-02-notify-dispatch-daemon.md` | PR 単位の実装計画 |
| `docs/superpowers/smoke/2026-05-02-notifyd-smoke.md` | 完走 smoke 結果 |

### 変更

| パス | 変更内容 |
|---|---|
| `programs/claude-tools/go.mod` | `github.com/godbus/dbus/v5` を追加 |
| `programs/claude-tools/go.sum` | 同上 |
| `programs/claude-tools/cmd/claude-notify-hook/main.go` | `forkDispatch` 前に socket dispatch を試行する関数追加 + 既存 dispatch を fallback に降格 |
| `programs/claude-tools/cmd/claude-notify-hook/main_test.go` | socket path への送信 / 接続失敗時の fallback / timeout 経路を table で網羅 |
| `programs/claude-tools/internal/notify/state.go` | (任意) `LoadAllReplaceIDs(stateDir) map[string]uint32` を export — daemon の warm start 用 |
| `programs/claude-tools/internal/notify/state_test.go` | 新 export のテスト追加 |
| `programs/claude-tools/internal/notify/focus.go` (新規移動) | `claude-notify-dispatch/main.go` から `focusTmux` / `focusWM` / `closeNotification` を切り出して共有 |
| `programs/claude-tools/cmd/claude-notify-dispatch/main.go` | 切り出し後の薄いエントリ (互換維持。実装は `internal/notify/focus.go`) |
| `programs/claude-tools/README.md` | `claude-notifyd` セクション追記 |
| `docs/manage_claude.md` | §5 notify pipeline 図を daemon 込みに更新 |
| `docs/keybinds.md` | §通知 daemon 周辺の動線を更新 |
| `docs/todos.md` | G-1.next #3 を `[x]` 化 / F-3.next #L33 を `[x]` 化 |

### 削除

なし (本 spec では legacy dispatch を温存)

## 6. Test plan

### 6.1 Unit (Go)

- protocol json round-trip / unknown `v` 拒否 / malformed line の skip
- `internal/notifyd/server`: 100 件の concurrent client connect で no race (`-race` 付与)
- replace-id warm-start: 既存 disk file → in-memory load 後に `Show` で再利用される
- D-Bus reconnect: FakeBus で disconnect signal → backoff → 再接続 round-trip
- hook fallback: dial timeout / connection refused / EPIPE 各々で `claude-notify-dispatch` を起動

### 6.2 Integration (real systemd / D-Bus)

- `systemctl --user start claude-notifyd.socket` → daemon 未起動状態で hook 発火 → socket activation で daemon 起動 → popup 1 つ表示
- 同 sid で 5 回連発 → popup が in-place 更新 (notif_id がインクリメントしない)
- 異なる sid で 3 件並走 → 各 popup 独立に click できる
- daemon kill → 新 hook が fallback dispatch を起動 → popup 表示
- daemon restart → 既存 popup は無反応、新 popup は正常

### 6.3 Smoke (実機)

`docs/superpowers/smoke/2026-05-02-notifyd-smoke.md` に記録:

- Claude Code Stop event × 3 → popup 表示 / 左クリック focus / 右クリック close 各 1 回確認
- daemon を `systemctl --user kill -s SIGSEGV claude-notifyd` で殺す → 次の hook が fallback で popup → daemon が systemd Restart で復活 → 再度 popup
- `journalctl --user -t claude-notifyd` で 1 ファイルにライフサイクル全ログが出る

## 7. Rollout

1. 実装 PR は `docs/superpowers/plans/2026-05-02-notify-dispatch-daemon.md` に従い PR-D1 〜 PR-D4 で段階投入
2. 全 PR merge 後 `chezmoi apply` で `~/.local/bin/claude-notifyd` + systemd unit を配置
3. `systemctl --user daemon-reload && systemctl --user enable --now claude-notifyd.socket` (bootstrap 自動化)
4. smoke 1 周
5. 1 週間 fallback dispatch が起動した形跡 (`journalctl -t claude-notify-dispatch`) がないことを確認
6. (将来別 plan) legacy `claude-notify-dispatch` の deprecate / 削除を検討

## 8. Open questions

1. **daemon の grace shutdown 上限**: SIGTERM 受領後、in-flight signal を最大何秒待つか。提案 5s
2. **`v >= 2` で query op を入れるか**: 例えば「現在の in-flight popup 数を返す」など。tmux statusbar で活用余地あり。本 spec は v1 のみ
3. **socket activation 不在環境**: WSL2 / sshd セッション直など systemd --user が動かないケースで daemon を手動起動する手順を README に書くか
4. **`internal/notify/focus.go` への切り出し範囲**: `focusWM` の `terminalClasses` const は daemon と dispatch で共有するが、将来 `~/.config/claude-notify/wm.toml` に外出しする余地

## 9. Acceptance criteria

- [ ] `programs/claude-tools/cmd/claude-notifyd/` が `go test -race ./...` で全 PASS
- [ ] `chezmoi apply` 後、`systemctl --user is-active claude-notifyd.socket` が `active`
- [ ] Claude Code → hook → daemon 経由で popup が表示される
- [ ] 左クリックで tmux switch / WM focus が動く
- [ ] daemon を kill しても次の hook が fallback で popup を表示する
- [ ] `<state_dir>/sessions/<sid>.id` の disk 形式が PR-9 時代と byte-equivalent
- [ ] `docs/todos.md` G-1.next #3 / F-3.next L33 が `[x]`
- [ ] `programs/claude-tools/internal/notifyd/` の coverage ≥ 80%
