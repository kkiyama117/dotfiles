################################################################################
# GENERAL program config and path
################################################################################
# Docker
export DOCKER_CONFIG="$XDG_CONFIG_HOME"/docker
# Wine
export WINEPREFIX="$XDG_DATA_HOME/wineprefixes/default"
# GAMESS
export GMS_PATH=/opt/gamess
path=(/opt/cuda/bin(N-/) $path)
export BOOST_ROOT=/usr/local

# ANDROID
export ANDROID_SDK_ROOT="/opt/android"
export ANDROID_SDK_ROOT="/opt/android-sdk"
export ANDROID_JAVA_HOME="opt/android-studio/jre"
export ANDROID_SDK_HOME="$XDG_CONFIG_HOME"/android
export ANDROID_AVD_HOME="$XDG_DATA_HOME"/android
export ANDROID_EMULATOR_HOME="$XDG_DATA_HOME"/android
export ADB_VENDOR_KEY="$XDG_CONFIG_HOME"/android
path=($ANDROID_SDK_ROOT/tools(N-/) $ANDROID_SDK_ROOT/tools/bin(N-/) $ANDROID_SDK_ROOT/platform-tools(N-/) ${ANDROID_SDK_ROOT}/emulator(N-/) $path)

# Flutter
export FLUTTER_ROOT="/opt/flutter"
export FLUTTER_PUB_CACHE="$HOME"/.pub-cache
path=($FLUTTER_ROOT/bin(N-/) $FLUTTER_PUB_CACHE/bin(N-/) $path)
export CHROME_EXECUTABLE=google-chrome-stable

# GAMESS and Quantum Expresso
export GMS_PATH=/opt/gamess
path=(/opt/cuda/bin(N-/) $path)
export BOOST_ROOT=/usr/local
path=($home/programs/q-e/bin(N-/) $path)

# CPP etc
export LD_LIBRARY_PATH=/usr/local/lib/pkgconfig
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/lib:/usr/lib
#unset DYLD_LIBRARY_PATH
 
export LDFLAGS=-L/usr/local/lib
export LDFLAGS=-L/usr/local/stow/gettext-021/lib:$LDFLAGS
export LDFLAGS=-L/usr/local/stow/gettext-021/lib/gettext:$LDFLAGS

export CPPFLAGS=-I/usr/local/include
export CPPFLAGS=-I/usr/local/stow/gettext-021/include:$CPPFLAGS

# Ipython
export IPYTHONDIR="$XDG_CONFIG_HOME"/jupyter
export JUPYTER_CONFIG_DIR="$XDG_CONFIG_HOME"/jupyter
