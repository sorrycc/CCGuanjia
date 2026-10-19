#!/usr/bin/env bash
# Runs CCGuanjia Dev with live reload (go tool mygo dev) beside the
# everyday CCGuanjia. MyGo gives it the identifier dev.sorrycc.ccguanjia.dev,
# so its own instance lock; CCGUANJIA_DATA_DIR gives it its own window state.
# The profiles it shows are the real ones: it reads ~/.claude and
# ~/.claude-*, and its config.json and shell.zsh are the everyday app's,
# unless CCGUANJIA_CONFIG_DIR moves them.
# Usage: scripts/dev.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export CCGUANJIA_DATA_DIR="${CCGUANJIA_DATA_DIR:-$HOME/Library/Application Support/CCGuanjia-dev}"

# MyGo creates its own data folder, but not one set with SetPath.
mkdir -p "$CCGUANJIA_DATA_DIR"

echo "==> mygo dev (data in $CCGUANJIA_DATA_DIR)"
cd "$ROOT"
exec go tool mygo dev
