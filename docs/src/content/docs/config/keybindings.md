---
title: Keybindings
description: Configuring keymaps in shux.
---

## Keymaps

shux supports tmux-like keybindings through its Lua configuration. The keymap system binds key sequences to actions.

## Default Keybindings

| Key | Action |
|-----|--------|
| `Ctrl-a` | Prefix key |
| `Ctrl-a c` | New window |
| `Ctrl-a n` | Next window |
| `Ctrl-a p` | Previous window |
| `Ctrl-a %` | Vertical split |
| `Ctrl-a "` | Horizontal split |
| `Ctrl-a d` | Detach session |

## Custom Keybindings

In your `~/.config/shux/config.lua`:

```lua
-- Change the prefix key
shux.set_prefix("C-b")

-- Bind custom actions
shux.bind("C-b k", "kill-window")
shux.bind("C-b r", "reload-config")
```

## Key Format

Keys are specified in a readable format:

| Format | Meaning |
|--------|---------|
| `C-a` | Control+a |
| `M-a` | Meta/Alt+a |
| `C-a C-b` | Control+a, then Control+b |
| `Escape` | Escape key |
| `Up`, `Down`, `Left`, `Right` | Arrow keys |
| `Tab`, `Enter`, `Backspace` | Special keys |

## Action Registry

All available actions are defined in the [Actions Reference](/reference/actions). Actions are the target of keybindings and can also be triggered programmatically via the internal protocol.

## Next Steps

- Learn about [Lua Configuration](/config/lua-config) — the configuration system
- Check [Actions Reference](/reference/actions) — all available actions
