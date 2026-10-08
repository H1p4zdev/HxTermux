#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if ! command -v pkg >/dev/null 2>&1; then
  printf 'HxTermux setup must run inside Termux.\n' >&2
  exit 1
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
