# dotfiles

```
chezmoi init --apply kkiyama117
```

Some files`./.local/share/initializer/` folder run by chezmoi and `.config/chezmoi/chezmoi.toml`

## Initialize Xrdp

You need to edit `/etc/xrdp/startwm.sh` to load $XDG_CONFIG_HOME/zsh/zprofile

## update

When you update some of settings, bitwarden-cli runs and require master password to unlock key.
`BW_SESSION` avoid it.

```
export BW_SESSION=$(bw unlock --raw)
```
