#!/usr/bin/env bash
# Phase A smoke runner. Runs inside the chezmoi-test container.
set -euo pipefail

SRC=/work
DEST="$(mktemp -d)"

mkdir -p "$HOME/.config/chezmoi"
cat > "$HOME/.config/chezmoi/chezmoi.toml" <<'EOF'
[data]
bitwardenOpenRouterItemId = "test-bw-item"
EOF

cd "$SRC"

echo "[smoke] rendering every *.tmpl with chezmoi execute-template"
# Use process substitution so the while loop runs in the parent shell and
# `set -e` reliably aborts on the first failing template.
# Exclude Chezmoi meta-files (.chezmoi*.toml.tmpl, .chezmoiexternal.toml.tmpl, etc.)
while IFS= read -r -d '' f; do
  if ! chezmoi --source="$SRC" execute-template < "$f" >/dev/null; then
    echo "[smoke] FAILED rendering $f" >&2
    exit 1
  fi
done < <(find . -name '*.tmpl' -type f -not -path './.git/*' -not -name '.chezmoi*.toml.tmpl' -print0)

echo "[smoke] running chezmoi diff against ephemeral destination"
# diff returns non-zero (typically 1) when there ARE differences — that is
# expected, since the destination is empty. Treat rc>1 as a real crash.
err_log="$(mktemp)"
set +e
chezmoi --source="$SRC" --destination="$DEST" diff --no-tty >/dev/null 2>"$err_log"
rc=$?
set -e
if [ "$rc" -gt 1 ]; then
  echo "[smoke] chezmoi diff crashed (rc=$rc), stderr follows:" >&2
  cat "$err_log" >&2
  exit "$rc"
fi
rm -f "$err_log"

echo "[smoke] OK"
