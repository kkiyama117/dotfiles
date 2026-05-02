# Containerized Test Environment Implementation Plan (Phase A)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single-Dockerfile test harness that runs `chezmoi execute-template` over every `*.tmpl` and `chezmoi diff` against an empty destination, executable identically on a Manjaro workstation (`docker` or `podman`) and in GitHub Actions, with a fake `bw` shim so no real Bitwarden vault is touched.

**Architecture:** One OCI image (`test/Dockerfile`, `manjarolinux/base` digest-pinned), a POSIX-sh `fake-bw` on `$PATH`, a bash entrypoint that synthesizes `~/.config/chezmoi/chezmoi.toml` and runs the smoke checks, a `Makefile` that supports both `docker` and `podman`, and a GitHub Actions workflow that uses the same image. The repo is mounted read-only at `/work` so the host filesystem cannot be mutated.

**Tech Stack:** Docker / Podman (OCI), bash, POSIX sh, `jq`, GitHub Actions, GNU Make, chezmoi.

**Spec:** [`docs/superpowers/specs/2026-05-02-containerized-test-env-design.md`](../specs/2026-05-02-containerized-test-env-design.md)

---

## File Structure

| Path | Action | Responsibility |
|---|---|---|
| `test/Dockerfile` | Create | OCI image: Manjaro base + chezmoi + fake-bw + non-root tester user |
| `test/fake-bw` | Create | Fake `bw` CLI shim returning canned JSON |
| `test/fixtures/bw-item.json` | Create | Canned Bitwarden item JSON used by `fake-bw` |
| `test/entrypoint.sh` | Create | Generates `chezmoi.toml`, runs `execute-template` and `diff` |
| `test/Makefile` | Create | `make build|test|shell|ci-test`, `RUNTIME` switch |
| `test/tests/test-fake-bw.sh` | Create | Self-test for `fake-bw` contract |
| `.chezmoiignore` | Modify | Append `test/` and workflow path |
| `.github/workflows/container-smoke.yml` | Create | CI smoke job |

---

## Task 1: Pre-flight — resolve base image digest

**Files:**
- None to write yet; capture the digest for use in Task 4.

- [ ] **Step 1: Pull the latest `manjarolinux/base` image**

Run:
```bash
docker pull manjarolinux/base:latest
```
Expected: layers download, ends with `Status: Downloaded newer image for manjarolinux/base:latest`.

- [ ] **Step 2: Capture the digest**

Run:
```bash
docker inspect --format='{{index .RepoDigests 0}}' manjarolinux/base:latest
```
Expected: a string like `manjarolinux/base@sha256:abcdef0123...`. Save this string — it will be hard-coded into `test/Dockerfile` in Task 4.

- [ ] **Step 3: Sanity-check the image is actually Manjaro**

Run:
```bash
docker run --rm manjarolinux/base:latest sh -c 'cat /etc/os-release | grep -E "^ID="'
```
Expected: `ID=manjaro`.

- [ ] **Step 4: If pull or sanity check fails, switch to fallback**

If `docker pull manjarolinux/base:latest` errors with "manifest unknown" or the `ID=` is not `manjaro`, fall back to `archlinux:base` and document this in the Dockerfile header. In that case capture:
```bash
docker pull archlinux:base
docker inspect --format='{{index .RepoDigests 0}}' archlinux:base
```
And in Task 4 uncomment the `RUN sed -i 's/^ID=arch$/ID=manjaro/' /etc/os-release` line.

- [ ] **Step 5: Record the chosen digest in a scratch note**

No commit yet. Hold the digest string in mind / a scratch file for Task 4.

---

## Task 2: Bitwarden fixture JSON

**Files:**
- Create: `test/fixtures/bw-item.json`

- [ ] **Step 1: Create the fixtures directory**

Run:
```bash
mkdir -p test/fixtures
```

- [ ] **Step 2: Write the fixture file**

Create `test/fixtures/bw-item.json` with this exact content:

```json
{
  "object": "item",
  "id": "test-bw-item",
  "name": "TEST_ITEM",
  "notes": "TEST_NOTES",
  "login": {
    "username": "TEST_USERNAME",
    "password": "TEST_PASSWORD",
    "totp": "000000"
  },
  "fields": [
    { "name": "name",       "value": "Test User",            "type": 0 },
    { "name": "email",      "value": "test@example.com",     "type": 0 },
    { "name": "signingkey", "value": "TESTSIGNINGKEY00",     "type": 0 },
    { "name": "CODEX",      "value": "sk-test-CODEX",        "type": 1 }
  ]
}
```

Field names cover the lookups currently in this repo. Adding a missing field name later is a one-line append.

- [ ] **Step 3: Validate JSON**

Run:
```bash
jq . test/fixtures/bw-item.json >/dev/null
```
Expected: exit 0, no output.

- [ ] **Step 4: Verify lookup helpers will work**

Run:
```bash
jq -r '.fields[] | select(.name=="email") | .value' test/fixtures/bw-item.json
```
Expected: `test@example.com`.

- [ ] **Step 5: Commit**

```bash
git add test/fixtures/bw-item.json
git commit -m "test(container): add canned Bitwarden item fixture"
```

---

## Task 3: fake-bw shim — TDD

**Files:**
- Test: `test/tests/test-fake-bw.sh`
- Create: `test/fake-bw`

- [ ] **Step 1: Write the failing test**

Create `test/tests/test-fake-bw.sh` with this exact content:

```bash
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
```

Make it executable:
```bash
chmod +x test/tests/test-fake-bw.sh
```

- [ ] **Step 2: Run the test to verify it fails**

Run:
```bash
test/tests/test-fake-bw.sh
```
Expected: error like `test/fake-bw: No such file or directory` (because we have not written the shim yet). Non-zero exit.

- [ ] **Step 3: Implement `test/fake-bw`**

Create `test/fake-bw` with this exact content:

```sh
#!/bin/sh
# Minimal `bw` CLI shim used inside the chezmoi test container.
# Returns canned JSON from $FAKE_BW_FIXTURE so chezmoi templates that call
# `bitwarden`/`bitwardenFields` work without a real Bitwarden vault.
set -eu

FIXTURE="${FAKE_BW_FIXTURE:-/home/tester/fixtures/bw-item.json}"

cmd="${1:-}"
shift || true

case "$cmd" in
  status)
    printf '{"status":"unlocked","userEmail":"test@example.com"}\n'
    ;;
  unlock)
    printf '%s\n' "FAKE_SESSION_TOKEN"
    ;;
  get)
    sub="${1:-}"
    case "$sub" in
      item|password|username|notes|totp|uri)
        cat "$FIXTURE"
        ;;
      *)
        cat "$FIXTURE"
        ;;
    esac
    ;;
  list)
    printf '['
    cat "$FIXTURE"
    printf ']\n'
    ;;
  lock|sync|login|logout|config)
    printf '{}\n'
    ;;
  *)
    echo "[fake-bw] unhandled subcommand: ${cmd:-<none>}" >&2
    printf '{}\n'
    ;;
esac
```

Make it executable:
```bash
chmod +x test/fake-bw
```

- [ ] **Step 4: Run the test to verify it passes**

Run:
```bash
test/tests/test-fake-bw.sh
```
Expected: 5+ lines starting with `PASS:`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add test/fake-bw test/tests/test-fake-bw.sh
git commit -m "test(container): add fake bw shim with self-test"
```

---

## Task 4: Dockerfile

**Files:**
- Create: `test/Dockerfile`

- [ ] **Step 1: Write the Dockerfile**

Create `test/Dockerfile`. **Replace `<DIGEST_FROM_TASK_1>` with the digest captured in Task 1 step 2** (or the fallback `archlinux:base` digest, with the `sed` line uncommented).

```dockerfile
# Phase A test image for chezmoi dotfiles smoke testing.
# Base digest is pinned for determinism; refresh via:
#   docker pull manjarolinux/base:latest && \
#   docker inspect --format='{{index .RepoDigests 0}}' manjarolinux/base:latest
FROM manjarolinux/base@sha256:<DIGEST_FROM_TASK_1>

# Fallback path: if the upstream image's /etc/os-release ID is not "manjaro",
# uncomment the next line so chezmoi's .chezmoi.osRelease.id reflects Manjaro.
# RUN sed -i 's/^ID=arch$/ID=manjaro/' /etc/os-release

RUN pacman -Syu --noconfirm --needed \
      chezmoi git bash openssh ca-certificates jq \
    && pacman -Scc --noconfirm

# fake-bw must land on PATH before USER switch so it stays root-owned
COPY fake-bw /usr/local/bin/bw
RUN chmod 0755 /usr/local/bin/bw

RUN useradd -m -s /bin/bash tester
USER tester
WORKDIR /home/tester

COPY --chown=tester:tester fixtures /home/tester/fixtures
COPY --chown=tester:tester entrypoint.sh /home/tester/entrypoint.sh
RUN chmod 0755 /home/tester/entrypoint.sh

ENV FAKE_BW_FIXTURE=/home/tester/fixtures/bw-item.json
ENTRYPOINT ["/home/tester/entrypoint.sh"]
```

- [ ] **Step 2: Build attempt (entrypoint.sh missing — expect a copy failure)**

Run:
```bash
docker build -t chezmoi-test:local -f test/Dockerfile test/
```
Expected: failure at the `COPY ... entrypoint.sh ...` line because that file is created in Task 5. This is the "test-fails-first" gate before Task 5.

- [ ] **Step 3: Commit the Dockerfile**

```bash
git add test/Dockerfile
git commit -m "test(container): add Dockerfile pinned to manjarolinux/base digest"
```

---

## Task 5: entrypoint.sh

**Files:**
- Create: `test/entrypoint.sh`

- [ ] **Step 1: Write the entrypoint**

Create `test/entrypoint.sh` with this exact content:

```bash
#!/usr/bin/env bash
# Phase A smoke runner. Runs inside the chezmoi-test container.
set -euo pipefail

SRC="${CHEZMOI_SRC:-/work}"
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
while IFS= read -r -d '' f; do
  if ! chezmoi --source="$SRC" execute-template < "$f" >/dev/null; then
    echo "[smoke] FAILED rendering $f" >&2
    exit 1
  fi
done < <(find . -name '*.tmpl' -type f -not -path './.git/*' -print0)

echo "[smoke] running chezmoi diff against ephemeral destination"
# diff returns non-zero (typically 1) when there ARE differences — that is
# expected, since the destination is empty. Treat rc>1 as a real crash.
set +e
chezmoi --source="$SRC" --destination="$DEST" diff --no-tty >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" -gt 1 ]; then
  echo "[smoke] chezmoi diff crashed (rc=$rc)" >&2
  exit "$rc"
fi

echo "[smoke] OK"
```

Make it executable:
```bash
chmod +x test/entrypoint.sh
```

- [ ] **Step 2: Build the image (should now succeed)**

Run:
```bash
docker build -t chezmoi-test:local -f test/Dockerfile test/
```
Expected: ends with `Successfully tagged chezmoi-test:local` (or `naming to docker.io/library/chezmoi-test:local` on BuildKit).

- [ ] **Step 3: Run the smoke test against the worktree**

Run from the repo root:
```bash
docker run --rm -v "$PWD:/work:ro" chezmoi-test:local
```
Expected output (order):
```
[smoke] rendering every *.tmpl with chezmoi execute-template
[smoke] running chezmoi diff against ephemeral destination
[smoke] OK
```
Exit 0.

If it fails, drop into a shell to inspect:
```bash
docker run --rm -it -v "$PWD:/work:ro" --entrypoint bash chezmoi-test:local
```
And reproduce manually: `cd /work && chezmoi --source=/work execute-template < <some.tmpl>`.

- [ ] **Step 4: Commit**

```bash
git add test/entrypoint.sh
git commit -m "test(container): add Phase A smoke entrypoint"
```

---

## Task 6: Makefile

**Files:**
- Create: `test/Makefile`

- [ ] **Step 1: Write the Makefile**

Create `test/Makefile` with this exact content (recipes use literal tab indentation):

```make
# Phase A test container shortcuts.
# Override RUNTIME=podman to run rootless, otherwise defaults to docker.

IMG       ?= chezmoi-test:local
RUNTIME   ?= docker
HERE      := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))
REPO_ROOT := $(shell git -C $(HERE) rev-parse --show-toplevel)

.PHONY: build test shell ci-test selftest help

help:
	@echo 'Targets:'
	@echo '  build     Build the test image ($(IMG)) with $(RUNTIME)'
	@echo '  test      Run the smoke entrypoint against $(REPO_ROOT)'
	@echo '  shell     Start an interactive bash inside the test image'
	@echo '  selftest  Run the fake-bw self-test (no container)'
	@echo '  ci-test   Alias for test, used by CI'
	@echo 'Variables:'
	@echo '  RUNTIME   docker (default) or podman'
	@echo '  IMG       image tag (default $(IMG))'

build:
	$(RUNTIME) build -t $(IMG) -f $(HERE)Dockerfile $(HERE)

test: build
	$(RUNTIME) run --rm -v $(REPO_ROOT):/work:ro $(IMG)

shell: build
	$(RUNTIME) run --rm -it -v $(REPO_ROOT):/work:ro --entrypoint bash $(IMG)

selftest:
	$(HERE)tests/test-fake-bw.sh

ci-test: test
```

- [ ] **Step 2: Run `make selftest`**

Run from repo root:
```bash
make -C test selftest
```
Expected: 5+ PASS lines, exit 0.

- [ ] **Step 3: Run `make test`**

Run from repo root:
```bash
make -C test test
```
Expected: image build (cached) and `[smoke] OK`, exit 0.

- [ ] **Step 4: Commit**

```bash
git add test/Makefile
git commit -m "test(container): add Makefile with docker/podman runtime switch"
```

---

## Task 7: `.chezmoiignore` patch

**Files:**
- Modify: `.chezmoiignore`

- [ ] **Step 1: Read the current `.chezmoiignore`**

Run:
```bash
cat .chezmoiignore
```
Note the existing structure (top of file lists docs and code-workspace files).

- [ ] **Step 2: Append test paths**

Append the following block to `.chezmoiignore`:

```
# Containerized test environment — never deploy to ~/
test/
.github/workflows/container-smoke.yml
```

- [ ] **Step 3: Verify the host's chezmoi does not see the new paths as deployable**

Run:
```bash
chezmoi --source="$PWD" managed | grep -E '^(test/|\.github/workflows/container-smoke\.yml)$' && echo "FAIL: still managed" || echo "PASS: not managed"
```
Expected: `PASS: not managed`.

- [ ] **Step 4: Re-run the container smoke test to confirm nothing breaks**

Run:
```bash
make -C test test
```
Expected: `[smoke] OK`.

- [ ] **Step 5: Commit**

```bash
git add .chezmoiignore
git commit -m "chore(chezmoi): exclude test/ container harness from deploy"
```

---

## Task 8: Acceptance verification — negative test + host isolation

**Files:**
- None modified (these are runtime checks, not artifacts).

- [ ] **Step 1: Negative test — deliberate template syntax error**

Inject a broken template at the repo root:
```bash
echo '{{ this is not valid template syntax' > ./broken.tmpl
make -C test test; rc=$?
rm -f ./broken.tmpl
echo "exit code was: $rc"
```
Expected: non-zero `rc` (the smoke test fails because `execute-template` rejects the file). Print line should be `exit code was: 1` (or any non-zero).

- [ ] **Step 2: Confirm green again after removing the broken file**

Run:
```bash
make -C test test
```
Expected: `[smoke] OK`, exit 0.

- [ ] **Step 3: Host isolation — no `$HOME/.config/chezmoi` writes**

Run:
```bash
mkdir -p ~/.config/chezmoi
before=$(find ~/.config/chezmoi -type f 2>/dev/null | sort | sha256sum | awk '{print $1}')
make -C test test
after=$(find ~/.config/chezmoi -type f 2>/dev/null | sort | sha256sum | awk '{print $1}')
echo "before=$before"
echo "after =$after"
[ "$before" = "$after" ] && echo "PASS: no host config changes" || echo "FAIL: host config changed"
```
Expected: `PASS: no host config changes`.

- [ ] **Step 4: Verify the source mount is read-only inside the container**

Run:
```bash
docker run --rm -v "$PWD:/work:ro" --entrypoint bash chezmoi-test:local -c \
  'touch /work/should-fail 2>&1 | head -1'
```
Expected: an error containing `Read-only file system`. (`touch` exits non-zero, but the wrapper run is allowed to surface that without failing the test step.)

- [ ] **Step 5: No commit** — these are runtime checks. If any fail, fix the entrypoint or Dockerfile and re-run from the failing earlier task.

---

## Task 9: GitHub Actions workflow

**Files:**
- Create: `.github/workflows/container-smoke.yml`

- [ ] **Step 1: Ensure the workflow directory exists**

Run:
```bash
mkdir -p .github/workflows
```

- [ ] **Step 2: Write the workflow**

Create `.github/workflows/container-smoke.yml` with this exact content:

```yaml
name: container-smoke

on:
  pull_request:
    paths:
      - '**.tmpl'
      - 'test/**'
      - '.chezmoi*'
      - '.github/workflows/container-smoke.yml'
  push:
    branches: [main]

jobs:
  smoke:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Build image
        run: docker build -t chezmoi-test:ci -f test/Dockerfile test/

      - name: Run smoke test
        run: docker run --rm -v "${{ github.workspace }}:/work:ro" chezmoi-test:ci
```

- [ ] **Step 3: Validate YAML syntax**

Run:
```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/container-smoke.yml'))" \
  && echo "PASS: valid YAML"
```
Expected: `PASS: valid YAML`.

- [ ] **Step 4: Re-run local smoke (the workflow file is in `.chezmoiignore`)**

Run:
```bash
make -C test test
```
Expected: `[smoke] OK`.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/container-smoke.yml
git commit -m "ci(container): add Phase A smoke workflow on PR + main push"
```

---

## Task 10: Final integration sanity check + PR

**Files:** none.

- [ ] **Step 1: Clean rebuild from base**

Run:
```bash
docker rmi chezmoi-test:local 2>/dev/null || true
make -C test build
make -C test test
```
Expected: full rebuild from cached base layer, then `[smoke] OK`.

- [ ] **Step 2: Verify all spec acceptance criteria are met**

Spec §8 checklist:

| Criterion | How to confirm |
|---|---|
| `make test` returns 0 on Manjaro with `RUNTIME=docker` | Step 1 above. |
| Same with `RUNTIME=podman` | If `command -v podman` exists, run `make -C test test RUNTIME=podman`. Otherwise note in PR body that podman is not installed locally; CI runs only docker. |
| Same green in `container-smoke.yml` on `ubuntu-latest` | Confirmed when this PR's CI run goes green. |
| Deliberate `*.tmpl` syntax error → non-zero exit | Task 8 step 1 already proved this locally. |
| `test/` and the workflow file are in `.chezmoiignore` | Task 7 step 3 already proved this. |

- [ ] **Step 3: Push the branch**

Run:
```bash
git push -u origin feat/containerized-test-env
```

- [ ] **Step 4: Open the pull request**

Run:
```bash
gh pr create --title "feat(test): containerized chezmoi smoke harness (Phase A)" --body "$(cat <<'EOF'
## Summary
- Adds `test/` (Dockerfile, fake-bw shim, fixtures, entrypoint, Makefile) for a single-runtime container that smoke-tests every `*.tmpl` and runs `chezmoi diff` against an ephemeral destination.
- Adds `.github/workflows/container-smoke.yml` to run the same image on PR + main push.
- Excludes the harness from chezmoi deploy via `.chezmoiignore`.
- Spec: `docs/superpowers/specs/2026-05-02-containerized-test-env-design.md`.
- Plan: `docs/superpowers/plans/2026-05-02-containerized-test-env.md`.

## Test plan
- [x] `make -C test selftest` — fake-bw shim contract
- [x] `make -C test test` — full container smoke locally with `RUNTIME=docker`
- [ ] CI green on this PR (covers `ubuntu-latest` parity)
- [x] Deliberate broken `*.tmpl` makes the smoke fail
- [x] No host `~/.config/chezmoi` mutation observed during runs
- [x] `/work` mount is read-only (write attempt fails inside container)

## Future work (out of scope)
See spec §7: A+ (`chezmoi init` real-env mimic), fake-bw random+memoize, B (`chezmoi apply --destination`), C (full bootstrap with paru/AUR), and a non-Manjaro Linux matrix.
EOF
)"
```

---

## Self-Review Notes

| Spec section | Plan task |
|---|---|
| §3.1 file layout | Tasks 2–7, 9 |
| §3.2 base image / digest | Task 1, Task 4 step 1 |
| §3.3 host isolation | Task 8 steps 3–4 |
| §3.4 chezmoi without `init` | Task 5 entrypoint |
| §4.1 Dockerfile | Task 4 |
| §4.2 fake-bw | Task 3 |
| §4.3 entrypoint | Task 5 |
| §4.4 Makefile | Task 6 |
| §4.5 GH Actions | Task 9 |
| §4.6 .chezmoiignore | Task 7 |
| §6 failure modes | Task 8 (negative test, RO mount, no host writes) |
| §7 future work | Out of scope by design |
| §8 acceptance criteria | Task 10 step 2 |

No placeholder tasks. All code is inline. Future-work items are deliberately deferred to follow-up plans.
