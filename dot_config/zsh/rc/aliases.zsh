#############################################################################
# Common
#############################################################################

alias cp='cp -irf' # confirm before overwriting something
alias df='df -h'                          # human-readable sizes
alias dircolor='eval `dircolors -b $XDG_CONFIG_HOME/dircolors`'
alias free='free -m'                      # show sizes in MB
alias mkdir='mkdir -p'
alias mv='mv -i'
alias rm='rm -i'
alias sudo='sudo -H'
alias osc52='printf "\x1b]52;;%s\x1b\\" "$(base64 <<< "$(date +"%Y/%m/%d %H:%M:%S"): hello")"'

# ls
alias ls='ls -Fh --color=auto'
alias la='ls -aFh --color=auto'
alias ll='ls -lh --color=auto'
alias lal='ls -alFh --color=auto'
alias lla='lal'

#############################################################################
# Suffix
#############################################################################
alias -s {md,markdown,txt}="$EDITOR"
alias -s {html,gif,mp4}='x-www-browser'
alias -s rb='ruby'
alias -s py='python'
alias -s hs='runhaskell'
alias -s php='php -f'
alias -s {jpg,jpeg,png,bmp}='feh'
alias -s mp3='mplayer'
function extract() {
	case $1 in
		*.tar.gz|*.tgz) tar xzvf "$1" ;;
		*.tar.xz) tar Jxvf "$1" ;;
		*.zip) unzip "$1" ;;
		*.lzh) lha e "$1" ;;
		*.tar.bz2|*.tbz) tar xjvf "$1" ;;
		*.tar.Z) tar zxvf "$1" ;;
		*.gz) gzip -d "$1" ;;
		*.bz2) bzip2 -dc "$1" ;;
		*.Z) uncompress "$1" ;;
		*.tar) tar xvf "$1" ;;
		*.arj) unarj "$1" ;;
	esac
}
alias -s {gz,tgz,zip,lzh,bz2,tbz,Z,tar,arj,xz}=extract

#alias rsync-git='rsync -a -C --filter=":- .gitignore"'
if (( $+commands[git] )) then
  alias ga="git add"
  alias gaA="git add -A"
  alias gb="git branch"
  alias gc="git commit"
  alias gca="git commit -a"
  alias gP="git push"
  alias gp="git pull"
  alias gs="git switch"
  alias gm="git merge"
  alias gu='git add . && git commit && git push'
fi

alias claude="$HOME/.claude/local/claude"

