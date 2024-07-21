
unsetopt promptcr            # Prevent overwriting non-newline output at the prompt

setopt auto_cd
setopt auto_list             # Display a list of possible completions with ^I (when there are multiple candidates for completion, display a list)
setopt auto_menu             # 補完キー連打で順に補完候補を自動で補完
setopt auto_param_keys       # カッコの対応などを自動的に補完
setopt auto_param_slash      # ディレクトリ名の補完で末尾の / を自動的に付加し、次の補完に備える
setopt auto_pushd            # Put the directory in the directory stack even when cd'ing normally.
setopt auto_remove_slash     # Automatically remove trailing / in completions
setopt auto_resume           # Resume when executing the same command name as a suspended process
setopt chase_links           # Symbolic links are converted to linked paths before execution
setopt correct               # Auto correct mistakes
setopt complete_in_word
setopt equals                # Expand =COMMAND to COMMAND pathname
setopt extended_glob         # Extended globbing. Allows using regular expressions with *
unsetopt flow_control        # Disable C-s, C-q (in shell editor)
setopt no_flow_control       # Do not use C-s/C-q flow control
setopt glob
setopt globdots		     # 明確なドットの指定なしで.から始まるファイルをマッチ
setopt list_packed           # Compactly display completion list
setopt list_types            # 補完候補一覧でファイルの種別を識別マーク表示 (訳注:ls -F の記号)
setopt ignore_eof            # Don't logout with C-d
setopt interactive_comments  # コマンドラインでも # 以降をコメントと見なす
setopt mark_dirs             # ファイル名の展開でディレクトリにマッチした場合 末尾に / を付加
setopt magic_equal_subst     # コマンドラインの引数で --prefix=/usr などの = 以降でも補完できる

#setopt nobeep               # No beep
setopt nocaseglob            # Case insensitive globbing
setopt nocheckjobs           # Don't warn about running processes when exiting
unsetopt no_clobber
setopt noflowcontrol
setopt no_hup                # Don't kill background jobs on logout
setopt nolistambiguous # Show menu
setopt nonomatch             # Enable glob expansion to avoid nomatch
setopt notify
setopt numericglobsort       # Sort filenames numerically when it makes sense
setopt path_dirs             # Find subdirectories in PATH when / is included in command name
setopt print_eight_bit       # 日本語ファイル名等8ビットを通す
setopt pushd_ignore_dups     # Delete old duplicates in the directory stack.
setopt pushd_to_home         # no pushd argument == pushd $HOME
setopt pushd_silent          # Don't show contents of directory stack on every pushd,popd
setopt short_loops           # Use simplified syntax for FOR, REPEAT, SELECT, IF, FUNCTION, etc.
unsetopt sh_word_split

