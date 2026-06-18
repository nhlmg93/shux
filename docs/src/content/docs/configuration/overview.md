---
title: Configuration overview
description: Neovim-style Lua configuration for shux.
---

shux uses a Neovim-style Lua config loaded once at **daemon start**. If no config file exists, built-in defaults apply.

## Config directory

```text
~/.config/shux/
  init.lua
  lua/
    options.lua
    keymaps.lua
  plugin/              # eager-loaded plugin scripts
  pack/*/start/*/plugin/*.lua
```

## Starter `init.lua`

```lua
shux.g.mapleader = "<C-b>"
require("options")
require("keymaps")
```

Copy the full example from [`runtime/example/`](https://github.com/nhlmg93/shux/tree/master/runtime/example) in the repository:

```bash
mkdir -p ~/.config/shux
cp -r runtime/example/* ~/.config/shux/
```

## When config is read

Lua runs when a **new daemon** starts. Attaching to an existing daemon does not reload configuration. To pick up config changes, restart the daemon (see `shux restart` and [CLI commands](/cli/commands/)).

## UI and theme

`shux.opt.ui` controls status bar visibility, pane borders, pane labels, and ANSI styling. See [UI chrome](/configuration/options/#ui-chrome/) in the Options reference for field defaults and examples.

## Next steps

- [Options](/configuration/options/) — `shux.opt` settings
- [Keymaps](/configuration/keymaps/) — prefix bindings and custom actions
- [Plugins](/plugins/overview/) — extend shux with Lua scripts
