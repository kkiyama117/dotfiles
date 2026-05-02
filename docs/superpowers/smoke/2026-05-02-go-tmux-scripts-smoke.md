# C subsystem (tmux scripts) Go migration smoke log

## C-1: claude-branch — 2026-05-02

- [x] go test -race ./cmd/claude-branch/... ./internal/gitwt/... — PASS
- [x] go test -race ./... — PASS (全パッケージ)
- [x] go build ./cmd/claude-branch — OK
- [x] chezmoi diff — status.conf 1 行差分のみ確認
- [ ] (manual) chezmoi apply → tmux source-file → status-right `[<branch>] ` 目視
