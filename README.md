# shux

shux is an experimental terminal multiplexer with a small tmux-like attach flow.

## Usage

Start or attach to shux:

```bash
./shux
```

If no backend is running, `./shux` starts a local daemon and attaches the current terminal. If a backend is already running, it attaches to the existing daemon.

Panes run an interactive shell. New daemons use `/bin/sh` by default:

```bash
./shux
```

To start a new daemon whose panes use `/bin/bash`:

```bash
./shux --bash
```

`--bash` only affects a newly spawned daemon. Attaching to an already-running daemon does not change its shell policy.

## Keys

shux uses a tmux-style prefix key:

```text
ctrl+b
```

Implemented bindings:

```text
ctrl+b d     detach this client; backend keeps running
ctrl+b q     quit shux; shuts down the backend daemon
ctrl+b %     split pane left/right
ctrl+b "     split pane top/bottom
ctrl+b o     focus next pane
```

Notes:

- `ctrl+c` is not a shux quit key.
- Detach with `ctrl+b d` when you want to leave the session running.
- Quit with `ctrl+b q` when you want to stop the shux backend.
