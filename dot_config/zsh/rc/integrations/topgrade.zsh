# chezmoi は bw 解錠が必要なため topgrade からは外す (topgrade.toml 側でも disable 済み)。
# 手動で `chezmoi_apply` を呼ぶ運用。
alias upd="topgrade -y --disable flutter pnpm shell containers chezmoi"

