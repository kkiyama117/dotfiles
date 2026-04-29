#!/usr/bin/env bash
# stdout: "[claude:N] " when N >= 1, else empty.
n=$(pgrep -c -x claude 2>/dev/null || echo 0)
[ "${n:-0}" -gt 0 ] && printf "[claude:%d] " "$n"
exit 0
