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
- kitty
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
- claude (Claude Code Root config; `~/.claude` is a symlink to `~/.config/claude`)

## Initialize Xrdp

You need to edit `/etc/xrdp/startwm.sh` to load $XDG_CONFIG_HOME/zsh/zprofile

## update

When you update some of settings, bitwarden-cli runs and require master password to unlock the vault.
The zsh wrappers in `dot_config/zsh/rc/functions/bw_session.zsh` automatically restore
`BW_SESSION` from a tmpfs cache (`${XDG_RUNTIME_DIR}/bw_session_${UID}`, mode 0600, cleared on
reboot) so that you only enter the master password once per boot session, not once per shell:

```
chezmoi diff            # first run prompts for master password → cached
chezmoi apply           # subsequent runs (any shell) reuse the cache silently
bw get item <id>        # data-only `bw` subcommands also restore from cache
```

`bw_session` / `bw_lock` helpers remain available for explicit control:

```
bw_session              # cache-first; prompt only on miss/invalid
bw_session -f           # force re-unlock (replaces cache)
bw_lock                 # unset BW_SESSION + `bw lock` + clear cache
```

`BW_SESSION` is a vault-wide access key inherited by child processes. The tmpfs cache disappears
on reboot, but on long-running machines run `bw_lock` when you finish to invalidate the session
server-side as well.

Shell startup does NOT auto-restore from the cache — the cache is only consulted when you
actually invoke `chezmoi apply/diff/update/verify` or a `bw` data subcommand, so opening a new
terminal triggers no `bw` call.

### `upd` (topgrade + chezmoi apply)

`upd` (`dot_config/zsh/rc/integrations/topgrade.zsh`) は topgrade を完走させた後に
`chezmoi_apply` を連結する zsh 関数。`chezmoi` step は topgrade 側では disable して
あり、bw 解錠を要する適用はこの関数で別途行う:

```
upd                     # topgrade -y ... && chezmoi apply (cache 経由で BW_SESSION 復元)
bw_lock                 # 作業終了時の明示破棄 (任意)
```

`upd` は topgrade が rc=0 のときだけ chezmoi apply を実行する。失敗時に chezmoi
だけ走らせたいなら直接 `chezmoi apply` を呼べばよい (`chezmoi_apply` 互換シムも残置)。
2回目以降は cache が効くため事前 `bw_session` は不要だが、cache miss を必ず避けたい
unattended 実行時は `bw_session` を前置する。
