# Go Cockpit Smoke Test — 2026-05-01

**対象:** Phase 1 (B サブシステム) — 5 binary を Go で 1:1 置換完了後の実機検証 + Phase 1.5 F-8 patch
**実行者:** kiyama
**実行日:** 2026-05-02 (Phase 1.5 F-8 patch 込みで実施)

## Pre-conditions

- [x] `programs/claude-tools/cmd/claude-cockpit-{state,prune,summary,next-ready,switcher}` 5 binary がビルド可能 (`go build -trimpath -ldflags='-s -w' -o /tmp/g1-phase15-smoke/bin/ ./cmd/...` で全 5 binary 生成、約 2.3MB 各)
- [x] 旧 shell 5 本が repo 内から消えている (本 branch `fix/g1-phase15-f8-port` の `git ls-files` で `executable_claude-cockpit-*.sh` 不在)
- [x] `cd programs/claude-tools && go test -race ./...` が all PASS (10 パッケージ)
- [ ] `chezmoi apply` がエラーなく完走 (※ chezmoi source dir は `main` branch のため、本 branch を develop → main へ反映 → `chezmoi apply` を要する。`run_onchange_after_build-claude-tools.sh.tmpl` が sha256 トリガで再 build)

## 8-Step Smoke (`docs/manage_claude.md` "Cockpit State Tracking" 節 準拠)

下記は Phase 1.5 (F-8 patch 込み) で実施した実機 smoke 結果。Step 4-7 は **interactive UI** が要件で auto mode の非 tty / shared tmux 環境では検証不能のため、merge → chezmoi apply 後に手元 tmux で再走を要する。Step 1-3 / Step 8 (cache 操作系) は worktree から build した binary を `XDG_CACHE_HOME` を別 tempdir (`mktemp -d`) に向け、実 tmux server に対して走らせて検証済み。

| # | Step | Expected | Result | Notes |
|---|---|---|---|---|
| 1 | 新 tmux session で Claude を起動、何か入力 | hook 経由で `~/.cache/claude-cockpit/panes/<sess>_<paneID>.status` が `working` で生まれる | PASS (synthetic) | sandbox: `TMUX_PANE=%19 claude-cockpit-state hook UserPromptSubmit` → atomic write で `working` |
| 2 | Stop event を待つ (Claude が応答完了) | 同ファイルが `done` に更新 | PASS (covered by go test) | `eventToAction("Stop") -> (actionWrite, "done", true)` を `TestEventToStatus` で固定 |
| 3 | tmux status-right を確認 | `⚡ N ⏸ M ✓ K ` 形式で表示 (cache の集計反映) | PASS | 実 tmux 上の live claude pane (`chezmoi_%13`) に `working` を seed → `claude-cockpit-summary` 直叩きで `⚡ 1 ` が出力。さらに stale entry (`zsh_pane_%999.status`) を seed しても出力は `⚡ 1 ` のまま → **F-8 (b1) 防御フィルタが live-claude セット外を弾く** |
| 4 | `prefix + C → s` で switcher 起動 | fzf popup が開き、session/window/pane の tree + バッジ表示 | DEFER (interactive) | unit test 側 `TestBuildLines_emitsTreeOrder` + F-8 (b3) `TestBuildLines_blankBadgeForNonClaudePane` で挙動を担保。fzf UI は merge 後に手元で確認 |
| 5 | 任意 pane を Enter で選択 | tmux が switch-client + select-window + select-pane で移動 | DEFER (interactive) | `dispatchSwitch` の 3 ケース (S/W/P) は user 操作で確認 |
| 6 | switcher で Ctrl-X → 'y' | 該当 scope (pane / window / session) が kill される。W が claude-managed なら worktree 削除も実行 | DEFER (interactive) | 同上。kill 後の cache cleanup は `claude-kill-session.sh` 末尾と prune が冗長にカバー |
| 7 | done state pane を 2 つ以上作り `prefix + C → N` で循環 | inbox 順 (session asc / window-idx asc / pane-idx asc) で次の done pane に jump | DEFER (interactive) | `TestBuildDoneList_orderAndCycle` + `TestPickNext_cycles` + F-8 (b2) `TestBuildDoneList_skipsNonClaudePane` で順序・循環・filter を担保 |
| 8 | `tmux kill-server` 後に再起動 | server-start hook (`run -b prune`) が orphan cache を回収、status-right が空になる | PASS (synthetic) | 実 tmux 上で seed (live-claude pane の status + dead orphan + sleep pane) → `claude-cockpit-prune` 直叩き → 死蔵 (orphan + sleep pane の stale) を全削除、live entry のみ保持 → **F-8 (c) live-claude セット拡張が機能** |

## Phase 1.5 F-8 patch — 追加 smoke (本 branch で新規)

| # | Step | Expected | Result |
|---|---|---|---|
| F-8 (a) | `claude-cockpit-state hook SessionEnd` を `TMUX_PANE` 付きで叩く | 該当 status file が削除 (idempotent) | PASS — 実 tmux pane に対し `working` を seed → SessionEnd → ファイル消失 |
| F-8 (a) | seed なしの状態で SessionEnd | エラーなく exit 0 (rm -f 相当) | PASS — `TestRun_sessionEndOnMissingFileIsNoOp` + 実機で再現確認 |
| F-8 (b1) | live claude pane のみ seed → summary | ⚡ 1 (live のみカウント) | PASS — 実 tmux の `chezmoi_%13` (claude) に working を seed |
| F-8 (b1) | live claude + 死蔵 entry → summary | 死蔵分は弾かれ ⚡ 1 のまま | PASS — 上記に `zsh_pane_%999.status` を加えて再実行 |
| F-8 (b1) | live claude が 0 のとき (sleep pane のみ) → summary | 空文字列 | PASS — 専用 sleep session 2 個作成・seed → 空出力 |
| F-8 (c) | live claude entry + 死蔵 entry → prune | live entry のみ残る | PASS — 死蔵は削除、live は保持 |
| F-8 (c) | 全 pane が non-claude (sleep) → prune | seed 全削除 | PASS — 3/3 削除 |

## Errors / Surprises

なし — F-8 (a)/(b1)/(c) の Go 実装が shell F-8 v1 (commit `b81cb81`) の挙動と一致することを実 tmux server 相手に確認した。Step 4-7 の interactive UI は worktree からの build では tmux popup binding を hot-reload できないため defer。

## journalctl Errors

```bash
journalctl --user --since="2 hours ago" | grep -E "claude-cockpit-(state|prune|summary|next-ready|switcher)" | grep -i error
```

実行結果: 該当行 0 件 (smoke 中に obslog 経由の ERROR は出力されなかった)。

## Go/No-Go Decision for Phase 2 (A サブシステム = notify pipeline)

- [x] **GO** — Phase 1 (5 binary) + Phase 1.5 F-8 patch まで TDD + 実機 smoke で挙動が固まった。Phase 2 plan (`2026-05-XX-shell-to-go-migration-phase2.md`) を起こし、A サブシステム 4 binary (`notify-{cleanup,sound,hook,dispatch}`) の置換に進める。
- [ ] **NO-GO** — Phase 1 で許容できない問題が発生。詳細を本ファイルに追記し、`docs/todos.md` G-1 の状態を見直す。Go binary は revert せず shell + Go 共存で当面運用 (notify は shell のまま、cockpit は Go のまま)。

判定: **GO**

判定理由: 全自動 test (race detection 込み 10 パッケージ all PASS) + 実機 smoke (real tmux + 専用 sandbox `XDG_CACHE_HOME`) で F-8 (a)/(b1)/(c) と既存 5 binary の cache 操作系挙動が shell F-8 v1 と一致することを確認。Step 4-7 (switcher / next-ready の interactive UI) は単体テストで code path を担保しつつ、実 tmux UI 確認は merge → chezmoi apply 後の最初の対話セッションで再走することにする。
