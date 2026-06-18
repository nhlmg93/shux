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

Gracefully restart shux with L3 handoff semantics: checkpoint resurrection state, detach clients, and keep live pane PTYs/processes attached to the running daemon.

```bash
shux restart
```

Clients are detached during restart. Reattach with `./shux` after the handoff completes.

## `shux new-session`

Create a named session and initialize its first window/pane.

```bash
shux new-session -s work
```

## `shux kill-session`

Close all windows in a session, remove it from the daemon, and checkpoint the store. Killing the last session stops the daemon and clears on-disk resurrection state.

```bash
shux kill-session -t work
```

## `shux list-sessions`

List known daemon sessions by name.

```bash
shux list-sessions
```

## tmux-compatible scripting commands

These mirror common `tmux` CLI operations. Most accept `-t TARGET` where `TARGET` is a session name, `session:WINDOW`, or `session:WINDOW.PANE` (1-based indexes), or raw IDs (`s-1`, `w-2`, `p-3`).

| Command | Description |
| --- | --- |
| `has-session -t NAME` | Exit 0 if session exists |
| `new-window [-t TARGET]` | Create a window (with initial pane) |
| `kill-window [-t TARGET]` | Close a window |
| `kill-pane [-t TARGET]` | Close a pane |
| `select-window [-t TARGET]` | Set default window target |
| `split-window [-t TARGET] [-h\|-v]` | Split pane (`-h` left/right) |
| `send-keys [-t TARGET] KEYS...` | Send keys/text (`Enter`, `C-c`, literals) |
| `capture-pane [-t TARGET]` | Print pane screen snapshot |
| `rename-window [-t TARGET] NAME` | Rename window |
| `rename-pane [-t TARGET] NAME` | Rename pane |
| `list-commands` | List remote commands |
| `kill-server` | Shut down the daemon |
| `source-file PATH` | Reload Lua configuration |
| `list-clients` | List attached clients |
| `switch-client -t SESSION` | Switch attached client to another session |
| `show-options [OPTION]` | Show daemon options |
| `set-option OPTION VALUE` | Set a runtime option (`statusline` today) |
| `show-environment [SESSION]` | Show session environment variables |
| `set-environment [-t SESSION] VAR VALUE` | Set session environment |
| `list-keys` | List prefix key bindings |
| `bind-key KEY ACTION` | Bind a prefix key at runtime |
| `list-buffers` | List paste buffers |
| `paste-buffer [-t TARGET] [-b NAME]` | Paste buffer into pane |
| `resize-pane [-t TARGET] [-L\|-R\|-U\|-D] [N]` | Resize pane edges |
| `swap-pane [-t TARGET] [-L\|-R\|-U\|-D]` | Swap with neighbor pane |
| `break-pane [-t TARGET]` | Break pane into new window |
| `join-pane -t DEST [-s SOURCE]` | Move pane into another window |
| `select-layout [-t TARGET] PRESET` | Apply layout (`even-horizontal`, `even-vertical`, `main-horizontal`) |
| `choose-tree [-s\|-w]` | Open tree picker on attached client |
| `command-prompt` | Open `:` command line on attached client |
| `display-menu` | Placeholder (use tree or prefix bindings) |

```bash
shux has-session -t main
shux new-window -t main
shux split-window -t main:2 -h
shux send-keys -t main:2.1 ls Enter
shux capture-pane -t p-1
```

## `shux list-windows`

List windows from the running daemon without entering the TUI.

```bash
shux list-windows
shux list-windows --json
shux list-windows -t work
```

Default output is a tabular summary of the default session's windows. Use `--json` for machine-readable output.

## `shux list-panes`

List panes from the running daemon without entering the TUI.

```bash
shux list-panes
shux list-panes --json
shux list-panes -t work
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

## `shux ps`

List **live** daemon state (like `docker ps` for running containers).

```bash
shux ps              # panes (default)
shux ps --sessions   # session names only
shux ps --json
```

Requires a running daemon. The `list-sessions`, `list-windows`, and `list-panes` commands remain for scripting.

## `shux ls`

List **on-disk** resurrection store — manifest and journals (like `docker images` for stored layers).

```bash
shux ls
shux ls --json
```

Works without a running daemon. Store path comes from `shux.opt.state_dir`.

## Store maintenance

```bash
shux prune              # remove orphan journals
shux prune --dry-run
shux checkpoint         # save checkpoint from running daemon
shux rm                 # remove entire store (manifest + journals)
shux rm --force         # allow while daemon is running
```

Orphan journals are also removed when panes close and on each daemon checkpoint.

## `shux --version`

Print the shux version (currently `0.1.0`).
