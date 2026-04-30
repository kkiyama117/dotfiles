# S-1 phase B: navi 起動エイリアスと zle widget をロード。
# - `navit` : navi で tldr セット (公式チートシート) を引く
# - `nv`    : 短縮起動
# - widget  : `eval "$(navi widget zsh)"` でデフォルト `Ctrl+G` が widget にバインドされる。
#             SKK Hiragana/変換モード時は SKK が `Ctrl+G` を abort として飲むため zsh まで届かない。
#             Latin モード (起動時の既定) のときのみ widget が発動する想定。
#             衝突詳細は `docs/keybinds.md` §3 と `docs/shell_discovery.md` §4 を参照。
if (( $+commands[navi] )); then
  # cheat 探索パスは `dot_config/navi/config.yaml` の `cheats.path` で
  # `~/.config/navi/cheats` を指定する想定 (todos.md S-2 で対応予定)。
  # 環境変数 `NAVI_PATH` は手元の navi バイナリでは効かなかったため設定しない。
  alias navit='navi --tldr'
  alias nv='navi'
  # widget は zsh の zle が初期化された後に bindkey する必要がある。
  # sheldon の `software_integrations` は `apply = ['defer']` で zsh-defer 経由のため、
  # ここでの eval は defer 後に実行され、bindkey も遅延登録される。
  eval "$(navi widget zsh)"
fi
