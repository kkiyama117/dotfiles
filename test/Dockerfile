# Phase A test image for chezmoi dotfiles smoke testing.
# Base digest is pinned for determinism; refresh via:
#   docker pull manjarolinux/base:latest && \
#   docker inspect --format='{{index .RepoDigests 0}}' manjarolinux/base:latest
# (Or substitute `podman` for `docker`.)
FROM docker.io/manjarolinux/base@sha256:a411decb8d219cb4b16b39618e9aa8ce8228acad9cba5b10420f804a20ca5c84

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
