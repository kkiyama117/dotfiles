: "history" && {
  : "zsh_history" && {
    HISTSIZE=100
    SAVEHIST=10000
  }
  setopt hist_ignore_dups
  setopt hist_ignore_all_dups
  setopt share_history
  setopt hist_no_store
  setopt hist_reduce_blanks
  setopt hist_verify
}
