---
title: Commands
description: Command-line reference for shux.
---

## Command-Line Reference

All shux commands use the `shux` binary.

## Session Commands

### `shux new`

Create and attach to a new session.

```bash
shux new -s session_name
shux new -s session_name -w 80 -h 24
```

| Flag | Description |
|------|-------------|
| `-s, --session` | Session name |
| `-w, --width` | Terminal width |
| `-h, --height` | Terminal height |

### `shux attach`

Attach to an existing session.

```bash
shux attach -s session_name
```

### `shux list`

List all sessions.

```bash
shux list
shux list --all
```

### `shux kill`

Kill a session (destroys all state).

```bash
shux kill -s session_name
```

## System Commands

### `shux version`

Print the shux version.

```bash
shux version
```

### `shux help`

Show help for a command.

```bash
shux help
shux help new
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error |
| `2` | Invalid arguments |
| `3` | Session not found |
| `4` | Permission denied |

## Next Steps

- Learn about [Actions](/reference/actions) — actions triggered by keybindings
- Learn about the [Protocol](/reference/protocol) — internal messaging
