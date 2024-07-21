autoload -Uz colors && zsh-defer colors
#--------------------------------------------------------------#
##          Post Execution                                    ##
#--------------------------------------------------------------#

if ! builtin command -v sheldon > /dev/null 2>&1; then
	if ! builtin command -v compinit > /dev/null 2>&1; then
		autoload -Uz compinit
		if [[ -n ${ZDOTDIR}/.zcompdump(#qN.mh+24) ]]; then
			compinit
		else
			compinit -C
		fi
	fi
fi

dircolor
