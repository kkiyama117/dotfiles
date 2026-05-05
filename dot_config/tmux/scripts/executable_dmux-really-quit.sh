#!/usr/bin/env bash
# dmux を「本当に」終了する: 現セッションの pane-exited recovery hook を
# 先に外してから、公式の終了キー `q` を dmux ペインに送る。
#
# 背景: dmux 5.7.1 の cleanExit (`q` 押下時) は cleanupPaneSplitHook を
# 呼ばずに process.exit(0) する。結果、登録されたままの pane-exited フック
# (controlPaneRecovery.js) が走って dmux が自動再投入される。
# 上流 issue: standardagents/dmux#80 (関連、対応待ち)。
#
# このスクリプトを `prefix + Q` に bind しておくことで、公式キー `q` の
# 意味論を保ったまま再起動だけを止める。
set -eu

session=$(tmux display-message -p '#S')

# pane-exited hook を解除 (登録されていなければ no-op)
tmux set-hook -u -t "$session" pane-exited 2>/dev/null || true

# pane-title が "dmux" のペインを探す (dmux 自身が select-pane -T dmux を呼ぶ)
dmux_pane=$(
  tmux list-panes -t "$session" -F '#{pane_id} #{pane_title}' \
    | awk '$2 == "dmux" { print $1; exit }'
)

if [ -n "$dmux_pane" ]; then
  tmux send-keys -t "$dmux_pane" q
fi
