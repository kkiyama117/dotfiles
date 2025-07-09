tmux() {
  if [ "$#" -eq 0 ]; then
    # 引数なし -> main セッションへアタッチ / なければ作成
    command tmux attach-session -t main 2>/dev/null \
      || command tmux new-session -s main
    command tmux source-file ~/.config/tmux/tmux.conf
  else
    # 引数あり→通常の tmux コマンドとして実行
    command tmux "$@"
    command tmux source-file ~/.config/tmux/tmux.conf
  fi
}

