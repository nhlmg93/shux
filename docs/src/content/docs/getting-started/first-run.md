---
title: First run
description: Start shux, attach to a running daemon, and detach cleanly.
---

## Start or attach

From an interactive terminal:

```bash
./shux
```

If no backend is running, `./shux` starts a local daemon and attaches your terminal. If a daemon is already running, it attaches to the existing backend.

Panes run an interactive shell. New daemons use `/bin/sh` by default. By default shux shows a bottom status bar and pane borders with labels; see [UI chrome](/configuration/options/#ui-chrome/) to customize visibility and styling.

## Use bash for new daemons

To start a new daemon whose panes use `/bin/bash`:

```bash
./shux --bash
```

`--bash` only affects a newly spawned daemon. Attaching to an already-running daemon does not change its shell policy.

## Detach and quit

| Goal | Key |
| --- | --- |
| Leave the session running | `ctrl+b` then `d` (detach) |
| Shut down the daemon | `ctrl+b` then `q` (quit) |

You can also detach from outside an attached client with:

```bash
shux detach
```

## Named sessions

Create another session (each gets its own windows and panes):

```bash
shux new-session -s work
shux attach -t work
```

Remove a session when you are done with it:

```bash
shux kill-session -t work
```

If you kill the last remaining session, the daemon stops and clears persisted resurrection data.

## What happens next

- **Detach** keeps the daemon, windows, panes, and resurrection journals running.
- **Quit** tears down the backend when the last client leaves (or when you explicitly quit).
- **Kill session** removes a session from the daemon; killing the last session stops the backend and clears the on-disk store.

Read [Daemon and clients](/concepts/daemon/) for the underlying model, and [Resurrection](/resurrection/overview/) for how state is preserved across detach and restart.
