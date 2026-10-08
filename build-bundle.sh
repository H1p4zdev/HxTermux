#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOOS_TARGET="${GOOS_TARGET:-android}"
GOARCH_TARGET="${GOARCH_TARGET:-arm64}"
DIST="$ROOT/dist"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

mkdir -p "$DIST"
APP="$STAGE/HxTermux"
mkdir -p "$APP"

(
  cd "$ROOT"
  CGO_ENABLED=0 GOOS="$GOOS_TARGET" GOARCH="$GOARCH_TARGET" go build -trimpath -ldflags='-s -w' -o "$APP/hxtermux" .
  CGO_ENABLED=0 GOOS="$GOOS_TARGET" GOARCH="$GOARCH_TARGET" go -C fetcher build -trimpath -ldflags='-s -w' -o "$APP/hypexfetch" .
)

for PATH_TO_BUNDLE in \
  .aliases .autostart .colorscheme .config/lf .fonts .local .oh-my-zsh/custom/themes \
  .scripts .termux .tmux.conf .zshrc install.sh README.md LICENSE; do
  mkdir -p "$APP/$(dirname "$PATH_TO_BUNDLE")"
  cp -R "$ROOT/$PATH_TO_BUNDLE" "$APP/$PATH_TO_BUNDLE"
done

chmod 755 "$APP/hxtermux" "$APP/hypexfetch" "$APP/install.sh"
ARCHIVE="$DIST/HxTermux-${GOOS_TARGET}-${GOARCH_TARGET}.tar.gz"
tar -C "$STAGE" -czf "$ARCHIVE" HxTermux
(
  cd "$DIST"
  sha256sum "$(basename "$ARCHIVE")" > "$(basename "$ARCHIVE").sha256"
)
printf 'Created %s\n' "$ARCHIVE"
