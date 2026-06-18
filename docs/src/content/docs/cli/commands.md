---
title: CLI commands
description: shux command-line interface reference.
---

## `shux`

Default command when run from an interactive terminal: attach to the daemon, or spawn one if none is listening.

```bash
./shux
./shux --bash    # use /bin/bash when spawning a new daemon
```

| Flag | Description |
| --- | --- |
| `--bash` | Use `/bin/bash` for panes when spawning a **new** daemon; ignored when attaching to an existing daemon |

Non-interactive use:

- If stdin/stdout are not TTYs and `SHUX_DAEMON` is set, the process runs as the daemon child (internal spawn path).
- Otherwise, an interactive terminal is required.

## `shux attach`

Alias: `shux a`, `shux attach-session`

Same behavior as `./shux` from an interactive terminal.

```bash
shux attach
shux attach --bash   # only affects new daemon spawn
```

## `shux detach`

Alias: `shux detach-client`

Detach shux clients without entering the UI.

```bash
shux detach
```

Equivalent to `ctrl+b d` from inside an attached client.

## `shux restart`

Gracefully restart the shux daemon: checkpoint resurrection state, release the listen socket, spawn a replacement, and shut down the current instance.

```bash
shux restart
```

Clients are detached during restart. Reattach with `./shux` when the new daemon is ready.

## `shux --version`

Print the shux version (currently `0.1.0`).
