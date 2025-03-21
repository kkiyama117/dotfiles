# config.nu
#
# Installed by:
# version = "0.101.0"
#
# This file is used to override default Nushell settings, define
# (or import) custom commands, or run any other startup tasks.
# See https://www.nushell.sh/book/configuration.html
#
# This file is loaded after env.nu and before login.nu
#
# You can open this file in your default editor using:
# config nu
#
# See `help config nu` for more options

# EDITOR
$env.EDITOR = "nvim"
$env.config.buffer_editor = "nvim"

# FIX MISE BUG?
# https://github.com/jdx/mise/issues/2768
$env.config.hooks.env_change = {}

# You can remove these comments if you want or leave
# them for future reference.source ~/.cache/starship/init.nu
#$env.config.shell_integration.reset_application_mode = false
# Fix https://github.com/nushell/nushell/issues/5585
$env.config.shell_integration.osc133 = false

# MISE
# generate `mise.nu`
let mise_path = $nu.default-config-dir | path join mise.nu
^mise activate nu | save $mise_path --force
# まだ使えない
#use ($nu.default-config-dir | path join mise.nu)
#$env.NU_LIB_DIRS ++= ($mise_path | path dirname | to nuon)

# STARSHIP
use ~/.cache/starship/init.nu
