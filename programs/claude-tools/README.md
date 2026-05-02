# claude-tools

chezmoi-managed Go binaries that replace the shell scripts under
`dot_local/bin/executable_claude-*.sh` and
`dot_config/tmux/scripts/cockpit/executable_*.sh`.

## Layout

- `cmd/<name>/main.go` — thin entry points (1 binary per former shell script)
- `internal/{cockpit,xdg,atomicfile,proc,obslog}` — shared packages
  (Phase 2 で `internal/notify` を追加予定)

## Build & Deploy

Distributed via chezmoi's `run_onchange_after_build-claude-tools.sh.tmpl`:

    chezmoi apply  # rebuilds binaries to ~/.local/bin/ when source changes

To build manually:

    go build -trimpath -ldflags="-s -w" -o ~/.local/bin/ ./cmd/...

## Test

    go test ./...

## Spec

See `docs/superpowers/specs/2026-05-01-shell-to-go-migration-design.md`
in the chezmoi repo root.
