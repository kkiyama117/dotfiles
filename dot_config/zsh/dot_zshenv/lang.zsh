# mise path
path=($XDG_DATA_HOME/mise/shims(N-/) $path)

# Need to keep variables about Rust
export RUSTUP_HOME="$XDG_DATA_HOME/rustup"
export CARGO_HOME="$XDG_DATA_HOME/cargo"
path=($CARGO_HOME/bin(N-/) $path)

# I am a firm believer in only using python installed with rye.
export RYE_HOME="$XDG_DATA_HOME/rye"
export PYTHONSTARTUP="$XDG_CONFIG_HOME"/python/python_startup.py
source $RYE_HOME/env

# opam
[[ ! -r "$XDG_DATA_HOME/opam-init/init.zsh" ]] || source "$XDG_DATA_HOME/opam-init/init.zsh"  > /dev/null 2> /dev/null

# npm
export NPM_CONFIG_USERCONFIG="$XDG_CONFIG_HOME"/npm/npmrc

