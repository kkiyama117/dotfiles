if (( $+commands[sk] )) then
  export SKIM_DEFAULT_OPTIONS="--ansi"
  lnlink="$HOME/.local/bin/fzf"
  if [ ! -L ${lnlink} ]; then
    ln -s "$(which sk)" ${lnlink}
  fi
fi

if (( $+commands[sk] )) then
    alias fzf_raw='sk'
    if (( $+commands[bat] )) then
      alias fzf='sk --preview="bat {} --color=always"'
    else
      alias fzf='sk --preview="cat {}"'
    fi
    if (( $+commands[rg] )) then
      alias ski='sk -i -c "rg {} --no-ignore --hidden --color=always --line-number"'
    else
      alias ski='sk -i -c "grep {} -rI --colour=always --line-number"'
    fi
    alias fzfi='ski'
fi
