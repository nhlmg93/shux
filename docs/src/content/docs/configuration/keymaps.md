---
title: Keymaps
description: Define and customize shux keybindings in Lua.
---

Keymaps are configured with `shux.keymap.set` in `lua/keymaps.lua` (or any file required from `init.lua`).

## Builtin actions

The example config wires tmux-like defaults:

```lua
local map = shux.keymap.set

map("prefix", "<leader>d", "detach", { desc = "Detach client" })
map("prefix", "<leader>q", "quit", { desc = "Quit when last client" })
map("prefix", "<leader>%", "split_lr", { desc = "Split left/right" })
map("prefix", "<leader>\"", "split_tb", { desc = "Split top/bottom" })
map("prefix", "<leader>o", "next_pane", { desc = "Next pane" })
map("prefix", "<leader>x", "close_pane", { desc = "Close pane" })
map("prefix", "<leader>&", "close_window", { desc = "Close window" })
map("prefix", "<leader>c", "new_window", { desc = "New window" })
map("prefix", "<leader>n", "next_window", { desc = "Next window" })
map("prefix", "<leader>p", "previous_window", { desc = "Previous window" })
map("prefix", "<leader>?", "list_keymaps", { desc = "List bindings" })
map("prefix", "<leader>[", "copy_mode_toggle", { desc = "Enter/exit copy mode" })
map("prefix", "<leader>]", "paste_register", { desc = "Paste copy register" })

for i = 1, 9 do
  map("prefix", "<leader>" .. i, "select_window_" .. i, { desc = "Window " .. i })
end
map("prefix", "<leader>0", "select_window_10", { desc = "Window 10" })

map("copy_mode", "h", "copy_left", { desc = "Move left" })
map("copy_mode", "j", "copy_down", { desc = "Move down" })
map("copy_mode", "k", "copy_up", { desc = "Move up" })
map("copy_mode", "l", "copy_right", { desc = "Move right" })
map("copy_mode", "w", "copy_word_forward", { desc = "Next word" })
map("copy_mode", "b", "copy_word_backward", { desc = "Previous word" })
map("copy_mode", "g", "copy_top", { desc = "Top of scrollback" })
map("copy_mode", "shift+g", "copy_bottom", { desc = "Bottom of scrollback" })
map("copy_mode", "pageup", "copy_page_up", { desc = "Page up" })
map("copy_mode", "pagedown", "copy_page_down", { desc = "Page down" })
map("copy_mode", "space", "copy_select_start", { desc = "Start selection" })
map("copy_mode", "y", "copy_yank_selection", { desc = "Yank selection and exit" })
map("copy_mode", "enter", "copy_yank_selection", { desc = "Yank selection and exit" })
map("copy_mode", "escape", "copy_cancel", { desc = "Exit copy mode" })
```

## Lua function bindings

Plugins and user config can bind Lua functions instead of builtin action names:

```lua
shux.keymap.set("prefix", "<leader>h", function()
  -- custom logic
end, { desc = "My action" })
```

## Listing bindings

Press `ctrl+b ?` to print current bindings to stderr, or see [Keybindings](/using/keybindings/) for the default table.
