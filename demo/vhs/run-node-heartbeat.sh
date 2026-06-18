#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NODE="${SHUX_DEMO_NODE:-$(command -v node)}"
exec "$NODE" "$ROOT/demo/vhs/node-heartbeat.js"
