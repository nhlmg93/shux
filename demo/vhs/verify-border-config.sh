#!/usr/bin/env bash
# Verify pane border config: unit tests + isolated VHS recording.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

echo "== build =="
make build

echo "== border unit tests =="
go test ./internal/ui/... ./internal/cfg/... -count=1 -run 'Border|Split|Internal|Vertical|Draw'

echo "== vhs border config tape =="
command -v vhs >/dev/null || { echo "install vhs: https://github.com/charmbracelet/vhs" >&2; exit 1; }
command -v ttyd >/dev/null || { echo "install ttyd (required by vhs)" >&2; exit 1; }

rm -f demo/vhs/border-config-test.gif
vhs demo/vhs/border-config-test.tape

if [[ ! -s demo/vhs/border-config-test.gif ]]; then
  echo "border-config-test.gif missing or empty" >&2
  exit 1
fi

SIZE="$(wc -c < demo/vhs/border-config-test.gif)"
if [[ "$SIZE" -lt 10000 ]]; then
  echo "border-config-test.gif suspiciously small ($SIZE bytes)" >&2
  exit 1
fi

echo "OK: border-config-test.gif ($SIZE bytes)"
