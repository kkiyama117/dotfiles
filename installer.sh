
# rust
curl https://sh.rustup.rs -sSf | sh


# chezmoi
curl -fsLS https://chezmoi.io/get

chezmoi init --apply https://github.com/username/dotfiles.git

# vim
git clone git@github.com:kkiyama117/neovim-conf.git $XDG_CONFIG_HOME/nvim

# zplug
curl -sL --proto-redir -all,https https://raw.githubusercontent.com/zplug/installer/master/installer.zsh | zsh

# Install plugins if there are plugins that have not been installed
if ! zplug check --verbose; then
    printf "Install? [y/N]: "
    if read -q; then
        echo; zplug install
    fi
fi
source .zshenv
