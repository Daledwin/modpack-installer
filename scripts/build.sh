#!/usr/bin/env bash
# Cross-compile the installer for Windows / Linux / macOS into ./dist.
# Requires Go (the embedded modpack.config default is baked into each binary).
set -euo pipefail
cd "$(dirname "$0")/.."

export CGO_ENABLED=0
mkdir -p dist

build() {
  local goos="$1" goarch="$2" out="$3"
  GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "-s -w" -o "dist/$out" ./cmd/installer
  echo "  ✓ dist/$out"
}

echo "Building installer binaries…"
build windows amd64 "installer-windows-amd64.exe"
build linux   amd64 "installer-linux-amd64"
build linux   arm64 "installer-linux-arm64"
build darwin  amd64 "installer-macos-intel"
build darwin  arm64 "installer-macos-apple-silicon"
echo "Done. Distribute ./dist plus install.sh / install.command (and modpack.config.json if not embedded)."
