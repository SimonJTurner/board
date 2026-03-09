# Board CLI shell completion
# Source this file to enable tab completion for the board command.
# Usage: source scripts/board-completion.sh
# Requires 'board' to be on PATH.
if [[ -n "${ZSH_VERSION:-}" ]]; then
  source <(board completion zsh 2>/dev/null) || true
else
  source <(board completion bash 2>/dev/null) || true
fi
