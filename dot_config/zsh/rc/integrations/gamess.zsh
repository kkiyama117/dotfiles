  : "neovim" && {
    if (( $+commands[nvim] )) then
      export NHOME="$HOME/.config/nvim"
      export DENOPS_PATH="$HOME/programs/others/vim-jp/denops.vim"
      export DENOPS_TEST_NVIM="/usr/bin/nvim"
    fi
  }
  export GMS_PATH=/opt/gamess
  export BOOST_ROOT=/usr/local
  if [[ -d $HOME/programs/tools/gamess/gamess ]]; then
    alias rungms=~/programs/tools/gamess/gamess/rungms 
  fi
