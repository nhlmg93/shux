---
title: Actions
description: All available actions in shux.
---

## Actions Reference

Actions are the fundamental operations in shux. They are bound to keys and can also be triggered programmatically.

## Session Actions

| Action | Description |
|--------|-------------|
| `new-session` | Create a new session |
| `attach-session` | Attach to an existing session |
| `detach-session` | Detach from current session |
| `kill-session` | Kill a session |
| `list-sessions` | List all sessions |

## Window Actions

| Action | Description |
|--------|-------------|
| `new-window` | Create a new window in current session |
| `kill-window` | Kill the current window |
| `next-window` | Switch to the next window |
| `previous-window` | Switch to the previous window |
| `select-window` | Select a specific window by name/index |
| `rename-window` | Rename the current window |

## Pane Actions

| Action | Description |
|--------|-------------|
| `split-window` | Split the current pane |
| `split-window-h` | Split horizontally (side by side) |
| `split-window-v` | Split vertically (top and bottom) |
| `select-pane` | Select a specific pane |
| `select-pane-up` | Move focus to pane above |
| `select-pane-down` | Move focus to pane below |
| `select-pane-left` | Move focus to pane to the left |
| `select-pane-right` | Move focus to pane to the right |
| `resize-pane` | Resize the current pane |
| `resize-pane-up` | Increase pane height upward |
| `resize-pane-down` | Increase pane height downward |
| `resize-pane-left` | Increase pane width leftward |
| `resize-pane-right` | Increase pane width rightward |
| `kill-pane` | Kill the current pane |

## Input Actions

| Action | Description |
|--------|-------------|
| `send-keys` | Send keystrokes to the active pane |
| `paste` | Paste from clipboard |

## Configuration Actions

| Action | Description |
|--------|-------------|
| `reload-config` | Reload the configuration file |
| `show-options` | Show current options |

## Implementation Status

| Status | Meaning |
|--------|---------|
| ✅ | Implemented and working |
| 🟡 | Implemented but may have edge cases |
| 🔴 | Not yet implemented |

## Next Steps

- Learn about [Keybindings](/config/keybindings) — how to bind these actions to keys
- Learn about [Writing Plugins](/plugins/writing-plugins) — register custom actions
