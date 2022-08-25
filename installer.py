import distro
import os
import platform
import subprocess

from enum import Flag, auto


class Distro(Flag):
    MANJARO = auto()
    OTHERS = auto()


def _calling(cmdlist, cwd=None):
    process = subprocess.run(
        cmdlist, stdout=subprocess.PIPE, text=True, cwd=cwd)
    return process.stdout.strip()


def _install(package, distro=Distro.MANJARO):
    if distro is Distro.MANJARO and not is_using_paru():
        install_paru()
    _calling(["paru", "-S", package])


def _git_clone(path):
    _calling(['git', 'clone', path])


def is_using_zsh():
    return os.environ.get("ZSH_VERSION") is not None


def install_zsh(os):
    if not is_using_zsh():
        return "Already using zsh"
    else:
        return _install('zsh')


def install_zplug():
    process1 = subprocess.run(
            [
              'curl', '-sl', '--proto-redir', '-all,https',
              'https://raw.githubusercontent.com/zplug/installer/master/installer.zsh'
                ], stdout=subprocess.PIPE, text=True)
    process = subprocess.run(
            ['zsh'], stdout=subprocess.PIPE, text=True, stdin=process1.stdout)
    if not _calling('zplug', 'check', '--verbose'):
        _calling(['zplug', 'install'])
        _calling(['source', '~/.zshenv'])
    return process.stdout.strip()


def is_using_paru():
    return _calling(["paru", "-V"]) is not None


def install_paru():
    if is_using_paru():
        return "Already using paru"
    _calling(["sudo", "pacman", "-S", "--needed", "base-devel"])
    _calling(["git", "clone", "https://aur.archlinux.org/paru.git"])
    return _calling(["makepkg", "-si"], cwd="paru")


def get_os():
    if platform.system() != "Linux":
        raise OSError("not linux")
    name = distro.name()
    if name == "Manjaro Linux":
        return Distro.MANJARO
    return Distro.OTHERS


def get_vim_conf():
    _git_clone('git@github.com:kkiyama117/test.git')


if __name__ == "__main__":
    os_name = get_os()

    if os_name is Distro.MANJARO:
        print(install_paru())
    if not is_using_zsh():
        print(install_zsh(os_name))
        install_zplug()

