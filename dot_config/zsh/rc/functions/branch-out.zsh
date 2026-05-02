# branch-out — `/branch-out` slash command の shell wrapper。
#
# claude -p で non-interactive にスラッシュコマンドを 1 回実行し、その内側で
# `~/.local/bin/claude-tmux-new` が新しい worktree + tmux window +
# 子 claude セッションを spawn する。tmux 内から呼ぶことを想定。
#
# 使い方:
#   branch-out "Plan a worktree spawner feature"
#   bo "fix the dispatcher race condition"
#
# 注意:
# - ブランチ名生成のため LLM 呼び出しが 1 回発生する (レイテンシ + API コスト)。
#   命名を自分で決めて LLM を省きたい場合は claude-tmux-new を直接叩く。
# - 初回は Bash 権限の確認 prompt が出ることがある (~/.claude/settings.json の
#   allowedTools に登録済みなら出ない)。
# - spawn 後フォーカスは新 window へ切り替わる。元の pane に戻りたい場合は
#   `prefix + p` などで戻る。

branch-out() {
    if (( ! $+commands[claude] )); then
        echo "branch-out: claude CLI not found" >&2
        return 1
    fi
    if [[ $# -eq 0 ]]; then
        echo "usage: branch-out <message>" >&2
        return 2
    fi
    claude -p "/branch-out $*"
}

alias bo='branch-out'
