 if (( $+commands[zellij] )) then
  function zellij_cd() {
    local result="$(zellij action new-pane -c --cwd "$@" -- zsh)"
    #if [ -z "${result}" ]; then
    #    result="$(navi --print </dev/tty)"
    #fi
    printf "%s" "$result"
  }
fi

 #zellij action new-pane --cwd ~/programs/ -c -- zsh
