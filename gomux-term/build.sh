#!/bin/bash
# Build simple terminal emulator as static library

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="$SCRIPT_DIR/build"

echo "Building simple terminal emulator..."

# Create build directory
mkdir -p "$BUILD_DIR"

# Compile our simple terminal (no dependencies!)
echo "Compiling simple_term.c..."
gcc -c -fPIC -O2 -o "$BUILD_DIR/simple_term.o" "$SCRIPT_DIR/src/simple_term.c"

# Create static library
echo "Creating static library..."
ar rcs "$BUILD_DIR/libgomux_term.a" "$BUILD_DIR/simple_term.o"

echo "Build complete: $BUILD_DIR/libgomux_term.a"
echo ""
echo "This is a minimal VT100 emulator with:"
echo "  - Cursor movement (ESC[H, ESC[A/B/C/D])"
echo "  - Clear screen (ESC[2J)"
echo "  - Colors (ESC[30-37m, ESC[40-47m)"
echo "  - Basic text, newlines, backspace"
echo "  - No external dependencies!"
