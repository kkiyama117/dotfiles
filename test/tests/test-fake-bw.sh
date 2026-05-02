#!/usr/bin/env bash
# Self-test for test/fake-bw. Run from repo root or anywhere — paths self-resolve.
set -euo pipefail

SHIM="$(cd "$(dirname "$0")/.." && pwd)/fake-bw"
FIX="$(cd "$(dirname "$0")/.." && pwd)/fixtures/bw-item.json"
export FAKE_BW_FIXTURE="$FIX"

fail=0
assert_eq() {
  local label=$1 want=$2 got=$3
  if [ "$want" = "$got" ]; then
    echo "PASS: $label"
  else
    echo "FAIL: $label"
    echo "  want: $want"
    echo "  got:  $got"
    fail=1
  fi
}

# 1. status is unlocked, JSON
out=$("$SHIM" status)
assert_eq "status.status" "unlocked" "$(printf '%s' "$out" | jq -r '.status')"

# 2. unlock --raw returns a non-empty token
out=$("$SHIM" unlock --raw)
[ -n "$out" ] && echo "PASS: unlock.raw non-empty" || { echo "FAIL: unlock.raw empty"; fail=1; }

# 3. get item <any-id> returns the fixture
out=$("$SHIM" get item any-id)
assert_eq "get.item.id" "test-bw-item" "$(printf '%s' "$out" | jq -r '.id')"
assert_eq "get.item.fields.email" "test@example.com" \
  "$(printf '%s' "$out" | jq -r '.fields[] | select(.name=="email") | .value')"

# 4. list items returns a JSON array of length 1
out=$("$SHIM" list items --search anything)
assert_eq "list.length" "1" "$(printf '%s' "$out" | jq 'length')"

# 5. unhandled subcommand returns {} on stdout, exits 0
out=$("$SHIM" totally-unknown 2>/dev/null)
assert_eq "unknown.stdout" "{}" "$out"

exit "$fail"
