#!/usr/bin/env bash
# Prepare real-user shux for VHS debugging (no isolated demo config).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
LOG="$ROOT/demo/vhs/debug-user.log"
: >"$LOG"

echo "=== shux debug snapshot $(date -Is) ===" | tee -a "$LOG"
echo "config: $HOME/.config/shux/init.lua" | tee -a "$LOG"
cat "$HOME/.config/shux/init.lua" | tee -a "$LOG"
echo "--- store ---" | tee -a "$LOG"
"$ROOT/shux" ls 2>&1 | tee -a "$LOG" || true
echo "--- port ---" | tee -a "$LOG"
ss -tlnp 2>/dev/null | rg 23234 | tee -a "$LOG" || echo "no daemon on 23234" | tee -a "$LOG"
