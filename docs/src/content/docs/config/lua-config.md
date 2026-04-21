---
title: Lua Configuration
description: Customizing shux with Lua configuration files.
---

## Configuration with Lua

shux uses a Lua-based configuration system. A configuration file (`~/.config/shux/config.lua`) is loaded on startup and allows you to customize all aspects of shux behavior.

## Basic Configuration

```lua
-- ~/.config/shux/config.lua

-- Set the prefix key
shux.set_prefix("C-a")

-- Set the default shell
shux.set_shell("/bin/zsh")
```

## Configuration Structure

The configuration file is a Lua script that calls shux's configuration API:

```lua
-- Keybindings
shux.bind("C-a", "new-window")
shux.bind("C-a n", "next-window")
shux.bind("C-a p", "previous-window")

-- Options
shux.set_option("default-shell", "/bin/bash")
shux.set_option("history-limit", 10000)

-- Session defaults
shux.set_option("default-session-name", "main")
```

## Configuration Loading

1. shux looks for `~/.config/shux/config.lua`
2. If not found, uses built-in defaults
3. Configuration is parsed and applied before the first session starts

## Hot Reloading

Configuration changes take effect on the next session attach. The running session's configuration is not dynamically reloaded — it persists for the session's lifetime.

## Next Steps

- Learn about [Keybindings](/config/keybindings) — detailed keymap configuration
- Learn about [Options](/config/options) — all available configuration options
