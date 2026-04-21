---
title: Recovery & Persistence
description: How shux saves and restores session state.
---

## The Recovery Model

shux's defining feature is its **disk-backed recovery model**. Every time you detach (or at regular intervals during operation), the entire session state is snapshotted to disk.

## How It Works

```
Memory State
    │
    ▼ (snapshot on detach / periodic)
Disk Snapshot (.shux directory)
    │
    ▼ (restore on reattach)
Memory State (restored)
```

## What Gets Saved

- Session name and ID
- All windows and their layout
- All panes and their processes
- Terminal content (scrollback buffer)
- Cursor positions
- Keymaps and configuration at time of snapshot

## Crash Recovery

If shux crashes unexpectedly:

1. The last disk snapshot is preserved
2. On next launch, shux can restore from that snapshot
3. You pick up as close to your last working state as possible

This is different from tmux, which relies on in-memory state only — a tmux process crash means lost session data.

## Durability Design

shux uses **tiger-style programming** for its durability layer:

- **Boundedness** — state transitions are explicit and auditable
- **Assertions** — every state assumption is checked
- **Static Analysis** — the Go compiler catches type errors before runtime
- **Deterministic simulation** — the persistence layer can be tested deterministically

## Next Steps

- Learn about [Ghostty](/concepts/ghostty) — the terminal emulation layer
- Check the [Reference](/reference/protocol) for the internal protocol
