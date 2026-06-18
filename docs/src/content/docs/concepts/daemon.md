---
title: Daemon and clients
description: How shux separates the long-running backend from attachable clients.
---

shux splits terminal multiplexing into two roles:

```text
┌─────────────┐     SSH      ┌──────────────────┐
│   Client    │ ◄──────────► │  shux daemon     │
│ (your TTY)  │              │  sessions/panes  │
└─────────────┘              └──────────────────┘
```

## Daemon

The daemon is the long-running process that owns:

- Sessions, windows, and panes
- PTY processes (shells and running programs)
- Lua configuration loaded at **daemon start**
- Resurrection journals and layout checkpoints

By default the daemon listens on `127.0.0.1:23234` (configurable via `shux.opt.bind`).

## Clients

A client is an interactive terminal attached to the daemon UI. Multiple clients can attach at once; they share the same backend state.

When you run `./shux` from a TTY, you become a client. Detaching (`ctrl+b d`) disconnects your terminal without stopping the daemon.

## Spawn vs attach

| Situation | What `./shux` does |
| --- | --- |
| No daemon on the bind address | Spawn a new daemon, bootstrap a default session, attach |
| Daemon already listening | Attach only |

Configuration and shell policy are fixed when the daemon starts. Attaching later does not reload `init.lua` or change options.

## Non-interactive entry points

- `shux attach` — same as `./shux` from an interactive terminal
- `shux detach` — detach clients without entering the UI
- `shux restart` — graceful daemon restart with checkpoint handoff

See [CLI commands](/cli/commands/) for full reference.
