# L-4: 旧実装は出力ファイル名 `6530365061_木山.pdf` がハードコードされており
#      公開リポジトリに学籍番号風 ID と姓を晒していた。
#      使い方:
#        pandapdf <page> [output.pdf]          # 直接実行
#        環境変数 PANDAPDF_OUTPUT で既定出力名を上書き可能（既定 output.pdf）
#        ZLE バインド (^Pd) はテンプレ展開のみ — page と output 名は手動編集
pandapdf () {
  local _output="${PANDAPDF_OUTPUT:-output.pdf}"
  if [[ $#BUFFER -eq 0 ]]; then
    if [[ $# == 0 ]]; then
      BUFFER="convert -auto-orient -page "
      CURSOR=$#BUFFER
      BUFFER="$BUFFER *.jpg $_output"
      zle redisplay
    else
      convert -auto-orient -page "$1" *.jpg "${2:-$_output}"
    fi
  else
    local _page=$BUFFER
    BUFFER="convert -auto-orient -page $_page *.jpg $_output"
    zle accept-line
  fi
}
zle -N pandapdf
bindkey '^Pd' pandapdf
