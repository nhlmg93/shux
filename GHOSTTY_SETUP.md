# Setting up Ghostty (libghostty) for gomux

## Quick Summary

Ghostty provides the **best terminal emulation** available, but requires:
1. Building the C library (via Zig)
2. Go bindings (already have: `go-libghostty`)

## Features You Get

| Feature | tonistiigi/vt100 | libghostty |
|---------|------------------|------------|
| **Scrollback buffer** | ❌ | ✅ (10k+ lines) |
| **256 colors** | ✅ | ✅ |
| **True color (24-bit)** | ❌ | ✅ |
| **Alternate screen** | ❌ | ✅ (vim works!) |
| **Mouse support** | ❌ | ✅ |
| **Kitty graphics** | ❌ | ✅ |
| **Sixel** | ❌ | ✅ |
| **Unicode/emoji** | Basic | ✅ Full |
| **Performance** | Good | **Excellent** |

## Build Instructions

### 1. Install Zig
```bash
# macOS
brew install zig

# Linux
curl -L https://ziglang.org/download/0.13.0/zig-linux-x86_64-0.13.0.tar.xz | tar -xJ
# Add to PATH
```

### 2. Build libghostty
```bash
git clone https://github.com/ghostty-org/ghostty.git
cd ghostty

# Build the library only
zig build -Doptimize=ReleaseFast libghostty

# Install
sudo zig build -Doptimize=ReleaseFast -p /usr/local libghostty
```

### 3. Update Go build
```bash
# PKG_CONFIG_PATH needs to find libghostty
export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:$PKG_CONFIG_PATH
go build ./...
```

## Simpler Alternative

If Zig is too complex, use **libtsm** (C) instead:
```bash
# Ubuntu/Debian
sudo apt install libtsm-dev

# Then use our previous C wrapper
```

## Current Recommendation

**For now: Keep tonistiigi/vt100**
- Pure Go (no build dependencies)
- Good enough for shells/basic apps
- Simple to distribute

**When ready for full features:**
- Switch to Ghostty (best emulation)
- Or libtsm (simpler C library)

## The Trade-off

| Approach | Build Complexity | Features | Distribution |
|----------|-----------------|----------|--------------|
| tonistiigi/vt100 | Zero | Basic | Easy |
| libtsm | Low | Full | Moderate |
| Ghostty | Medium | **Best** | Complex |

Your call: stay simple or go for full emulation?
