---
title: Windows
description: Understanding shux windows.
---

## What is a Window?

A window is a container within a session. It holds a single full-screen terminal view, which may be split into multiple panes.

## Windows vs Panes

| | Window | Pane |
|---|---|---|
| **Level** | Session → Window → Pane | Session → Window → Pane |
| **Scope** | One full-screen view | A region within a window |
| **Splits** | Created by splitting windows | Created by splitting panes |
| **Fullscreen** | Each window is a separate full-screen view | Shares the window's view |

## Creating Windows

Within a session, press your prefix key (`Ctrl-a`) then `c` to create a new window.

## Switching Windows

| Action | Key |
|--------|-----|
| Next window | `Ctrl-a n` |
| Previous window | `Ctrl-a p` |
| Specific window | `Ctrl-a` then number |

## Window Properties

- Each window has its own title
- Each window maintains its own set of panes
- Windows are independent — closing one does not affect others

## Next Steps

- Learn about [Panes](/concepts/panes) — the regions within windows
- Learn about [Sessions](/concepts/sessions) — the container for windows
