#!/usr/bin/env bash
set -euo pipefail
# Run from the directory containing the downloaded mkui/ folder.
cd mkui
git init -q && git add -A && git commit -qm "mkui: makefile TUI, terminal-native theming"
go mod tidy
make check
echo "ready -- now run: claude"
