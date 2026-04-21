---
title: Writing Plugins
description: How to write your own shux plugins.
---

## Writing Your First Plugin

Plugins are Lua modules that extend shux's behavior. Let's build a simple plugin that shows a status message when a new pane is created.

## Plugin Structure

Create `~/.config/shux/plugins/hello-status/init.lua`:

```lua
-- ~/.config/shux/plugins/hello-status/init.lua

local M = {}

function M.setup(opts)
  -- This runs when the plugin is loaded
  local name = opts and opts.name or "shux"
  print("Hello from " .. name .. "!")
end

return M
```

Load it in your config:

```lua
-- ~/.config/shux/config.lua
shux.load_plugin("hello-status", { name = "my-plugin" })
```

## Registering Actions

Plugins can register new actions that can be bound to keys:

```lua
function M.setup(opts)
  shux.register_action("say-hello", function(session, pane)
    pane.send_input("echo 'Hello from my plugin!'\n")
  end)
end
```

Then bind it in your config:

```lua
shux.bind("C-b h", "say-hello")
```

## Using Plugin Hooks

```lua
function M.setup(opts)
  -- Log when sessions are created
  shux.on("session_create", function(session)
    print("New session created: " .. session.name)
  end)

  -- Log when panes are split
  shux.on("pane_split", function(pane)
    print("Pane split: " .. tostring(pane.id))
  end)
end
```

## Plugin API Reference

| Function | Description |
|----------|-------------|
| `shux.register_action(name, fn)` | Register a new action |
| `shux.on(event, fn)` | Hook into a lifecycle event |
| `shux.get_session(name)` | Get a session by name |
| `shux.get_sessions()` | List all sessions |
| `shux.get_pane(id)` | Get a pane by ID |

## Best Practices

1. **Keep plugins small** — one plugin, one responsibility
2. **Use options** — make your plugin configurable
3. **Clean up resources** — handle the `session_destroy` hook
4. **Error handling** — use `pcall` for unsafe operations

## Next Steps

- Check the [Plugin Overview](/plugins/overview) for the full architecture
- See [Actions Reference](/reference/actions) for available action types
