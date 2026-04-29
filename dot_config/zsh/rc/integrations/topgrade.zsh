# F-1: chezmoi は bw 解錠が必要なため topgrade の step からは外し
# (topgrade.toml 側でも disable 済み)、topgrade 完走後に
# `chezmoi_apply` (rc/functions/bw_session.zsh) を連結する。
# topgrade が成功 (rc=0) のときだけ chezmoi_apply を実行する。
# unattended 実行時は事前に `bw_session` で BW_SESSION を export しておく。
upd() {
    if (( ! $+commands[topgrade] )); then
        echo "upd: topgrade is not installed" >&2
        return 1
    fi
    topgrade -y --disable flutter pnpm shell containers chezmoi "$@" \
        && chezmoi_apply
}

