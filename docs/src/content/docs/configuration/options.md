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
shux.opt.resurrection = true

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
| `state_dir` | XDG state dir | Resurrection journals + manifest directory |
| `resurrection` | `true` | Record PTY output for restore |

## Map leader

Set the prefix key with:

```lua
shux.g.mapleader = "<C-b>"
```

This must match how you define prefix keymaps in `keymaps.lua`.

## CLI overrides

`./shux --bash` sets the shell to `/bin/bash` when spawning a new daemon. It does not override `shux.opt` for an already-running backend.
