# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This is a dotfiles repository managed by [Chezmoi](https://www.chezmoi.io/), containing personal configuration files for various tools and applications on a Linux system (Manjaro).

## Common Commands

### Chezmoi Operations
- `chezmoi init --apply kkiyama117` - Initial repository setup
- `chezmoi update` - Pull latest changes and apply them
- `chezmoi apply` - Apply configuration changes to the system
- `chezmoi diff` - Show differences between repository and actual files
- `chezmoi add ~/.config/someapp` - Add a new configuration file to management
- `chezmoi edit ~/.config/someapp` - Edit a managed file
- `chezmoi cd` - Navigate to the Chezmoi source directory

### Secret Management
This repository uses Bitwarden for managing secrets. Templates (`.tmpl` files) reference secrets using:
- `{{ (bitwarden "item" "secret-name").value }}`
- `{{ (bitwardenFields "item" "secret-name").field_name.value }}`

## Architecture & Structure

### Directory Organization
- `dot_config/` - Maps to `~/.config/` (XDG config directory)
- `dot_local/` - Maps to `~/.local/` (XDG local directory)
- `private_dot_*` - Private files (permissions 0600)
- `executable_*` - Executable scripts
- `*.tmpl` - Template files processed by Chezmoi

### Key Configuration Areas
1. **Shell Environment (ZSH)**
   - Main configs: `dot_config/zsh/.zshrc`, `.zshenv`, `.zprofile`
   - Aliases: `dot_config/zsh/rc/aliases.zsh`
   - Plugin management via Sheldon: `dot_config/sheldon/plugins.toml`

2. **Development Tools**
   - Git configuration with Bitwarden integration: `dot_config/git/config.tmpl`
   - Neovim configuration: `dot_config/nvim/`
   - Terminal emulators: wezterm, alacritty configs

3. **System Integration**
   - Systemd user services: `dot_config/systemd/user/`
   - Desktop entries: `dot_local/share/applications/`

### Template System
Templates use Go's text/template syntax with Chezmoi functions:
- OS detection: `{{ .chezmoi.os }}`, `{{ .chezmoi.osRelease.id }}`
- Conditional logic: `{{ if eq .chezmoi.os "linux" }}...{{ end }}`
- Password manager integration for secrets

### Important Patterns
1. **Never commit secrets directly** - Always use Bitwarden integration
2. **Test template changes** with `chezmoi diff` before applying
3. **Use appropriate file prefixes** (private_, executable_) for permissions
4. **Follow XDG Base Directory specification** for file locations

