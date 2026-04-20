# shux

A terminal multiplexer in Go with full terminal emulation via Ghostty.

## Features

- **Full Terminal Emulation** - Complete VT220/xterm compatibility via Ghostty
- **Scrollback Buffer** - 10,000+ lines of history
- **True Color** - 24-bit RGB color support
- **Mouse Support** - Full mouse integration
- **Kitty Graphics** - Modern terminal graphics protocol
- **Unicode/Emoji** - Full Unicode support
- **Actor Architecture** - Clean concurrency with goroutines
- **Single Binary** - Static link everything into one file

## Quick Start

### Prerequisites

You need to build Ghostty's C library once:

```bash
./build-ghostty.sh
```

This installs `libghostty-vt-static` to `/usr/local`.

### Build

```bash
export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:$PKG_CONFIG_PATH
go build ./cmd/shux
```

For a fully static binary (no dynamic dependencies):

```bash
export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:$PKG_CONFIG_PATH
go build -ldflags '-linkmode external -extldflags "-static"' ./cmd/shux
```

### Run

```bash
./shux
```

Keys:
- `Ctrl+A` then `c` - Create new window
- `Ctrl+A` then `n` - Next window
- `Ctrl+A` then `p` - Previous window
- `Ctrl+A` then `q` - Quit

## Architecture

```
User Input → Bubble Tea → Actor System → Term → PTY → Shell
                                    ↓
                              libghostty (Ghostty terminal)
                                    ↓
                              GridRef API → Render to UI
```

## Dependencies

**Build-time:**
- Go 1.21+
- Zig (for building Ghostty)
- pkg-config

**Runtime:**
- None (static binary)

## Why Ghostty?

Ghostty provides the most complete terminal emulation available:
- Standards-compliant (xterm audit complete)
- High performance (Zig-compiled, SIMD)
- Modern features (Kitty graphics, true color)
- Embeddable library (`libghostty-vt`)

## License

MIT
