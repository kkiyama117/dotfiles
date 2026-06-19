__tmux_inner() {
  __TMUX_SESSION_TARGET="main"
  __TMUX_REPO="${HOME}/.local/share/chezmoi"
  # initialize main session with $HOME as root
  command tmux new-session -d -s $__TMUX_SESSION_TARGET -n $__TMUX_SESSION_TARGET -c "$HOME"
  command tmux select-pane -t=$__TMUX_SESSION_TARGET:$__TMUX_SESSION_TARGET.0 -T $__TMUX_SESSION_TARGET
  command tmux source-file ~/.config/tmux/tmux.conf
  # 上下分割 & リサイズ (top: Hermes 75%, bottom: 25%)
  command tmux splitw -d -p 25 -t $__TMUX_SESSION_TARGET:$__TMUX_SESSION_TARGET.0
  # 下を左右に分割
  command tmux splitw -h -d -p 50 -t $__TMUX_SESSION_TARGET:$__TMUX_SESSION_TARGET.1

  # === Advisor window: Claude (left) + Kimi (right) ===
  command tmux new-window -d -t $__TMUX_SESSION_TARGET: -n advisor -c "$__TMUX_REPO"
  # Left pane: Claude Code
  command tmux select-pane -t $__TMUX_SESSION_TARGET:advisor.0 -T claude
  # Right pane: Kimi CLI (cwd set explicitly for robustness)
  command tmux splitw -h -d -p 50 -t $__TMUX_SESSION_TARGET:advisor.0 -c "$__TMUX_REPO"
  command tmux select-pane -t $__TMUX_SESSION_TARGET:advisor.1 -T kimi

  # === Start agents (sleep to let panes spawn shells) ===
  sleep 0.5
  # Hermes in main top pane
  command tmux send-keys -t $__TMUX_SESSION_TARGET:$__TMUX_SESSION_TARGET.0 "hermes --tui --continue" C-m
  # Claude Code in advisor left pane (resume existing session)
  command tmux send-keys -t $__TMUX_SESSION_TARGET:advisor.0 "claude --continue" C-m
  # Kimi CLI in advisor right pane
  command tmux send-keys -t $__TMUX_SESSION_TARGET:advisor.1 "kimi" C-m

  # attach or switch
  if [[ -v TMUX ]]; then
    command tmux switch-client -t $__TMUX_SESSION_TARGET 2>/dev/null
  else
    command tmux attach-session -t $__TMUX_SESSION_TARGET 2>/dev/null
  fi
}

tmux() {
  if [ "$#" -eq 0 ]; then
    # 引数なし -> main セッションへアタッチ / なければ作成
    # switch if inside of tmux
    if [[ -v TMUX ]]; then
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

__tmux_claude() {
  __TMUX_SESSION_TARGET=claude
  echo "Create tmux claude session"
  command tmux new-session -d -s $__TMUX_SESSION_TARGET -n $__TMUX_SESSION_TARGET
  # 左右分割 & リサイズ
  command tmux splitw -h -d -p 50 -t $__TMUX_SESSION_TARGET:$__TMUX_SESSION_TARGET.0
  # 右側を選んで, claude 起動
  command tmux select-pane -t $__TMUX_SESSION_TARGET:$__TMUX_SESSION_TARGET.1 -T claude
  sleep 1;
  command tmux send-keys -t $__TMUX_SESSION_TARGET:$__TMUX_SESSION_TARGET.1 "pwd; claude --continue" C-m
  # 左選ぶ
  command tmux select-pane -t $__TMUX_SESSION_TARGET:$__TMUX_SESSION_TARGET.0
  # attach or switch
  if [[ -v TMUX ]]; then
    command tmux switch-client -t $__TMUX_SESSION_TARGET 2>/dev/null
  else
    command tmux attach-session -t $__TMUX_SESSION_TARGET 2>/dev/null
  fi
}

tmux_claude(){
    # 引数なし -> claude セッションへアタッチ / なければ作成
    # switch if inside of tmux
    if [[ -v TMUX ]]; then
      # Startup configuration for tmux
      command tmux switch-client -t claude 2>/dev/null \
        || __tmux_claude
    else
      echo "Tmux Claude session"
      # Startup configuration for tmux
      command tmux attach-session -d -t claude 2>/dev/null \
        || __tmux_claude
    fi
}

tmux_claude_new() {
  # Thin wrapper around the portable shell script so tmux popups can call the
  # same logic. Keeps tmux / tmux_claude untouched.
  ~/.local/bin/claude-tmux-new "$@"
}

