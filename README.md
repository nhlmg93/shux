# shux

> you shouldn't have.

A terminal multiplexer that just works.

## Features

- Full terminal emulation via Ghostty
- True color, mouse support, Unicode, and scrollback
- Single-process design with no daemon
- Explicit session/window/pane ownership loops in plain Go
- Disk-backed detach and restore
- Lua config and lightweight plugin hooks
- Single static-ish binary once Ghostty is built

## Quick Start

### Prerequisites

Build Ghostty's VT library once:

```bash
./build-ghostty.sh
```

That installs `libghostty-vt-static` to `/usr/local`.

### Build

```bash
export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:$PKG_CONFIG_PATH
go build ./cmd/shux
```

For a fully static binary:

```bash
export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:$PKG_CONFIG_PATH
go build -ldflags '-linkmode external -extldflags "-static"' ./cmd/shux
```

### Run

```bash
./shux
```

Keys:

- `Ctrl+B` then `w` — create window
- `Ctrl+B` then `n` — next window
- `Ctrl+B` then `p` — previous window
- `Ctrl+B` then `d` — detach, save snapshot, and exit
- `Ctrl+B` then `q` — quit

## Persistence Model

`shux` is single-process and disk-backed:

- start: attach to a saved snapshot if one exists, otherwise create a fresh session
- detach: save the current layout to disk and exit
- reattach: restore windows and panes from the saved snapshot

There is no tmux-style server, daemon, or client/server split.

## Config

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

`shux` uses explicit ownership loops instead of a separate actor framework.

```text
Bubble Tea UI
    │
    ▼
 session loop
    │
    ▼
 window loop
    │
    ▼
  pane loop ──▶ PTY ──▶ shell
    │
    └────────▶ libghostty render state
```

Data flow:

1. Bubble Tea normalizes input into `KeyInput` or `WriteToPane`
2. the active session forwards to the active window and pane
3. the pane writes to the PTY and feeds output into libghostty
4. pane updates trigger UI refresh messages
5. the UI asks for `GetPaneContent` and renders from cached pane content

## Testing

Run the full Docker-backed suite locally:

```bash
make test
```

That path builds the test image, builds Ghostty in Docker, and runs `go test ./...` inside the container.

For a native non-Docker run, use:

```bash
make test-native
```

This builds the pinned Ghostty VT library under `ghostty-build/usr` and runs `go test ./...` directly on the host.
It expects the same userland test tools as the Docker image, including `nano`, `vim`, and `less`.

GitHub Actions uses a dedicated native workflow on pushes and pull requests so Go dependencies and the Ghostty VT build can be cached across runs.

A separate Docker parity workflow covers the containerized test path on a schedule, on demand, and when Docker test infrastructure changes.

`shux` also includes fuzz targets for pure helpers like snapshot decoding, stat parsing, and row rendering.

## Dependencies

Build-time:

- Go 1.26+
- Zig
- pkg-config

Runtime:

- none beyond the built binary and terminal environment

## Why Ghostty?

Ghostty provides complete, embeddable terminal emulation:

- standards-compliant xterm behavior
- high performance native VT implementation
- modern features like true color and graphics protocols
- a clean library boundary through `libghostty-vt`

## License

MIT
