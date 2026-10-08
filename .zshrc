export ZSH=$HOME/.oh-my-zsh
ZSH_THEME="ma"
plugins=(
  git 
  bgnotify
  zsh-fzf-history-search
  zsh-autocomplete
  zsh-autosuggestions
  zsh-syntax-highlighting
)

PATH="$PREFIX/bin:$HOME/.local/bin:$PATH"
export PATH

export TERM=xterm-256color 

source $ZSH/oh-my-zsh.sh
[[ -r "$HOME/.config/lf/icons" ]] && source "$HOME/.config/lf/icons"

# Prediction List View
#zstyle ':autocomplete:*' default-context history-incremental-search-backward
#zstyle ':autocomplete:history-incremental-search-backward:*' min-input 1

[[ -r "$HOME/.aliases" ]] && source "$HOME/.aliases"
[[ -r "$HOME/.autostart" ]] && source "$HOME/.autostart"
