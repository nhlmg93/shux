---
title: Panes
description: Understanding shux panes.
---

## What is a Pane?

A pane is the smallest unit in shux's hierarchy. Each pane is a full Ghostty terminal instance running inside a split region of a window.

## Pane Hierarchy

```
Session
└── Window
    └── Pane (1)
    └── Pane (2)
    └── Pane (3)
```

## Splitting Panes

Within a window:

| Split Type | Key |
|------------|-----|
| Vertical split | `Ctrl-a %` |
| Horizontal split | `Ctrl-a "` |

## Navigating Panes

| Action | Key |
|--------|-----|
| Switch to pane | `Ctrl-a` + arrow keys |
| Resize pane | `Ctrl-a` + `Ctrl` + arrow keys |

## Pane Independence

Each pane runs a separate Ghostty terminal. This means:

- Each pane has its own PTY (pseudo-terminal)
- Each pane runs its own shell process
- Each pane emulates a real terminal (colors, cursor, paste, etc.)

This is what makes shux different from multiplexers that share a single terminal — every pane is a real terminal.

## Next Steps

- Learn about [Windows](/concepts/windows) — the container for panes
- Learn about [Recovery](/concepts/recovery) — how pane state is saved
