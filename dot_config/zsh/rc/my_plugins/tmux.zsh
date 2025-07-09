tmux() {
  if [ "$#" -eq 0 ]; then
    echo "Custom tmux" 
    # 引数なし -> main セッションへアタッチ / なければ作成
    command tmux attach-session -t main 2>/dev/null \
      || command tmux new-session -s main
    command tmux source-file ~/.config/tmux/tmux.conf
    # Startup configuration for tmux
    command tmux splitw -h -p 50 -t 0 'pwd'
    command tmux selectw -t 0
  else
    # 引数あり→通常の tmux コマンドとして実行
    command tmux "$@"
  fi
}

