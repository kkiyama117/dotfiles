# Development tool environment.
#
# 旧 `dot_config/zsh/dot_zshenv/for_development.zsh` (M-9) — chezmoi が
# `dot_zshenv` をディレクトリ扱いし、target が `~/.config/zsh/.zshenv/...`
# となって zsh が読まない状態だったため、`rc/` 配下に移動して sheldon
# `my_conf_pre_load` (sync source) で読ませるよう変更。
#
# H-6: `$home` (lowercase) → `$HOME` 修正
# H-7: 二重 export / 連続上書き / 相対パス / pkgconfig が LD_LIBRARY_PATH
#      に紛れ込んでいた問題を整理

# Docker
export DOCKER_CONFIG="$XDG_CONFIG_HOME/docker"

# Wine
export WINEPREFIX="$XDG_DATA_HOME/wineprefixes/default"

# CUDA / GAMESS / Quantum Espresso
path=(
    /opt/cuda/bin(N-/)
    $HOME/programs/q-e/bin(N-/)
    $path
)
export GMS_PATH=/opt/gamess
export BOOST_ROOT=/usr/local

# Android SDK
export ANDROID_SDK_ROOT="/opt/android-sdk"
export ANDROID_JAVA_HOME="/opt/android-studio/jre"
export ANDROID_SDK_HOME="$XDG_CONFIG_HOME/android"
export ANDROID_AVD_HOME="$XDG_DATA_HOME/android"
export ANDROID_EMULATOR_HOME="$XDG_DATA_HOME/android"
export ADB_VENDOR_KEY="$XDG_CONFIG_HOME/android"
path=(
    $ANDROID_SDK_ROOT/tools(N-/)
    $ANDROID_SDK_ROOT/tools/bin(N-/)
    $ANDROID_SDK_ROOT/platform-tools(N-/)
    $ANDROID_SDK_ROOT/emulator(N-/)
    $path
)

# Flutter
export FLUTTER_ROOT="/opt/flutter"
export FLUTTER_PUB_CACHE="$HOME/.pub-cache"
path=(
    $FLUTTER_ROOT/bin(N-/)
    $FLUTTER_PUB_CACHE/bin(N-/)
    $path
)
export CHROME_EXECUTABLE=google-chrome-stable

# C/C++ build environment (gettext stowed in /usr/local/stow)
export LD_LIBRARY_PATH="/usr/local/lib:/usr/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export PKG_CONFIG_PATH="/usr/local/lib/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
export LDFLAGS="-L/usr/local/stow/gettext-021/lib/gettext -L/usr/local/stow/gettext-021/lib -L/usr/local/lib"
export CPPFLAGS="-I/usr/local/stow/gettext-021/include -I/usr/local/include"

# Jupyter / IPython
export IPYTHONDIR="$XDG_CONFIG_HOME/jupyter"
export JUPYTER_CONFIG_DIR="$XDG_CONFIG_HOME/jupyter"
