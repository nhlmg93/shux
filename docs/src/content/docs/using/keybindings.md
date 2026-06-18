---
title: Keybindings
description: Default prefix keybindings in shux.
---

shux uses a tmux-style prefix key:

```text
ctrl+b
```

Press the prefix, then a second key to run a shux command. The prefix is not sent to the active pane.

## Default bindings

| Key | Action |
| --- | --- |
| `ctrl+b d` | Detach this client; the backend keeps running. |
| `ctrl+b q` | Quit shux; shuts down the backend daemon. |
| `ctrl+b %` | Split the active pane left/right. |
| `ctrl+b "` | Split the active pane top/bottom. |
| `ctrl+b o` | Focus the next pane. |
| `ctrl+b x` | Close the active pane. If it is the last pane in a window, close that window; if it is the last window, quit shux. |
| `ctrl+b &` | Close the active window (all panes). If it is the last window, quit shux. |
| `ctrl+b :` | Open the command prompt (tmux-style). Type a command and press Enter. |
| `ctrl+b s` | Tree view with sessions collapsed (tmux `choose-tree -s`). |
| `ctrl+b w` | Tree view with windows collapsed (tmux `choose-tree -w`). |
| `ctrl+b c` | Create a new window. |
| `ctrl+b n` | Next window. |
| `ctrl+b p` | Previous window. |
| `ctrl+b 1` through `ctrl+b 9` | Select window by number. |
| `ctrl+b 0` | Select window 10. |
| `ctrl+b ?` | List key bindings (stderr). |
| `ctrl+b [` | Enter or exit copy mode. |
| `ctrl+b ]` | Paste from the shux copy register into the active pane. |

## Notes

- `ctrl+c` is not a shux quit key; it is sent to the active pane.
- `ctrl+b` enters prefix mode and is not sent to the active pane.
- Detach with `ctrl+b d` when you want to leave the session running.
- Quit with `ctrl+b q` when you want to stop the shux backend.
- Copy mode status bar and search highlight colors are configured via `shux.opt.ui`; see [Options](/configuration/options/#ui-chrome/) under "UI chrome".

## Command prompt

Press `ctrl+b :` to open a tmux-style command line at the bottom of the screen. Type a command and press Enter; press Escape to cancel.

Supported commands include:

| Command | Action |
| --- | --- |
| `detach` | Detach this client. |
| `quit` / `exit` | Quit shux. |
| `new-window` | Create a new window. |
| `next-window` / `previous-window` | Cycle windows. |
| `select-window -t N` | Select window by number (1-based). |
| `split-window -h` / `-v` | Split active pane left/right or top/bottom. |
| `kill-pane` / `kill-window` | Close pane or window. |
| `rename-window NAME` / `rename-pane NAME` | Rename (or open rename prompt if name omitted). |
| `select-pane -L/-R/-U/-D` | Focus pane in direction. |
| `display-panes` | Show pane numbers for quick select. |
| `list-keymaps` / `help` | List prefix bindings (stderr). |
| `toggle-sync-panes` | Toggle synchronize panes. |
| `toggle-pane-zoom` | Toggle pane zoom. |

Unknown commands print an error to stderr and leave the session running.

## Tree view

tmux-style session/window/pane tree (`choose-tree`):

| Key | Action |
| --- | --- |
| `ctrl+b s` | Open tree with sessions collapsed. |
| `ctrl+b w` | Open tree with windows collapsed. |
| `0`–`9` | Quick-select visible row (shortcut column). |
| `↑` / `↓` or `k` / `j` | Move selection. |
| `←` / `→` or `h` / `l` | Collapse / expand branch. |
| `Enter` | Select session, window, or pane. |
| `t` | Toggle tag on current item. |
| `T` | Clear all tags. |
| `X` | Kill all tagged items. |
| `f` | Filter list (substring match on names). |
| `/` or `?` | Search forward (prompt). |
| `n` / `N` | Repeat search forward / backward. |
| `O` | Cycle sort order (index / name). |
| `r` | Reverse sort order. |
| `J` / `K` | Swap window with next / previous in session. |
| `m` / `M` | Mark / clear marked pane. |
| `x` | Kill selected pane, window, or session. |
| `<` / `>` | Scroll session preview thumbnails. |
| `H` | Jump to current location in the tree. |
| `Esc` / `q` | Close tree view. |

Command prompt equivalents: `choose-tree`, `choose-session`, `choose-window`.

Preview pane shows a snapshot of the selected item when available. Session rows show a horizontal thumbnail strip of windows (`<` / `>` scroll when there are more windows than fit). Tree lines use box-drawing connectors; tagged rows show `*`.

## Customizing bindings

Bindings are defined in Lua via `shux.keymap.set`. See [Keymaps](/configuration/keymaps/) and the [example config](https://github.com/nhlmg93/shux/tree/master/runtime/example) in the repository.
