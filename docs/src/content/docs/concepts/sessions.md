---
title: Sessions
description: Understanding shux sessions.
---

## What is a Session?

A session is the top-level unit of shux. It represents a complete terminal workspace with its own set of windows, panes, and state.

## Session Lifecycle

```
create → attach → work → detach → (survives!) → reattach
```

Sessions are **disk-backed**. When you detach, the entire state is snapshotted to disk. When you reattach, the session is restored as close to its original state as possible.

## Creating Sessions

```bash
./shux new -s mysession    # Create and attach
./shux new -s mysession    # If it exists, attaches to it
```

Sessions are identified by name. You can have multiple sessions running simultaneously.

## Managing Sessions

```bash
./shux list               # List all sessions
./shux attach -s name     # Attach to a session
./shux kill -s name       # Kill a session (destroys state)
```

## Detach & Reattach

Detaching saves all state to disk. Reattaching restores it. This is the core of shux's reliability model — your work survives process crashes, network drops, and reboots.

## Next Steps

- Learn about [Windows](/concepts/windows) — the workspace containers
- Learn about [Recovery](/concepts/recovery) — how shux persists and restores state
