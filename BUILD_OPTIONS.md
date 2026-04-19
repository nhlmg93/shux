# Build Options for gomux

## Two Terminal Emulator Backends

gomux supports two terminal emulation backends via Go build tags:

### 1. Default: Pure Go (tonistiigi/vt100)

**Build:**
```bash
go build ./cmd/gomux
# or explicitly:
go build -tags !ghostty ./cmd/gomux
```

**Features:**
- ✅ 100% Pure Go (no CGO)
- ✅ Single binary (already)
- ✅ Basic VT100
- ✅ 16/256 colors
- ✅ Cross-compilation easy
- ❌ No scrollback
- ❌ No true color (24-bit)
- ❌ Limited escape sequences

**Best for:** Shells, simple apps, learning, distribution

---

### 2. Full: Ghostty (libghostty)

**Build:**
```bash
# 1. Build Ghostty C library first (one time)
./build-ghostty.sh

# 2. Build gomux with ghostty tag
export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:$PKG_CONFIG_PATH
go build -tags ghostty -ldflags '-linkmode external -extldflags "-static"' ./cmd/gomux
```

**Features:**
- ✅ Full terminal emulation
- ✅ Scrollback buffer (10k+ lines)
- ✅ True color (24-bit RGB)
- ✅ 256 colors
- ✅ Alternate screen (vim works perfectly)
- ✅ Mouse support
- ✅ Kitty graphics protocol
- ✅ Sixel graphics
- ✅ Full Unicode/emoji
- ✅ Excellent performance

**Best for:** Power users, vim/htop/full apps

---

## Comparison

| Feature | Default (vt100) | Ghostty |
|---------|-----------------|---------|
| **Build** | `go build` | `go build -tags ghostty` |
| **Pure Go** | ✅ Yes | ❌ CGO |
| **Single binary** | ✅ Yes | ✅ Yes (static link) |
| **Scrollback** | ❌ No | ✅ 10k+ lines |
| **True color** | ❌ No | ✅ Yes |
| **vim compatibility** | ⚠️ Basic | ✅ Perfect |
| **Cross-compile** | ✅ Easy | ⚠️ Harder |

## Quick Start

**For development/learning (default):**
```bash
go run ./cmd/gomux
```

**For production/full features (Ghostty):**
```bash
# One-time setup
./build-ghostty.sh

# Build with full emulation
go build -tags ghostty ./cmd/gomux
./gomux  # Now vim, htop, etc. work perfectly!
```

## Testing Both

```bash
# Test default (vt100)
go test ./...

# Test Ghostty (requires library installed)
go test -tags ghostty ./...
```

## Architecture

Both implementations share the same interface:
- `Term` struct
- `New()` / `Spawn()`
- `readLoop()` goroutine
- `Receive()` for actor messages
- `GetTermContent` for UI rendering

Only the internal terminal emulation differs:
- Default: `vt100.VT100.Content[][]rune`
- Ghostty: `libghostty.Terminal.GridRef()` cell API
