: "aliases" && {
  if (( $+commands[lsd] )) then
    alias ls='lsd -Fh'
  else
    alias ls='ls -Fh --color=auto'
  fi
  alias la='ls -a'
  alias ll='ls -l'
  alias lal='ls -al'

  alias rm='rm -i'
  alias cp='cp -i'
  alias mv='mv -i'

  alias mkdir='mkdir -p'

  alias cp="cp -i"                          # confirm before overwriting something
  alias df='df -h'                          # human-readable sizes
  alias free='free -m'                      # show sizes in MB
  alias more=less

  if (( $+commands[delta] )) then
    alias diff='delta'
  else
    alias diff='diff'
  fi

  if (( $+commands[rg] )) then
    alias grep='rg --color=always --no-ignore --hidden'
    alias grep_raw='rg --color=always'
  else
    alias grep='grep --colour=auto'
    alias grep_raw='grep --colour=auto'
  fi

  #alias rsync-git='rsync -a -C --filter=":- .gitignore"'
  if (( $+commands[git] )) then
    alias g="git"
    alias ga="git add"
    alias gb="git branch"
    alias gc="git commit"
    alias gca="git commit -a"
    alias gP="git push"
    alias gp="git pull"
    alias gs="git switch"
    alias gm="git merge"
  fi

  if (( $+commands[bat] )) then
    alias cat='bat'
  fi
  
  if (( $+commands[fd] )) then
    alias find='fd'
  fi
  
  # skim
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

  : "navi" && {
    if (( $+commands[navi] )) then
      alias n='navi'
      if (( $+commands[navi] )) then
        alias nt='navi --tldr'
      fi
    fi
  }

: "rofi" && {
    if (( $+commands[rofi] )) then
      alias dmenu='rofi -show run -modi run -location 1 -width 100 \
		  -lines 2 -line-margin 0 -line-padding 1 \
		  -separator-style none -font "mono 10" -columns 9 -bw 0 \
		  -disable-history \
		  -hide-scrollbar \
		  -color-window "#222222, #222222, #b1b4b3" \
		  -color-normal "#222222, #b1b4b3, #222222, #005577, #b1b4b3" \
		  -color-active "#222222, #b1b4b3, #222222, #007763, #b1b4b3" \
		  -color-urgent "#222222, #b1b4b3, #222222, #77003d, #b1b4b3" \
		  -kb-row-select "Tab" -kb-row-tab ""'
    fi
  }
  
  : "neovim" && {
    if (( $+commands[nvim] )) then
      alias vim=nvim
      alias vi=nvim
      alias edit=nvim
      alias edit=nvim
      alias dedit=dvi
      # debugging nvim
      alias dvi="DEBUG=true nvim"
      alias dedit=dvi
      alias vimo=/usr/bin/vim
    fi
  }

  : "chezmoi" && {
    if (( $+commands[chezmoi] )) then
      alias editconf='chezmoi edit'
      alias conf='chezmoi apply -rv'
    fi
  }
}

