---
title: Options
description: Configuration options for shux.
---

## Configuration Options

shux provides various options that control runtime behavior. These are set in your Lua configuration file.

## Available Options

### General Options

| Option | Default | Description |
|--------|---------|-------------|
| `default-shell` | `/bin/sh` | Shell to use for new panes |
| `default-session-name` | `""` | Default session name (empty = prompt) |
| `history-limit` | `10000` | Scrollback buffer lines |
| `status-position` | `bottom` | Status bar position |

### Session Options

| Option | Default | Description |
|--------|---------|-------------|
| `detach-kill` | `false` | Kill session on detach |
| `reattach` | `true` | Auto-reattach to existing session |
| `session-dir` | `~/.local/share/shux/` | Directory for session storage |

### Display Options

| Option | Default | Description |
|--------|---------|-------------|
| `default-width` | `80` | Default terminal width |
| `default-height` | `24` | Default terminal height |
| `mouse` | `false` | Enable mouse support |

## Setting Options

```lua
-- In ~/.config/shux/config.lua
shux.set_option("default-shell", "/bin/zsh")
shux.set_option("history-limit", 50000)
shux.set_option("session-dir", "/tmp/shux-sessions")
```

## Next Steps

- Learn about [Lua Configuration](/config/lua-config) — how to structure your config file
- Learn about [Keybindings](/config/keybindings) — customize your keymap
