# dotfiles

```
chezmoi init --apply kkiyama117
```

Some files`./.local/share/initializer/` folder run by chezmoi and `.config/chezmoi/chezmoi.toml`

## Contents

### Shell & Prompt
- zsh (with [sheldon](https://github.com/rossmacarthur/sheldon) plugin manager)
- nushell
- [starship](https://starship.rs/) prompt

### Terminal Emulators
- wezterm
- ghostty

### Editor
- Neovim (`nvim`)

### Multiplexer
- tmux

### Version / Runtime Managers
- [mise](https://mise.jdx.dev/) (polyglot runtime manager)
- rye (Python)
- cargo (Rust)
- npm

### CLI Tools (zsh integrations)
- bat, fd, ripgrep, lsd, skim, zoxide
- git-delta (pager)
- navi (cheatsheet)
- onefetch (repo summary)
- rofi (launcher)
- topgrade (system updater)
- pueue (task queue)

### Git & GitHub
- git (with delta + Bitwarden-injected user info)
- gh (GitHub CLI)

### Japanese Input / SKK
- mozc
- yaskkserv2

### Desktop / Window System
- X11
- wired (notification daemon)
- GTK 3
- dircolors

### Sync & System
- syncthing
- systemd (user services)
- environment.d
- pacman

### Outside `dot_config`
- `dot_local/share/{cargo,rye,gnupg,zsh}`

### Meta
- chezmoi (own config + run-once script)
- claude (Claude Code config; `~/.claude` is a symlink to `~/.config/claude`)

## Initialize Xrdp

You need to edit `/etc/xrdp/startwm.sh` to load $XDG_CONFIG_HOME/zsh/zprofile

## update

When you update some of settings, bitwarden-cli runs and require master password to unlock key.
`BW_SESSION` avoid it.

```
export BW_SESSION=$(bw unlock --raw)
```
