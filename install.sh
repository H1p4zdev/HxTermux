#!/usr/bin/env bash
set -euo pipefail

if ! command -v pkg >/dev/null; then
  printf 'Run this installer inside Termux.\n' >&2
  exit 1
fi

SCRIPT_PATH="${BASH_SOURCE[0]:-}"
ROOT=""
if [[ -n "$SCRIPT_PATH" && -f "$SCRIPT_PATH" ]]; then
  ROOT="$(cd -- "$(dirname -- "$SCRIPT_PATH")" && pwd)"
fi
if [[ -z "$ROOT" || ! -f "$ROOT/main.go" || ! -d "$ROOT/.colorscheme" ]]; then
  ROOT="$HOME/.local/share/hxtermux/source"
  command -v git >/dev/null || pkg install -y git
  if [[ -d "$ROOT/.git" ]]; then
    git -C "$ROOT" pull --ff-only origin main
  else
    mkdir -p "$(dirname "$ROOT")"
    git clone --depth 1 https://github.com/H1p4zdev/HxTermux.git "$ROOT"
  fi
fi

if [[ -x "$ROOT/hxtermux" ]]; then
  exec "$ROOT/hxtermux" --setup
fi

command -v go >/dev/null || pkg install -y golang
cd "$ROOT"
exec go run . --setup
