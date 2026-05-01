# Go Cockpit Smoke Test — 2026-05-01

**対象:** Phase 1 (B サブシステム) — 5 binary を Go で 1:1 置換完了後の実機検証
**実行者:** kiyama
**実行日:** 2026-05-XX (実施日に合わせて更新)

## Pre-conditions

- [ ] `~/.local/bin/claude-cockpit-{state,prune,summary,next-ready,switcher}` 5 binary が存在 (chezmoi apply 完了後)
- [ ] 旧 shell 5 本が repo 内・filesystem 上から消えている
- [ ] `chezmoi apply` がエラーなく完走
- [ ] `cd programs/claude-tools && go test ./...` が all PASS

## 8-Step Smoke (`docs/manage_claude.md` "Cockpit State Tracking" 節 準拠)

| # | Step | Expected | Result | Notes |
|---|---|---|---|---|
| 1 | 新 tmux session で Claude を起動、何か入力 | hook 経由で `~/.cache/claude-cockpit/panes/<sess>_<paneID>.status` が `working` で生まれる | TBD | |
| 2 | Stop event を待つ (Claude が応答完了) | 同ファイルが `done` に更新 | TBD | |
| 3 | tmux status-right を確認 | `⚡ N ⏸ M ✓ K ` 形式で表示 (cache の集計反映) | TBD | |
| 4 | `prefix + C → s` で switcher 起動 | fzf popup が開き、session/window/pane の tree + バッジ表示 | TBD | |
| 5 | 任意 pane を Enter で選択 | tmux が switch-client + select-window + select-pane で移動 | TBD | |
| 6 | switcher で Ctrl-X → 'y' | 該当 scope (pane / window / session) が kill される。W が claude-managed なら worktree 削除も実行 | TBD | |
| 7 | done state pane を 2 つ以上作り `prefix + C → N` で循環 | inbox 順 (session asc / window-idx asc / pane-idx asc) で次の done pane に jump | TBD | |
| 8 | `tmux kill-server` 後に再起動 | server-start hook (`run -b prune`) が orphan cache を回収、status-right が空になる | TBD | |

## Errors / Surprises

(発生した unexpected behavior をここに列挙。無ければ "なし")

## journalctl Errors

```bash
journalctl --user --since="2 hours ago" | grep -E "claude-cockpit-(state|prune|summary|next-ready|switcher)" | grep -i error
```

(実行結果を貼る。`logger -t` 経由の ERROR が出ていなければ正常)

## Go/No-Go Decision for Phase 2 (A サブシステム = notify pipeline)

- [ ] **GO** — Phase 1 が問題なく動作。Phase 2 plan を `2026-05-XX-shell-to-go-migration-phase2.md` として作成し、A サブシステム 4 binary (`notify-{cleanup,sound,hook,dispatch}`) の置換に進む。
- [ ] **NO-GO** — Phase 1 で許容できない問題が発生。詳細を本ファイルに追記し、`docs/todos.md` G-1 の状態を見直す。Go binary は revert せず shell + Go 共存で当面運用 (notify は shell のまま、cockpit は Go のまま)。

判定: TBD (smoke 全 step 完了後に決定)

判定理由: TBD
