#!/usr/bin/env sh

# rust
if [ ! command -v rustup &> /dev/null ]; then
    curl https://sh.rustup.rs -sSf | sh
  else
    echo "rust is already installed"
fi


# chezmoi
if [ ! command -v chezmoi &> /dev/null ]; then
    curl -fsLS https://chezmoi.io/get
  else
    echo "chezmoi is already installed"
fi
chezmoi init --apply https://github.com/kkiyama117/dotfiles.git
