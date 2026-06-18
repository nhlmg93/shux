#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FILE="$ROOT/demo/vhs/draft.txt"
cat > "$FILE" << 'EOF'
# shux resurrection demo
edited in neovim
EOF
exec nvim -n "$FILE"
