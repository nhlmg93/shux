#!/usr/bin/env bash
# Reset demo state and start an isolated shux daemon for VHS recording.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export SHUX_DEMO_ROOT="$ROOT"
export XDG_CONFIG_HOME="$ROOT/demo/vhs/config"
STATE_DIR="$ROOT/demo/vhs/state"
LOG="$ROOT/demo/vhs/daemon.log"
mkdir -p "$STATE_DIR"

if command -v mise >/dev/null 2>&1; then
  export SHUX_DEMO_NODE="$(mise which node 2>/dev/null || true)"
fi

fuser -k 23299/tcp 2>/dev/null || true
sleep 0.2
rm -rf "$STATE_DIR"/*
mkdir -p "$STATE_DIR"
rm -f "$ROOT/demo/vhs/draft.txt" "$ROOT/demo/vhs/draft.txt.save"

# Non-interactive shux starts the daemon child (see main.go isDaemonChild).
"$ROOT/shux" --bash </dev/null >>"$LOG" 2>&1 &
for _ in $(seq 1 40); do
  if "$ROOT/shux" list-sessions >/dev/null 2>&1; then
    exit 0
  fi
  sleep 0.25
done
echo "demo: daemon did not become ready" >&2
exit 1
