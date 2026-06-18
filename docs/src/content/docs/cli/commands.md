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
shux attach -t work
shux attach --bash   # only affects new daemon spawn
shux attach -C       # experimental control mode on stdin/stdout
```

When multiple sessions exist, plain `shux attach` targets the daemon default session (`main` unless changed by restoration).

### Control mode (experimental)

`shux attach -C` switches the attach client to a line-oriented protocol for automation.
It runs over stdin/stdout (no full-screen UI), and may change between releases.

#### Incoming notifications

Subscribe with `subscribe pane-output` and/or `subscribe layout-change`.

- `%output ...` emits pane screen snapshots when pane output updates.
- `%layout ...` emits window layout updates.

#### Commands

Each command is one line:

```text
subscribe pane-output layout-change
new-window
split horizontal
select-pane p-2
capture-pane
```

Response lines:

- `ok ...` for success
- `error ...` for invalid commands or rejected operations

`capture-pane` returns quoted text from the cached pane screen snapshot.

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

## `shux new-session`

Create a named session and initialize its first window/pane.

```bash
shux new-session -s work
```

## `shux list-sessions`

List known daemon sessions by name.

```bash
shux list-sessions
```

## `shux list-windows`

List windows from the running daemon without entering the TUI.

```bash
shux list-windows
shux list-windows --json
```

Default output is a tabular summary of the default session's windows. Use `--json` for machine-readable output.

## `shux list-panes`

List panes from the running daemon without entering the TUI.

```bash
shux list-panes
shux list-panes --json
```

Default output is a tabular summary of pane geometry. Use `--json` for automation and scripting.

## `shux display-message FORMAT`

Render a format string using daemon introspection variables.

```bash
shux display-message '#{pane_id}'
shux display-message '#{session_id}:#{window_id}:#{pane_id}'
shux display-message '#{pane_id}' --json
```

Supported format variables:

- `#{session_id}`
- `#{window_id}`
- `#{window_index}`
- `#{pane_id}`
- `#{pane_index}`

Default output prints the rendered message text. Use `--json` to return a structured payload that includes context fields and `message`.

## `shux --version`

Print the shux version (currently `0.1.0`).
