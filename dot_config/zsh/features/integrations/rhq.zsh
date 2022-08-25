function __fuzzy-select-repositories() {
  local selected=$(rhq list | sk --prompt='REPOS> ' --query="$LBUFFER")
  if [[ -n $selected ]]; then
    BUFFER="cd \"${selected}\""
    zle accept-line
  fi
  zle clear-screen
}

zle -N __fuzzy-select-repositories
bindkey '^r' __fuzzy-select-repositories
