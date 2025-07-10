__tmux_inner() {
  SESSION="main"
  # initialize
  command tmux new-session -d -s $SESSION -n $SESSION
  command tmux select-pane -t=$SESSION:$SESSION.0 -T $SESSION
  command tmux source-file ~/.config/tmux/tmux.conf
  # 上下分割 & リサイズ
  command tmux splitw -d -p 15 -t $SESSION:$SESSION.0
  # 下を左右に分割
  command tmux splitw -h -d -p 50 -t $SESSION:$SESSION.1
  # attach or switch
  if [[ -v SOME_VARIABLE ]]; then
    command tmux switch-client -t $SESSION 2>/dev/null
  else
    command tmux attach-session -t $SESSION 2>/dev/null
  fi
}

tmux() {
  if [ "$#" -eq 0 ]; then
    # 引数なし -> main セッションへアタッチ / なければ作成
    # switch if inside of tmux
    if [[ -v SOME_VARIABLE ]]; then
      # Startup configuration for tmux
      command tmux switch-client -t main 2>/dev/null \
        || __tmux_inner
    else
      # Startup configuration for tmux
      command tmux attach-session -d -t main 2>/dev/null \
        || __tmux_inner
    fi
  else
    # 引数あり→通常の tmux コマンドとして実行
    command tmux "$@"
  fi
}

tmux_claude(){
    # 引数なし -> claude セッションへアタッチ / なければ作成
    # Startup configuration for tmux
    command tmux attach-session -d -t claude 2>/dev/null \
      || __tmux_claude
}

__tmux_claude() {
  SESSION=claude
  command tmux new-session -d -s $SESSION
  # 左右分割 & リサイズ
  command tmux rename-window -t=$SESSION:1 $SESSION
  command tmux splitw -h -d -p 50 -t=$SESSION:$SESSION.0
  # 右側を選んで, claude 起動
  command tmux select-pane -t $SESSION:$SESSION.1 -T claude
  command tmux send-keys -t $SESSION:claude claude C-m
  # 左選ぶ
  command tmux select-pane -t=$SESSION:$SESSION:0
  # attach
  # attach or switch
  if [[ -v SOME_VARIABLE ]]; then
    command tmux switch-client -t $SESSION 2>/dev/null
  else
    command tmux attach-session -t $SESSION 2>/dev/null
  fi
}

