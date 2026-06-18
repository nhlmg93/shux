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
| `ctrl+b c` | Create a new window. |
| `ctrl+b n` | Next window. |
| `ctrl+b p` | Previous window. |
| `ctrl+b 1` through `ctrl+b 9` | Select window by number. |
| `ctrl+b 0` | Select window 10. |
| `ctrl+b ?` | List key bindings (stderr). |

## Notes

- `ctrl+c` is not a shux quit key; it is sent to the active pane.
- `ctrl+b` enters prefix mode and is not sent to the active pane.
- Detach with `ctrl+b d` when you want to leave the session running.
- Quit with `ctrl+b q` when you want to stop the shux backend.

## Customizing bindings

Bindings are defined in Lua via `shux.keymap.set`. See [Keymaps](/configuration/keymaps/) and the [example config](https://github.com/nhlmg93/shux/tree/master/runtime/example) in the repository.
