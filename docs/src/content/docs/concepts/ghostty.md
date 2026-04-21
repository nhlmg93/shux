---
title: Ghostty Terminal Emulation
description: How shux uses Ghostty for real terminal emulation.
---

## Real Terminal Emulation

Unlike traditional multiplexers (tmux, screen) that emulate a terminal in software, shux uses [Ghostty](https://ghostty.org) — a modern, GPU-accelerated terminal emulator — to provide **real terminal behavior** in every pane.

## Why Ghostty?

| Feature | tmux (emulated) | shux + Ghostty (real) |
|---------|----------------|----------------------|
| Colors | Limited palette | Full 24-bit color |
| Fonts | Monospace only | System fonts, ligatures |
| Unicode | Partial | Full Unicode support |
| Graphics | None | Image protocol, OSC 8 links |
| GPU | CPU-rendered | Hardware accelerated |
| Paste | Buffer-based | Native paste handling |

## Architecture

```
┌─────────────────────────────────────┐
│           shux process              │
│  ┌───────────┐  ┌─────────────────┐ │
│  │ Actor     │  │ Actor           │ │
│  │ (Session) │  │ (Window)        │ │
│  └─────┬─────┘  └──────┬──────────┘ │
│        │                │            │
│  ┌─────┴────────────────┴───────┐   │
│  │         Pane Actors           │   │
│  │  ┌──────────┐ ┌──────────┐   │   │
│  │  │ Ghostty  │ │ Ghostty  │   │   │
│  │  │ Terminal │ │ Terminal │   │   │
│  │  └──────────┘ └──────────┘   │   │
│  └──────────────────────────────┘   │
└─────────────────────────────────────┘
```

Each pane is backed by a real Ghostty process with its own PTY, providing genuine terminal behavior.

## Benefits

1. **Accurate emulation** — what you see is exactly what a real terminal would show
2. **Modern features** — ligatures, true type fonts, image rendering
3. **Performance** — GPU-accelerated rendering
4. **Compatibility** — works correctly with all terminal applications

## Next Steps

- Learn about [Sessions](/concepts/sessions) — how Ghostty terminals are organized
- Check the [Quick Start](/quick-start) to see it in action
