if (( $+commands[zoxide] )) then
  eval "$(zoxide init zsh)"
  function __zoxide_zi() {
      result="$(zoxide query -ls -- "$@" | sk)" && cd "$result"
  }
fi
