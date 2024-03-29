# option for vanilla zsh
: "options" && {
  setopt auto_cd
  setopt auto_pushd
  setopt auto_list
  setopt auto_menu             # 補完キー連打で順に補完候補を自動で補完
  setopt auto_param_keys       # カッコの対応などを自動的に補完
  setopt auto_param_slash      # ディレクトリ名の補完で末尾の / を自動的に付加し、次の補完に備える
  setopt correct                                                  # Auto correct mistakes
  setopt list_packed
  setopt list_types            # 補完候補一覧でファイルの種別を識別マーク表示 (訳注:ls -F の記号)
  setopt interactive_comments  # コマンドラインでも # 以降をコメントと見なす
  setopt mark_dirs             # ファイル名の展開でディレクトリにマッチした場合 末尾に / を付加
  setopt magic_equal_subst     # コマンドラインの引数で --prefix=/usr などの = 以降でも補完できる
  setopt nobeep                                                   # No beep
  setopt nocheckjobs                                              # Don't warn about running processes when exiting
  setopt notify
  setopt print_eight_bit       # 日本語ファイル名等8ビットを通す
  setopt pushd_ignore_dups
  : "glob" && {
    setopt extendedglob                                             # Extended globbing. Allows using regular expressions with *
    setopt nocaseglob                                               # Case insensitive globbing
    setopt numericglobsort                                          # Sort filenames numerically when it makes sense
    # setopt globdots　　　　　# 明確なドットの指定なしで.から始まるファイルをマッチ
  }
}
