: "history" && {
  : "zsh_history" && {
    HISTSIZE=100
    SAVEHIST=10000
  }
  setopt appendhistory                                            # Immediately append history instead of overwriting
  setopt hist_ignore_dups
  setopt histignorealldups
  setopt share_history
  setopt hist_no_store
  setopt hist_reduce_blanks
  setopt hist_verify
  setopt inc_append_history                                       # save commands are added to the history immediately, otherwise only when shell exits.
  setopt histignorespace                                          # Don't save commands that start with space
}
