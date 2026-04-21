---
title: Plugin Overview
description: An overview of shux's plugin system.
---

## Plugin System

shux supports plugins — Lua modules that extend its functionality. Plugins can add new actions, modify behavior, and integrate with external tools.

## Plugin Architecture

Plugins are loaded from the plugin directory on startup:

```
~/.config/shux/plugins/
├── my-plugin.lua
└── another-plugin/
    ├── init.lua
    └── (other files)
```

## How Plugins Work

Each plugin is a Lua module that exports a `setup` function:

```lua
-- ~/.config/shux/plugins/my-plugin.lua

function M.setup(opts)
  -- Initialize your plugin
  -- Register new actions
  -- Hook into lifecycle events
end

return M
```

## Loading Plugins

Enable plugins in your configuration:

```lua
-- ~/.config/shux/config.lua
shux.load_plugin("my-plugin", { key = "value" })
shux.load_plugin("another-plugin")
```

## Plugin Lifecycle

1. **Load** — Plugin module is loaded from disk
2. **Setup** — `setup()` function is called with options
3. **Run** — Plugin is active during session lifetime
4. **Unload** — Plugin resources are cleaned up on session end

## Available Plugin Hooks

| Hook | When Called |
|------|-------------|
| `on_session_create` | New session is created |
| `on_session_destroy` | Session is killed |
| `on_window_create` | New window is created |
| `on_pane_create` | New pane is created |
| `on_detach` | Session is detached |
| `on_attach` | Session is reattached |

## Next Steps

- Learn [Writing Plugins](/plugins/writing-plugins) — build your own plugins
- Check [Actions Reference](/reference/actions) — actions you can register
