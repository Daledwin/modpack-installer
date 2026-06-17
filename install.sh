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
if [ ! -f "$BINPATH" ]; then
  echo "Missing $BINPATH — run scripts/build.sh first (or unpack the full dist/)." >&2
  exit 1
fi

# macOS: the binaries are unsigned, so a downloaded bundle is quarantined and
# Gatekeeper would kill them. Clear the quarantine attribute on the whole dist.
if [ "$OS" = "Darwin" ]; then
  xattr -dr com.apple.quarantine "$DIR/dist" 2>/dev/null || true
fi

chmod +x "$BINPATH" 2>/dev/null || true
if [ ! -x "$BINPATH" ]; then
  echo "Cannot make '$BINPATH' executable (it may be on a noexec mount or a" >&2
  echo "FAT/exFAT USB stick, or the exec bit was lost in a zip). Copy the folder" >&2
  echo "to your home directory and retry, or run:  chmod +x \"$BINPATH\"" >&2
  exit 1
fi

exec "$BINPATH" "$@"
