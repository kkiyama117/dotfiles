pandapdf () {
  if [[ $#BUFFER -eq 0 ]]; then
    if [[ $# == 0 ]]; then
      BUFFER='convert -auto-orient -page '
      CURSOR=$#BUFFER # カーソル位置を行末に設定
      # BUFFER=$BUFFER\' # カーソルを括弧内に移動する場合
      BUFFER=$BUFFER' *.jpg 6530365061_木山.pdf'
      zle redisplay
    else
      convert -auto-orient -page $1 *.jpg 6530365061_木山.pdf
    fi
  else
      PAGE=$BUFFER
      BUFFER='convert -auto-orient -page '
      BUFFER=$BUFFER$PAGE' *.jpg 6530365061_木山.pdf'
      zle accept-line
  fi
}
zle -N pandapdf
bindkey '^Pd' pandapdf
