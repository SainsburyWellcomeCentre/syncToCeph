#!/bin/sh
# Regenerate the command reference (docs/cli/) and the shell completion
# scripts (completions/) from the command definitions in the code.
# Run this after changing any command, flag or help text.
set -eu
cd "$(dirname "$0")/.."
echo "Regenerating docs/cli/ and completions/ ..."
rm -rf docs/cli
go run ./internal/tools/gendocs docs/cli completions
echo "Done."
