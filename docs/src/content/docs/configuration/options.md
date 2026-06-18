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

-- Chrome visibility and ANSI styling (see "UI chrome" below)
-- shux.opt.ui = {
--   statusline = true,
--   pane_borders = true,
--   pane_labels = true,
--   statusline_style = "reverse",
-- }

-- Status bar segment content (see "Status line" below)
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
| `statusline` | built-in default | Status bar **content** (left/right segments via Lua) |
| `ui` | see below | Status bar **visibility**, pane chrome, and ANSI styling |

## UI chrome

`shux.opt.ui` controls pane borders, labels, and how the status bar is shown and styled. Assign the whole table at once (`shux.opt.ui = { ... }`); dot notation on individual fields does not persist.

This is separate from `shux.opt.statusline`, which controls what text appears in the bar:

| Setting | Type | Purpose |
| --- | --- | --- |
| `shux.opt.ui.statusline` | boolean | Show or hide the bottom status bar |
| `shux.opt.statusline` | table | Left/right segment content (Lua strings or functions) |

Hide the bar but keep custom segment logic for later:

```lua
shux.opt.ui = { statusline = false }
shux.opt.statusline = {
  left = function(ctx) return ctx.session_id end,
}
```

### Fields

| Field | Default | Description |
| --- | --- | --- |
| `statusline` | `true` | Show the bottom status bar; `false` hides it and gives panes the full window height |
| `pane_borders` | `true` | Draw box-drawing borders around panes |
| `pane_labels` | `true` | Show pane title labels on borders |
| `statusline_style` | `"reverse"` | Status bar row styling (see below) |
| `search_match_ansi` | built-in | ANSI SGR prefix for copy-mode search matches in pane content |
| `search_active_ansi` | built-in | ANSI SGR prefix for the active copy-mode search match |
| `copy_mode_status_ansi` | built-in | ANSI SGR prefix for copy-mode overlay status text |

Unset ANSI fields use shux's built-in highlights. Set a field to override only that highlight.

### `statusline_style`

- `"reverse"` — reverse-video background for the full status row (default)
- `"plain"` — no extra styling on the status row
- any other string — ANSI SGR prefix prepended to the row; shux appends `\x1b[0m` after the text

Minimal UI (no status bar or borders):

```lua
shux.opt.ui = {
  statusline = false,
  pane_borders = false,
  pane_labels = false,
}
```

Custom status bar and overlay styling:

```lua
shux.opt.ui = {
  statusline_style = "plain",
  -- or an ANSI SGR prefix, e.g. "\27[1;36m"
  copy_mode_status_ansi = "\27[1;33m",
}
```

Custom search highlights:

```lua
shux.opt.ui = {
  search_match_ansi = "\27[48;5;24m",    -- dim blue background
  search_active_ansi = "\27[48;5;214m",  -- orange background
}
```

## Status line

When `shux.opt.ui.statusline` is `true` (the default), shux renders a bottom status row with:

- Left: `session_id | window_index:window_name | active_pane`
- Right: `[SYNC:ON|OFF]` plus hostname (or `LayoutSnapshot.Status` if set by a plugin)

Override segment **content** with `shux.opt.statusline` (independent of `shux.opt.ui.statusline` visibility):

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
