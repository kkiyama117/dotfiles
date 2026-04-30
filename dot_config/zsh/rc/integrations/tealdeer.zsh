# S-1 phase B: tealdeer (tldr の Rust 実装) 用の起動 alias と fzf 連携。
#
# 役割は `docs/shell_discovery.md` の L2 (公式コマンドの基本用法を参照)。
# 自分専用のスニペットは navi (L3) に寄せる方針。
#
# キーバインド: `Alt+H` は zsh の `run-help` で既に使用中のため割り当てない。
# 起動はコマンド名 (`tldr` / `tld` / `tldrf`) で行う。詳細は
# `docs/keybinds.md` §3 を参照。

if (( $+commands[tldr] )); then
  # 短縮: tldr ではなく tld でも引ける
  alias tld='tldr'

  # tldrf: skim/fzf でコマンドを選択して tldr ページを表示
  # - `sk` (skim) があれば優先、無ければ fzf にフォールバック
  # - リスト取得は `tldr --list` (キャッシュからオフラインで返る)
  tldrf() {
    local picker
    if (( $+commands[sk] )); then
      picker=sk
    elif (( $+commands[fzf] )); then
      picker=fzf
    else
      print -u2 "tldrf: requires sk or fzf"
      return 1
    fi
    local cmd
    cmd="$(tldr --list 2>/dev/null | tr -d ',' | tr ' ' '\n' | grep -v '^$' | "$picker" --prompt='tldr> ')" || return
    [[ -n "$cmd" ]] && tldr "$cmd"
  }
fi
