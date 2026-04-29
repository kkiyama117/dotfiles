# bw_session / bw_lock — Bitwarden CLI のロック解除と解除のヘルパ。
#
# 旧版は ~/.config/zsh/bin/bw_session として PATH 経由で配布していたが、
# PATH 経由で外部実行されると子プロセス内で export しても親シェルに届かない
# 構造バグがあった。sheldon 経由で defer source される関数定義に切り替えた。
#
# 自動実行はしない（シェル起動毎の bw unlock プロンプトを避ける）。
# `chezmoi apply` などで Bitwarden 参照が必要なときだけ手動で `bw_session` を
# 呼び、終わったら `bw_lock` で BW_SESSION を unset する運用を推奨。

bw_session() {
    if (( ! $+commands[bw] )); then
        echo "bw_session: bitwarden-cli (bw) is not installed" >&2
        return 1
    fi

    if [[ "$1" == "-f" ]] || [[ -z "$BW_SESSION" ]]; then
        local key
        key="$(bw unlock --raw)" || return $?
        if [[ -z "$key" ]]; then
            echo "bw_session: failed to unlock vault" >&2
            return 1
        fi
        export BW_SESSION="$key"
    else
        echo "bw_session: already unlocked (use -f to re-unlock)"
    fi
}

bw_lock() {
    if [[ -n "$BW_SESSION" ]]; then
        unset BW_SESSION
        echo "BW_SESSION cleared"
    else
        echo "bw_lock: BW_SESSION is already empty"
    fi
}

# chezmoi_apply — bw unlock を必要とする chezmoi apply のラッパ。
#
# - 既に BW_SESSION が解錠済みなら何もしない（再入力を要求しない）
# - 解錠していなければ bw_session で解錠してから chezmoi apply
# - この関数経由で解錠したセッションは終了時に bw_lock で必ず破棄
# - chezmoi apply の終了コードをそのまま返す
#
# 引数はそのまま `chezmoi apply` に渡す（例: `chezmoi_apply --dry-run`）。
chezmoi_apply() {
    if (( ! $+commands[chezmoi] )); then
        echo "chezmoi_apply: chezmoi is not installed" >&2
        return 1
    fi

    local was_unlocked=0
    if [[ -n "$BW_SESSION" ]]; then
        was_unlocked=1
    else
        bw_session || return $?
    fi

    chezmoi apply "$@"
    local rc=$?

    if (( ! was_unlocked )); then
        bw_lock
    fi

    return $rc
}
