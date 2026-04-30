#!/usr/bin/env bash
# Cleanup helper for the Claude Code notify dispatcher (F-3.next #5).
#
# claude-notify-dispatch.sh persists "<sid>.id" under
# "${XDG_RUNTIME_DIR:-/tmp}/claude-notify/sessions/" so wired-notify can
# replace popups in place per Claude session. Files for ended sessions
# linger; on a tmpfs runtime dir they vanish at reboot, but long-running
# logins accumulate files indefinitely.
#
# Behaviour:
#   - Removes "*.id" files whose mtime is older than $TTL_DAYS (default 7).
#     mtime is rewritten on every dispatcher invocation, so it tracks
#     "last notification emitted for this session".
#   - Removes leftover ".tmp.*" mktemp artefacts older than 60 minutes.
#     The dispatcher's mv -f is atomic, but a SIGKILL between mktemp and
#     mv leaves stragglers.
#   - Refuses to operate outside the expected base path as a guard
#     against env injection (defence in depth — XDG_RUNTIME_DIR is
#     usually trustworthy, but the suffix check is essentially free).
#
# Wired into systemd --user via claude-notify-cleanup.{service,timer}.

set -euo pipefail

ttl_days="${CLAUDE_NOTIFY_CLEANUP_TTL_DAYS:-7}"
[[ "$ttl_days" =~ ^[1-9][0-9]*$ ]] || ttl_days=7

base_dir="${XDG_RUNTIME_DIR:-/tmp}/claude-notify/sessions"

case "$base_dir" in
  */claude-notify/sessions) ;;
  *)
    command -v logger >/dev/null 2>&1 \
      && logger -t claude-notify-cleanup "refusing unexpected base_dir: $base_dir" \
      || true
    exit 0
    ;;
esac

[[ -d "$base_dir" ]] || exit 0

removed_id="$(find "$base_dir" -maxdepth 1 -type f -name '*.id' \
  -mtime "+$ttl_days" -print -delete 2>/dev/null | wc -l)"
removed_tmp="$(find "$base_dir" -maxdepth 1 -type f -name '.tmp.*' \
  -mmin +60 -print -delete 2>/dev/null | wc -l)"

if (( removed_id > 0 || removed_tmp > 0 )); then
  command -v logger >/dev/null 2>&1 \
    && logger -t claude-notify-cleanup \
       "removed: id=$removed_id tmp=$removed_tmp ttl_days=$ttl_days base=$base_dir" \
    || true
fi

exit 0
