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

| Key | Action |
| --- | --- |
| `ctrl+b d` | Detach this client; the backend keeps running. |
| `ctrl+b q` | Quit shux; shuts down the backend daemon. |
| `ctrl+b %` | Split the active pane left/right. |
| `ctrl+b "` | Split the active pane top/bottom. |
| `ctrl+b o` | Focus the next pane. |
| `ctrl+b x` | Close the active pane. If it is the last pane in a window, close that window; if it is the last window, quit shux. |
| `ctrl+b c` | Create a new window. |
| `ctrl+b n` | Next window. |
| `ctrl+b p` | Previous window. |
| `ctrl+b 1` through `ctrl+b 9` | Select window by number. |
| `ctrl+b 0` | Select window 10. |
| `ctrl+b ?` | List key bindings (stderr). |

Scroll the active pane through scrollback with `Page Up` / `Page Down`, or the mouse wheel over a pane.

Notes:

- `ctrl+c` is not a shux quit key; it is sent to the active pane.
- `ctrl+b` enters prefix mode and is not sent to the active pane.
- Detach with `ctrl+b d` when you want to leave the session running.
- Quit with `ctrl+b q` when you want to stop the shux backend.

## Configuration (Lua)

shux uses a Neovim-style Lua config loaded once at **daemon start**. If no config file exists, built-in defaults apply.

```text
~/.config/shux/
  init.lua
  lua/
    options.lua
    keymaps.lua
  plugin/              # eager-loaded plugin scripts
  pack/*/start/*/plugin/*.lua
```

Example `~/.config/shux/init.lua`:

```lua
shux.g.mapleader = "<C-b>"
require("options")
require("keymaps")
```

See [`runtime/example/`](runtime/example/) for a full starter config.

Options (`shux.opt`):

| Option | Default | Description |
| --- | --- | --- |
| `shell` | `/bin/sh` | Pane shell (`--bash` overrides at spawn) |
| `bind` | `127.0.0.1:23234` | Daemon listen address |
| `scrollback` | `10000` | libghostty scrollback lines |
| `state_dir` | XDG state dir | Resurrection journals + manifest |
| `resurrection` | `true` | Record PTY output for restore |

Plugins can register autocmds via `shux.api.shux_create_autocmd` and keymaps with Lua functions via `shux.keymap.set`.
