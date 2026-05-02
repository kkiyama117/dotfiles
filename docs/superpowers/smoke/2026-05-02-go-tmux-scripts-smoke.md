# C subsystem (tmux scripts) Go migration smoke log

## C-1: claude-branch — 2026-05-02

- [x] go test -race ./cmd/claude-branch/... ./internal/gitwt/... — PASS
- [x] go test -race ./... — PASS (全パッケージ)
- [x] go build ./cmd/claude-branch — OK
- [x] chezmoi diff — status.conf 1 行差分のみ確認
- [ ] (manual) chezmoi apply → tmux source-file → status-right `[<branch>] ` 目視

## C-2: claude-respawn-pane — 2026-05-02

- [x] go test -race ./cmd/claude-respawn-pane/... ./internal/tmux/... — PASS
- [x] go test -race ./... — PASS (全パッケージ)
- [x] chezmoi diff — bindings.conf 1 行差分のみ確認
- [ ] (manual) 2-pane window: claude pane respawn 確認
- [ ] (manual) 1-pane window: current pane fallback 確認

## C-3: claude-kill-session — 2026-05-02

- [x] go test -race ./cmd/claude-kill-session/... ./internal/tmux/... ./internal/gitwt/... — PASS
- [x] go test -race ./... — PASS (22 パッケージ全 PASS、新規 cmd/claude-kill-session 含む)
- [x] go vet ./... — clean
- [x] go build ./cmd/claude-kill-session — OK (ELF x86-64 statically linked)
- [x] chezmoi diff `~/.config/tmux/conf/bindings.conf` — L86 が `~/.local/bin/claude-kill-session` に切替わっていることを確認
- [x] 旧 `dot_config/tmux/scripts/executable_claude-kill-session.sh` を `git rm` で撤去
- [ ] (manual) managed=yes window: kill + worktree remove + cache cleanup OK
- [ ] (manual) non-claude window: refuse + display-message OK
- [ ] (manual) fallback (pane_current_path): test 用に @claude-* tag を unset した window で確認

## ★ C 中間チェックポイント (PR-C-3 完走後) — 2026-05-02

- [ ] (manual) Step CK.1: C-1〜C-3 通し smoke (status-right `[<branch>] ` / `prefix + C → r` / `prefix + C → x`)

## C-3 後の go/no-go: GO (2026-05-02)

automated 層 (unit + race + vet + build + chezmoi diff) は全 GREEN。実機 tmux 目視 smoke は user 手動にデファ。
