# gomux

A minimal terminal multiplexer in Go, built to understand how tmux works.

## Overview

This project explores terminal multiplexing concepts through hands-on implementation:

- **PTY Management** - Pseudo-terminal creation and process spawning
- **Actor Model** - Using [gotor](https://github.com/nhlmg93/gotor) for message-passing architecture
- **Session/Window/Pane Hierarchy** - Like tmux's core abstractions
- **Raw Mode Terminal** - Direct control character handling

## Features

- Multiple panes within windows
- Multiple windows (tabs) within sessions
- Keyboard-driven navigation (Ctrl+A prefix)
- Process lifecycle management
- Graceful terminal state handling

## Installation

```bash
go get github.com/nhlmg93/gomux
go build
```

## Usage

```bash
./gomux
```

**Key bindings (Ctrl+A prefix):**

| Key | Action |
|-----|--------|
| `1`, `2` | Switch to pane 1/2 |
| `c` | Create new pane |
| `x` | Kill active pane |
| `n` | Next window |
| `p` | Previous window |
| `w` | Create new window |
| `q` | Quit |

## Architecture

```
Supervisor (top-level coordination)
  └── SessionActor (manages windows)
        └── WindowActor (manages panes)
              └── PaneActor (manages PTY)
```

Messages flow up (PaneExited → WindowEmpty → SessionEmpty) and commands flow down.

## Testing

```bash
go test ./...
```

## Learning Goals

This is a learning project focused on:
- Understanding terminal multiplexing internals
- Exploring actor model patterns in Go
- Building PTY management from scratch

Not intended for production use.

## License

MIT License - see LICENSE file
