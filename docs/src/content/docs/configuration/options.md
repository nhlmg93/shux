---
title: Options
description: Runtime options set via shux.opt in Lua.
---

Set options in Lua before the daemon finishes starting, typically in `lua/options.lua`:

```lua
shux.opt.shell = "/bin/sh"
shux.opt.bind = "127.0.0.1:23234"
shux.opt.scrollback = 10000
shux.opt.journal_max_mb = 64
shux.opt.journal_replay_delay_ms = 200
shux.opt.resurrection = true
shux.opt.statusline = {
  left = function(ctx)
    return string.format("%s | %d:%s | %s", ctx.session_id, ctx.window_index, ctx.window_name, ctx.active_pane)
  end,
  right = function(ctx)
    return ctx.hostname
  end,
}

local state = shux.fn.stdpath("state")
if state and state ~= "" then
  shux.opt.state_dir = state
end
```

## Available options

| Option | Default | Description |
| --- | --- | --- |
| `shell` | `/bin/sh` | Pane shell (`--bash` overrides at spawn) |
| `bind` | `127.0.0.1:23234` | Daemon listen address |
| `scrollback` | `10000` | libghostty scrollback lines |
| `journal_max_mb` | `64` | Max on-disk journal size per pane (resurrection) |
| `journal_replay_delay_ms` | `200` | Delay before replaying journals into a respawned pane |
| `state_dir` | XDG state dir | Resurrection journals + manifest directory |
| `resurrection` | `true` | Record PTY output for restore |
| `statusline` | built-in default | Status bar segments (left/right) via Lua |

## Status line

By default, shux renders:

- Left: `session_id | window_index:window_name | active_pane`
- Right: hostname (or `LayoutSnapshot.Status` if set by a plugin)

Override with `shux.opt.statusline`:

```lua
shux.opt.statusline = {
  left = function(ctx)
    return string.format("s:%s w:%d %s", ctx.session_id, ctx.window_index, ctx.window_name)
  end,
  right = {
    "pane",
    function(ctx) return ctx.active_pane end,
    function(ctx) return ctx.hostname end,
  },
}
```

`left` and `right` accept:

- a string
- a function `(ctx) -> string`
- a list of strings/functions (joined with spaces)

Available `ctx` fields:

- `session_id`
- `window_id`
- `window_index`
- `window_name`
- `active_pane`
- `hostname`
- `title`
- `status`

## Map leader

Set the prefix key with:

```lua
shux.g.mapleader = "<C-b>"
```

This must match how you define prefix keymaps in `keymaps.lua`.

## CLI overrides

`./shux --bash` sets the shell to `/bin/bash` when spawning a new daemon. It does not override `shux.opt` for an already-running backend.
