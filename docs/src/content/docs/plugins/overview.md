---
title: Plugins
description: Extend shux with Lua plugins and autocmds.
---

:::note[Early API]
The plugin surface is still evolving. Treat APIs as experimental until documented behavior is covered by integration tests.
:::

shux loads Lua from two conventional locations under `~/.config/shux/`:

```text
plugin/                    # eager-loaded scripts
pack/*/start/*/plugin/*.lua  # pack-style layout (Neovim-inspired)
```

## Autocmds

Plugins can register autocmds via `shux.api.shux_create_autocmd`. Events include daemon and client lifecycle, plus pane and window changes:

| Event | When it fires |
| --- | --- |
| `DaemonStarted` | Daemon finished bootstrap |
| `ClientAttached` | A client connected |
| `ClientDetached` | A client disconnected |
| `PaneCreated` / `PaneClosed` | Pane lifecycle |
| `WindowCreated` / `WindowClosed` | Window lifecycle |
| `WindowLayoutChanged` | Pane splits or layout updates |

## Keymaps

Register interactive bindings with `shux.keymap.set`. See [Keymaps](/configuration/keymaps/).

## Example layout

Start from [`runtime/example/`](https://github.com/nhlmg93/shux/tree/master/runtime/example) and add `plugin/myplugin.lua` or a pack directory as needed.
