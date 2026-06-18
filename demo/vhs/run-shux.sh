#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export SHUX_DEMO_ROOT="$ROOT"
export XDG_CONFIG_HOME="$ROOT/demo/vhs/config"
mkdir -p "$ROOT/demo/vhs/state"
if command -v mise >/dev/null 2>&1; then
  export SHUX_DEMO_NODE="$(mise which node 2>/dev/null || true)"
fi
exec "$ROOT/shux" --bash "$@"
