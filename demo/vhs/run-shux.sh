#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export XDG_CONFIG_HOME="$ROOT/demo/vhs/config"
export XDG_STATE_HOME="$ROOT/demo/vhs/state"
export SHUX_DEMO_SHELL="$ROOT/demo/vhs/bash-shell.sh"
mkdir -p "$XDG_STATE_HOME"
exec "$ROOT/shux" --bash "$@"
