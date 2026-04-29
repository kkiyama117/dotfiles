#!/usr/bin/env bash
# Claude Code sound player. Plays the sound file at $1 using the first
# available backend: pipewire -> pulseaudio -> ffmpeg.
set -euo pipefail

sound="${1:-}"
[[ -z "$sound" || ! -r "$sound" ]] && exit 0

if command -v pw-play >/dev/null 2>&1; then
  exec pw-play --volume=0.6 "$sound"
elif command -v paplay >/dev/null 2>&1; then
  exec paplay --volume=39322 "$sound"
elif command -v ffplay >/dev/null 2>&1; then
  exec ffplay -nodisp -autoexit -loglevel quiet -volume 60 "$sound"
fi

exit 0
