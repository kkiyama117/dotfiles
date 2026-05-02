# Containerized Test Environment for Chezmoi Dotfiles

- **Status**: Draft (awaiting user review)
- **Date**: 2026-05-02
- **Branch**: `feat/containerized-test-env`
- **Scope**: Smoke-test the dotfiles repository inside a disposable container so contributors and CI never need to run `chezmoi apply`/`diff` against the real `$HOME`.

---

## 1. Goals

1. **Host isolation** — running the test environment must not touch the contributor's real `$HOME`, real Bitwarden vault, or real package manager.
2. **Same artifact for local and CI** — one OCI image, runnable by both Docker and Podman (rootless), executable in GitHub Actions and on a Manjaro workstation.
3. **Phase A first**: validate that every `*.tmpl` in the repo renders without error and that `chezmoi diff` runs to completion on a Manjaro-flavored OS image.
4. **Future-ready**: leave clear extension seams for `chezmoi init` real-env mimic (A+), `chezmoi apply --destination=/tmp/dest` (B), and full bootstrap with paru/AUR (C).

### Non-goals

- Reproducing exact AUR build outputs.
- Real Bitwarden authentication. Tests use a fake `bw` shim.
- Running on Windows containers. Windows branches (`AppData/`) are not exercised here.

---

## 2. Solution Survey

| Option | Verdict | Why |
|---|---|---|
| **Docker (OCI image + Docker CLI)** | ✅ Primary | GitHub Actions native, mature ecosystem, easy `docker compose` upgrade path. |
| **Podman (OCI image + Podman CLI)** | ✅ Local alternative | Rootless by default, daemonless, drop-in CLI compat. Same Dockerfile. |
| **Apptainer / Singularity (SIF)** | ❌ Rejected | Not preinstalled in GH Actions; auto `$HOME` bind mount risks host bleed-through; sudo/paru workflows for future C are awkward; no clear win for dotfiles testing. |
| **chezmoi-only without container** | ❌ Rejected | Cannot guarantee host isolation; defeats the original ask. |

**Decision**: ship a single `Dockerfile`. Local users pick `docker` or `podman`; CI uses `docker` (preinstalled). Apptainer is not maintained.

---

## 3. Architecture

### 3.1 Files added

```
test/
├── Dockerfile             # manjarolinux/base + chezmoi + fake bw
├── fake-bw                # POSIX sh, mimics `bw` CLI surface used by chezmoi
├── entrypoint.sh          # generates ~/.config/chezmoi/chezmoi.toml then runs smoke
├── fixtures/
│   └── bw-item.json       # canned JSON returned by fake-bw
└── Makefile               # `make build|test|shell`, RUNTIME=docker|podman
.github/workflows/
└── container-smoke.yml    # PR + push smoke job
.chezmoiignore             # add `test/` so it never deploys to ~/
```

### 3.2 Base image choice

`manjarolinux/base` — pinned by digest. Templates branch on
`.chezmoi.osRelease.id == "manjaro"`, so an `archlinux` image would silently
skip Manjaro-specific paths and give a false green.

**Digest resolution (implementation step)**: at implementation time, run
`docker pull manjarolinux/base:latest && docker inspect --format='{{index .RepoDigests 0}}' manjarolinux/base:latest`
and substitute the resulting `sha256:...` into the Dockerfile.

**Fallback**: if `manjarolinux/base` is unavailable on Docker Hub at
implementation time, fall back to `archlinux:base` and override
`/etc/os-release` to set `ID=manjaro` so chezmoi's `.chezmoi.osRelease.id`
reflects Manjaro. Document the choice in the Dockerfile header comment.

### 3.3 Host isolation guarantees

- Source repo mounted at `/work:ro` (read-only). Cannot be written back.
- No env vars from host (`BW_SESSION` etc.) are passed in.
- No host `~/.config/chezmoi` mount.
- chezmoi config inside the container is generated fresh every run.

### 3.4 Running chezmoi without `chezmoi init`

`chezmoi init` clones a remote repo, which we don't want in Phase A.
Instead the entrypoint:

1. `mkdir -p ~/.config/chezmoi`
2. Writes `~/.config/chezmoi/chezmoi.toml` with `[data] bitwardenOpenRouterItemId = "test-bw-item"` (so `promptStringOnce` is satisfied).
3. Runs `chezmoi --source=/work execute-template < $tmpl` for each `*.tmpl`.
4. Runs `chezmoi --source=/work --destination=/tmp/dest diff --no-tty || true`. The diff command's exit code reflects diff presence, not crash; we ignore it and rely on `set -e` for hard errors elsewhere.

Phase A+ (see §7) re-introduces `chezmoi init` for real-env parity.

---

## 4. Components

### 4.1 `test/Dockerfile`

```dockerfile
# Digest resolved at implementation time — see §3.2 for procedure and fallback.
FROM manjarolinux/base@sha256:<digest>

RUN pacman -Syu --noconfirm --needed \
      chezmoi git bash openssh ca-certificates \
    && pacman -Scc --noconfirm

# fake bw must be on PATH before user switch
COPY fake-bw /usr/local/bin/bw
RUN chmod 0755 /usr/local/bin/bw

RUN useradd -m -s /bin/bash tester
USER tester
WORKDIR /home/tester

COPY --chown=tester:tester entrypoint.sh /home/tester/entrypoint.sh
COPY --chown=tester:tester fixtures /home/tester/fixtures

ENTRYPOINT ["/home/tester/entrypoint.sh"]
```

### 4.2 `test/fake-bw`

POSIX sh, ~30 lines. Contract:

| Invocation | Response |
|---|---|
| `bw status` | `{"status":"unlocked","userEmail":"test@example.com"}` |
| `bw unlock --raw` (or with `--passwordenv`) | string `FAKE_SESSION_TOKEN` |
| `bw get item <any-id>` | full JSON from `fixtures/bw-item.json` |
| `bw list items --search <any>` | `[ <fixture> ]` |
| anything else | `{}` on stdout, message on stderr, exit 0 |

The fixture populates `fields[]` with names commonly used in this repo
(`name`, `email`, `signingkey`, `CODEX`, etc.) so `bitwardenFields` lookups
resolve.

### 4.3 `test/entrypoint.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail
SRC=${CHEZMOI_SRC:-/work}
DEST=$(mktemp -d)

mkdir -p "$HOME/.config/chezmoi"
cat > "$HOME/.config/chezmoi/chezmoi.toml" <<'EOF'
[data]
bitwardenOpenRouterItemId = "test-bw-item"
EOF

cd "$SRC"

# Phase A: render every template
find . -name '*.tmpl' -type f -not -path './.git/*' -print0 \
  | while IFS= read -r -d '' f; do
      chezmoi --source="$SRC" execute-template < "$f" >/dev/null
    done

# Phase A: diff against an empty destination — proves all templates expand
chezmoi --source="$SRC" --destination="$DEST" diff --no-tty || true

echo "[smoke] OK"
```

### 4.4 `test/Makefile`

```make
IMG ?= chezmoi-test:local
RUNTIME ?= docker

.PHONY: build test shell ci-test

build:
	$(RUNTIME) build -t $(IMG) test/

test: build
	$(RUNTIME) run --rm -v $(CURDIR):/work:ro $(IMG)

shell: build
	$(RUNTIME) run --rm -it -v $(CURDIR):/work:ro --entrypoint bash $(IMG)

ci-test: test
```

### 4.5 `.github/workflows/container-smoke.yml`

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
      - uses: actions/checkout@v4
      - run: docker build -t chezmoi-test test/
      - run: docker run --rm -v ${{ github.workspace }}:/work:ro chezmoi-test
```

### 4.6 `.chezmoiignore` patch

Append:

```
test/
.github/workflows/container-smoke.yml
```

These never deploy to `~/`.

---

## 5. Data Flow

```
contributor                          GitHub Actions
   │                                    │
   ▼                                    ▼
make test  ─────────────────────►  docker build test/
   │                                    │
   ▼                                    ▼
docker run -v $PWD:/work:ro IMG ◄─── docker run -v $GITHUB_WORKSPACE:/work:ro IMG
   │
   ▼
entrypoint.sh
   │
   ├── write ~/.config/chezmoi/chezmoi.toml (canned data)
   │
   ├── execute-template each *.tmpl   ── fake-bw answers Bitwarden lookups
   │
   └── chezmoi diff --destination=/tmp/dest
                │
                ▼
          [smoke] OK   (or non-zero on template error)
```

---

## 6. Failure Modes

| Failure | Detection | Mitigation |
|---|---|---|
| Template syntax / missing field | `execute-template` non-zero | CI fails, PR blocked. |
| `bw` subcommand not handled | shim logs to stderr, exits 0 | Extend shim incrementally. |
| `.chezmoiscripts/run_once_*` accidentally executed | None in Phase A — `diff` does not run scripts; we never call `apply`. | If accidentally added later, `:ro` mount on `/work` and ephemeral container limit blast radius. |
| Host `~/` written | Impossible — no host `$HOME` is mounted. | — |
| `manjarolinux/base` upstream drift | Pinned digest. | Renovate / manual bump. |

---

## 7. Future Work (Backlog)

Captured during brainstorming, intentionally **out of scope** for this design but seam-ready:

1. **A+: `chezmoi init` real-env mimic.** Add a second entrypoint mode (`MODE=init`) that runs `chezmoi init --apply=false file:///work` (or a local fork) and asserts the clone-and-render flow succeeds, including the `read-source-state.pre` hook path with the fake `bw` already on `$PATH`. *(Per user request 2026-05-02.)*
2. **fake-bw random + memoize.** Replace the static fixture with a generator that produces a fresh random value on the first lookup of a given `(item-id, field)` pair and memoizes it for the lifetime of the container, so templates that pin format expectations (hex length, prefix) get exercised. *(Per user request 2026-05-02.)*
3. **B: `chezmoi apply --destination=/tmp/dest`.** Add `MODE=apply`, with `run_once_*` stubbed via a script-shim layer.
4. **C: Full bootstrap image (`Dockerfile.full`).** sudo + wheel-NOPASSWD `tester`, runs the real `run_once_all_os.sh.cmd.tmpl` end-to-end with paru/AUR. Separate, slower CI job (nightly or weekly schedule).
5. **WSL / non-Manjaro Linux matrix.** Second base image (`debian:stable`) to exercise non-Manjaro branches in `.chezmoiignore` and templates.

---

## 8. Acceptance Criteria (for Phase A implementation)

- [ ] `make test` returns 0 from a clean checkout on Manjaro with both `RUNTIME=docker` and `RUNTIME=podman`.
- [ ] Same command runs green in `container-smoke.yml` on `ubuntu-latest`.
- [ ] No file under `$HOME` is created or modified on the host during a run.
- [ ] Introducing a deliberate `*.tmpl` syntax error makes `make test` fail with a non-zero exit code.
- [ ] `test/` and `.github/workflows/container-smoke.yml` are listed in `.chezmoiignore` and `chezmoi diff` on the host shows no attempt to deploy them.

---

## 9. Open Questions

None blocking Phase A. Items 1–5 in §7 are deferred follow-ups.
