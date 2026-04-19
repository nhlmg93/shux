#!/bin/bash
# Build Ghostty static library for single-binary gomux

set -e

GHOSTTY_VERSION="1.0.0"
INSTALL_PREFIX="/usr/local"

echo "Building Ghostty (libghostty-vt-static)..."
echo "This enables full terminal emulation in a single binary"
echo ""

# Check for Zig
if ! command -v zig &> /dev/null; then
    echo "Zig not found. Installing..."
    # Download Zig 0.13.0 (what Ghostty uses)
    ZIG_URL="https://ziglang.org/download/0.13.0/zig-linux-x86_64-0.13.0.tar.xz"
    curl -L "$ZIG_URL" | tar -xJ -C /tmp
    export PATH="/tmp/zig-linux-x86_64-0.13.0:$PATH"
fi

# Clone Ghostty if needed
if [ ! -d "/tmp/ghostty" ]; then
    echo "Cloning Ghostty..."
    git clone --depth 1 --branch v${GHOSTTY_VERSION} https://github.com/ghostty-org/ghostty.git /tmp/ghostty
fi

cd /tmp/ghostty

# Build static library only
echo "Building libghostty-vt-static..."
zig build -Doptimize=ReleaseFast \
    -Dcpu=baseline \
    libghostty

# Install
echo "Installing to $INSTALL_PREFIX..."
sudo mkdir -p "$INSTALL_PREFIX/lib" "$INSTALL_PREFIX/include"
sudo cp zig-out/lib/libghostty-vt-static.a "$INSTALL_PREFIX/lib/"
sudo cp -r include/ghostty "$INSTALL_PREFIX/include/"

# Create pkg-config file
sudo tee "$INSTALL_PREFIX/lib/pkgconfig/libghostty-vt-static.pc" > /dev/null << 'EOF'
prefix=/usr/local
exec_prefix=${prefix}
libdir=${prefix}/lib
includedir=${prefix}/include

Name: libghostty-vt-static
Description: Ghostty terminal emulator library (static)
Version: 1.0.0
Libs: -L${libdir} -lghostty-vt-static
Cflags: -I${includedir}
EOF

echo ""
echo "✓ Ghostty static library installed!"
echo ""
echo "Now build gomux:"
echo "  export PKG_CONFIG_PATH=$INSTALL_PREFIX/lib/pkgconfig:\$PKG_CONFIG_PATH"
echo "  go build -ldflags '-linkmode external -extldflags \"-static\"' ./cmd/gomux"
echo ""
echo "You'll get a single static binary with full terminal emulation!"
