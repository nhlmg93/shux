#!/usr/bin/env bash
# Clear corrupt resurrection store and restart the real shux daemon.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
LOG="$ROOT/demo/vhs/debug-user.log"

echo "=== shux debug fix $(date -Is) ===" | tee -a "$LOG"
"$ROOT/shux" rm --force 2>&1 | tee -a "$LOG"

fuser -k 23234/tcp 2>/dev/null || true
sleep 0.3

"$ROOT/shux" --bash </dev/null >>"$LOG" 2>&1 &
for _ in $(seq 1 40); do
  if "$ROOT/shux" list-sessions >/dev/null 2>&1; then
    echo "daemon ready" | tee -a "$LOG"
    "$ROOT/shux" ls 2>&1 | tee -a "$LOG"
    exit 0
  fi
  sleep 0.25
done
echo "daemon did not become ready" | tee -a "$LOG"
exit 1
