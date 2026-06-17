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

# Bundle the Linux/macOS pieces as a tar.gz — unlike a zip, tar preserves the
# executable bit, so install.sh / install.command stay runnable after transfer.
echo "Packaging unix bundle…"
tar -czf dist/modpack-installer-unix.tar.gz \
  install.sh install.command modpack.config.json \
  dist/installer-linux-amd64 dist/installer-linux-arm64 \
  dist/installer-macos-intel dist/installer-macos-apple-silicon
echo "  ✓ dist/modpack-installer-unix.tar.gz"

echo "Done. Windows: ship dist/installer-windows-amd64.exe. Linux/macOS: ship the tar.gz."
