---
title: Protocol
description: The internal messaging protocol of shux.
---

## Internal Protocol

shux uses an internal actor-based messaging protocol for communication between its components. This document describes the protocol for external integrations.

## Actor Model

shux follows the BEAM-inspired actor model:

```
Session Actor  ←→  Window Actor  ←→  Pane Actors
     ↑                                       ↑
     └─────────── Internal Bus ──────────────┘
```

Each component (session, window, pane) is an independent actor that communicates via message passing.

## Message Format

Messages are serialized as JSON:

```json
{
  "type": "message_type",
  "source": "actor_id",
  "target": "actor_id",
  "payload": { ... }
}
```

## Message Types

### Session Messages

| Type | Direction | Payload |
|------|-----------|---------|
| `session.create` | → shux | `{ name, width, height }` |
| `session.attach` | ← shux | `{ id, windows }` |
| `session.detach` | → shux | `{ id }` |
| `session.destroy` | → shux | `{ id }` |

### Window Messages

| Type | Direction | Payload |
|------|-----------|---------|
| `window.create` | → session | `{ name }` |
| `window.kill` | → session | `{ id }` |
| `window.rename` | → session | `{ id, name }` |

### Pane Messages

| Type | Direction | Payload |
|------|-----------|---------|
| `pane.split` | → window | `{ direction }` |
| `pane.input` | → pane | `{ data }` |
| `pane.resize` | → pane | `{ width, height }` |

## Persistence Layer

The persistence layer serializes session state to disk using a snapshot-based approach:

```
~/.local/share/shux/
└── sessions/
    └── <session-id>/
        ├── snapshot.json    # Latest state snapshot
        ├── events.log       # Event log for recovery
        └── config.lua       # Session config at snapshot time
```

## Next Steps

- Learn about [Recovery](/concepts/recovery) — how snapshots work
- Learn about [Lua Configuration](/config/lua-config) — the configuration system
