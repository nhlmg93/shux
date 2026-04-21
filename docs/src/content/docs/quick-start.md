---
title: Quick Start
description: Get up and running with shux in 5 minutes.
---

## Start Your First Session

```bash
./shux new -s mysession
```

This creates and attaches to a new session named `mysession`.

## Create Windows

Within shux, use the prefix key (default `Ctrl-a`) followed by:

| Action | Key |
|--------|-----|
| New window | `c` |
| Switch window | `n` / `p` (next/previous) |
| List windows | (coming soon) |

## Split Panes

Within a window, split the active pane:

| Action | Key |
|--------|-----|
| Split vertically | `Ctrl-a` then `%` |
| Split horizontally | `Ctrl-a` then `"` |
| Switch pane | `Ctrl-a` then arrow keys |
| Resize pane | `Ctrl-a` then `Ctrl` + arrow keys |

## Detach and Reattach

```bash
# Detach from current session
Ctrl-a d

# Reattach to your session
./shux attach -s mysession
```

## Key Properties of shux

1. **Sessions survive** — state is saved to disk on detach
2. **Real terminal emulation** — every pane runs a full Ghostty terminal
3. **Simple model** — sessions → windows → panes, nothing more

## What's Next?

- Read the [Concepts](/concepts/sessions) guide to understand the architecture
- Learn about [Lua Configuration](/config/lua-config) for customization
- Check the [Reference](/reference/commands) for all available commands
