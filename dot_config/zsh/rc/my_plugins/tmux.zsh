__tmux_inner() {
  SESSION="main"
  command tmux new-session -d -s $SESSION
  command tmux source-file ~/.config/tmux/tmux.conf
  # 上下 分割 & リサイズ
  command tmux splitw -d -p 15 -t $SESSION
  # 下側を左右分割
  command tmux splitw -h -d -p 50 -t $SESSION
  # 上選ぶ
  command tmux select-pane -t $SESSION
  # attach
  command tmux attach-session -t $SESSION 2>/dev/null
}

tmux() {
  if [ "$#" -eq 0 ]; then
    # 引数なし -> main セッションへアタッチ / なければ作成
    # Startup configuration for tmux
    command tmux attach-session -t main 2>/dev/null \
      || __tmux_inner
  else
    # 引数あり→通常の tmux コマンドとして実行
    command tmux "$@"
  fi
}

