#!/bin/sh
# Smart pinentry chooser invoked by gpg-agent.
#
# gpg-agent inherits the calling client's environment when
#   gpg-connect-agent updatestartuptty /bye
# is run from that client (already wired in ~/.config/zsh/dot_zshrc).
#
# Selection order:
#   1. JetBrains IDE override (PINENTRY_USER_DATA=IJ_PINENTRY=...)
#   2. Explicit GUI override (PINENTRY_USER_DATA=FORCE_GUI)
#   3. Headless / remote signals (SSH_*, TMUX, TERM_PROGRAM=tmux)
#      -> pinentry-curses
#   4. --display passed and pinentry-gnome3 available -> pinentry-gnome3
#   5. Fallback -> pinentry-curses

if [ -n "$PINENTRY_USER_DATA" ]; then
  case "$PINENTRY_USER_DATA" in
    IJ_PINENTRY=*)
      exec "/opt/rustrover/jbr/bin/java" \
        -cp "/opt/rustrover/plugins/vcs-git/lib/git4idea-rt.jar:/opt/rustrover/lib/externalProcess-rt.jar" \
        git4idea.gpg.PinentryApp
    ;;
    *FORCE_GUI*)
      [ -x /usr/bin/pinentry-gnome3 ] && exec /usr/bin/pinentry-gnome3 "$@"
    ;;
  esac
fi

if [ -n "$SSH_CONNECTION" ] || [ -n "$SSH_TTY" ] || [ -n "$SSH_CLIENT" ] \
  || [ -n "$TMUX" ] || [ "$TERM_PROGRAM" = "tmux" ]; then
  exec /usr/bin/pinentry-curses "$@"
fi

prev=""
display=""
for a in "$@"; do
  if [ "$prev" = "--display" ]; then
    display="$a"
    break
  fi
  prev="$a"
done

if [ -n "$display" ] && [ -x /usr/bin/pinentry-gnome3 ]; then
  exec /usr/bin/pinentry-gnome3 "$@"
fi

exec /usr/bin/pinentry-curses "$@"
