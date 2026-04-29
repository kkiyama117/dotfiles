# M-7: 旧実装は `cd` を再定義していたが、zoxide.zsh の `cd` 関数を
#      defer 順で上書きしてしまうバグがあった。`chpwd_functions` フックに
#      切り替え、cd / z 後に共通で発火するようにした。
typeset -g __ONEFETCH_LAST_REPO=""

__onefetch_chpwd() {
  (( $+commands[onefetch] )) || return
  git rev-parse --is-inside-work-tree >/dev/null 2>&1 || return
  local _root
  _root=$(git rev-parse --show-toplevel 2>/dev/null) || return
  local _repo=${_root:t}
  if [[ $__ONEFETCH_LAST_REPO != $_repo ]]; then
    onefetch
    __ONEFETCH_LAST_REPO=$_repo
  fi
}

if [[ ${chpwd_functions[(Ie)__onefetch_chpwd]:-0} -eq 0 ]]; then
  chpwd_functions+=(__onefetch_chpwd)
fi
