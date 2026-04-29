#!/usr/bin/env bash
# Idempotent TPM bootstrap. Safe to call from chezmoi run_once or by hand.
set -euo pipefail

TPM_DIR="$HOME/.config/tmux/plugins/tpm"

if [ ! -d "$TPM_DIR" ]; then
  echo "[tpm-bootstrap] cloning TPM into $TPM_DIR"
  git clone --depth 1 https://github.com/tmux-plugins/tpm "$TPM_DIR"
fi

if [ -x "$TPM_DIR/bin/install_plugins" ]; then
  echo "[tpm-bootstrap] running install_plugins"
  # tmux サーバー未起動時に install_plugins が失敗しても chezmoi apply を止めない。
  # 失敗時は tmux 起動後 prefix+I で手動インストール可能。
  "$TPM_DIR/bin/install_plugins" \
    || echo "[tpm-bootstrap] install_plugins failed; run 'prefix + I' inside tmux to retry"
fi
