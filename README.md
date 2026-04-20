# shux

> you shouldn't have.

A terminal multiplexer that just works.

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
- `Ctrl+B` then `w` - Create new window
- `Ctrl+B` then `n` - Next window
- `Ctrl+B` then `p` - Previous window
- `Ctrl+B` then `q` - Quit

### Config

Default config path:

```text
~/.config/shux/init.lua
```

Load an alternate config file at startup:

```bash
./shux --config /path/to/init.lua
```

Minimal config:

```lua
local shux = require("shux")

return shux.config({
  session = {
    name = "default",
  },
  shell = os.getenv("SHELL") or "/bin/sh",
})
```

Minimal plugin module:

```lua
-- ~/.config/shux/lua/plugins/dev.lua
local M = {}

function M.setup(shux)
  shux.set_session_name("dev")
end

return M
```

## Architecture

Single-process, actor-based design. No daemon, no fork.

```
┌─────────────────────────────────────────────────────────┐
│                        shux                              │
│                                                          │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐  │
│  │  Bubble Tea │───▶│   Session   │───▶│    Window   │  │
│  │    (UI)     │    │   (Actor)   │    │   (Actor)   │  │
│  └─────────────┘    └─────────────┘    └──────┬──────┘  │
│         ▲                                      │        │
│         │         libghostty VT               Pane       │
│         │              Engine                (Actor)     │
│         │                   │                    │        │
│         └───────────────────┴────────────────────┘        │
│                             │                           │
│                             ▼                           │
│                          ┌──────┐                       │
│                          │ PTY  │──▶ Shell              │
│                          └──────┘                       │
└─────────────────────────────────────────────────────────┘
```

**Data Flow:**
1. User input → Bubble Tea key events
2. `WriteToPane` message → Pane actor
3. libghostty parses VT sequences
4. `GetPaneContent` → RenderState API → UI redraw



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
