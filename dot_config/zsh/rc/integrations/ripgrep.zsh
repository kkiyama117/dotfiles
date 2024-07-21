  if (( $+commands[rg] )) then
    alias grep='rg --color=always --no-ignore --hidden'
    alias grep_raw='rg --color=always'
  else
    alias grep='grep --colour=auto'
    alias grep_raw='grep --colour=auto'
  fi
