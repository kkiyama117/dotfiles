: "theme" && {
  : "theme of zsh" && {
    # zsh bar
    if [[ -n $SSH_CONNECTION ]]; then
      autoload -Uz promptinit; promptinit; prompt fire green yellow cyan black yellow white
    else
      autoload -Uz promptinit; promptinit; prompt fire magenta blue cyan yellow green yellow
    fi
  }
  : "color settings" && {
    autoload -Uz colors; colors
  }
}
