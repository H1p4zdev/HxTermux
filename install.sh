#!/usr/bin/env bash
set -euo pipefail

if ! command -v pkg >/dev/null 2>&1; then
  printf 'HxTermux setup must run inside Termux.\n' >&2
  exit 1
fi

SCRIPT_PATH="${BASH_SOURCE[0]:-}"
ROOT=""
if [[ -n "$SCRIPT_PATH" && -f "$SCRIPT_PATH" ]]; then
  SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
  if [[ -f "$SCRIPT_DIR/main.go" && -d "$SCRIPT_DIR/.colorscheme" ]]; then
    ROOT="$SCRIPT_DIR"
  fi
fi

if [[ -z "$ROOT" ]]; then
  if ! command -v git >/dev/null 2>&1; then
    printf 'Installing Git…\n'
    pkg update -y && pkg install -y git
  fi
  ROOT="$HOME/.local/share/hxtermux/source"
  mkdir -p "$(dirname "$ROOT")"
  if [[ -d "$ROOT/.git" ]]; then
    printf 'Updating HxTermux source…\n'
    git -C "$ROOT" pull --ff-only origin main
  elif [[ -e "$ROOT" ]]; then
    printf 'Install path exists but is not an HxTermux Git checkout: %s\n' "$ROOT" >&2
    exit 1
  else
    printf 'Downloading HxTermux…\n'
    git clone --depth 1 https://github.com/H1p4zdev/HxTermux.git "$ROOT"
  fi
fi

cd "$ROOT"
if [[ -x "$ROOT/hxtermux" ]]; then
  exec "$ROOT/hxtermux" --setup
fi
if ! command -v go >/dev/null 2>&1; then
  printf 'Installing Go for the HxTermux setup interface…\n'
  pkg update -y && pkg install -y golang
fi
cd "$ROOT"
go run . --setup
