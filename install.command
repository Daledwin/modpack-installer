#!/usr/bin/env bash
# macOS double-click entry point: just delegates to install.sh.
cd "$(dirname "$0")"
./install.sh "$@"
read -n 1 -s -r -p "Press any key to close…"
echo
