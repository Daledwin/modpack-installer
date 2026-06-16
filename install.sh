#!/usr/bin/env bash
# Linux / macOS launcher: picks the right native binary in ./dist and runs it.
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"

OS="$(uname -s)"
ARCH="$(uname -m)"
case "$OS" in
  Linux)
    case "$ARCH" in
      aarch64|arm64) BIN="installer-linux-arm64" ;;
      *)             BIN="installer-linux-amd64" ;;
    esac ;;
  Darwin)
    case "$ARCH" in
      arm64) BIN="installer-macos-apple-silicon" ;;
      *)     BIN="installer-macos-intel" ;;
    esac ;;
  *)
    echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

BINPATH="$DIR/dist/$BIN"
if [ ! -x "$BINPATH" ]; then
  chmod +x "$BINPATH" 2>/dev/null || true
fi
if [ ! -f "$BINPATH" ]; then
  echo "Missing $BINPATH — run scripts/build.sh first." >&2; exit 1
fi
exec "$BINPATH" "$@"
