: "theme" && {
  : "theme of zsh" && {
    # zsh bar
    if [[ -n $SSH_CONNECTION ]]; then
      autoload -Uz promptinit; promptinit; prompt fire green yellow cyan black yellow white
    else
      autoload -Uz promptinit; promptinit; prompt fire magenta blue cyan yellow green yellow
    fi
  }
  
  : "Determine terminal capabilities." && {
    if ! zmodload zsh/langinfo zsh/terminfo ||
       [[ $langinfo[CODESET] != (utf|UTF)(-|)8 || $TERM == (dumb|linux) ]] ||
       (( terminfo[colors] < 256 )); then
      # Don't use the powerline config. It won't work on this terminal.
      local USE_POWERLINE=false
      # Define alias `x` if our parent process is `login`.
      local parent
      if { parent=$(</proc/$PPID/comm) } && [[ ${parent:t} == login ]]; then
        alias x='startx ~/.xinitrc'
      fi
    fi
  } 2>/dev/null

  : "color settings" && {
    autoload -Uz colors; colors
    #source /usr/share/zsh/p10k.zsh
    
    # To customize prompt, run `p10k configure` or edit ~/.config/zsh/.p10k.zsh.
    [[ ! -f ~/.config/zsh/.p10k.zsh ]] || source ~/.config/zsh/.p10k.zsh
    (( ! ${+functions[p10k]} )) || p10k finalize

    ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE='fg=244'
  }
}
