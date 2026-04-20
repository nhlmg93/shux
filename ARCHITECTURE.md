# shux Architecture

> you shouldn't have.

## Overview

A modern terminal multiplexer designed to replace tmux with a simpler architecture.

**Core principle:** Disk-only persistence is fast enough on modern hardware.

No daemon. No fork. No client/server split. Just snapshot and restore.

---

## Architecture

### Single-Process Design

```
┌─────────────────────────────────────┐
│        shux mysession              │
│                                     │
│  ┌──────────────┐ ┌─────────────┐  │
│  │   UI Thread  │ │  PTY/Grid   │  │
│  │ (Bubble Tea) │ │  (libghostty)│ │
│  │              │ │             │  │
│  │ Renders      │ │ Manages     │  │
│  │ user input   │ │ shells      │  │
│  └──────────────┘ └─────────────┘  │
│                                     │
│  Single process - no fork, no IPC  │
└─────────────────────────────────────┘
```

**Lifecycle:**

```
Start:    shux mysession
          └─> If snapshot exists: restore
          └─> If no snapshot: create fresh session

Work:     (use terminal normally)
          ├─> Layout changes? Auto-snapshot every 30s
          ├─> Scrollback accumulates in memory
          └─> CWD tracked per pane

Detach:   Ctrl+A D
          └─> Serialize state to disk
          └─> Exit process

Reattach: shux mysession
          └─> Load snapshot from disk
          └─> Re-spawn shells in saved CWDs
          └─> Restore window layout
          └─> Replay scrollback
```

### State Storage

```
~/.local/share/shux/
├── mysession/
│   ├── snapshot.gob          # Session structure
│   ├── panes/
│   │   ├── 1/
│   │   │   ├── scrollback.txt
│   │   │   └── history       # Shell history (optional)
│   │   └── 2/
│   │       └── scrollback.txt
│   └── last-active          # Timestamp
└── globalsession/
    └── ...
```

### Snapshot Format

```go
type SessionSnapshot struct {
    Version     int
    Timestamp   time.Time
    SessionName string
    
    Windows []struct {
        ID       uint32
        Name     string
        Active   bool
        Layout   LayoutType
        
        Panes []struct {
            ID          uint32
            Cwd         string
            Env         map[string]string
            Scrollback  []string
        }
    }
    
    GlobalState struct {
        ActiveWindow uint32
        PrefixKey    string
    }
}
```

---

## Why This Works

### Performance Reality

| Operation | Time |
|-----------|------|
| Disk read (SSD) | ~0.1 ms |
| Disk read (HDD) | ~1 ms |
| Shell spawn | ~50-100 ms |
| Full restore (4 panes) | ~200-400 ms |

**Total reattach time:** Sub-second even on HDD.

### User Experience

**tmux detach/reattach:**
- Instant (< 10ms)
- Same processes (true persistence)
- Complex daemon architecture

**shux detach/reattach:**
- Fast (~200ms)
- New processes (restart shells)
- Simple single-process architecture

**Trade-off:** 200ms vs 10ms for dramatically simpler code.

### Why Not Daemon?

**Daemon complexity (rejected):**
```
1. fork() on detach
2. Process management (zombies, signals)
3. IPC protocol (UI to daemon)
4. PID files, reconnection logic
5. Platform differences (Unix vs Windows)
```

**Disk-only simplicity (accepted):**
```
1. gob.Marshal() on detach
2. gob.Unmarshal() on attach
3. Re-spawn shells
4. Done
```

---

## Comparison: shux vs tmux

| Feature | tmux | shux |
|---------|------|-------|
| **Architecture** | Client/server daemon | Single process |
| **Reattach speed** | Instant (< 10ms) | Fast (~200ms) |
| **Process persistence** | True (same PIDs) | Restart (new PIDs) |
| **Code complexity** | High | Low |
| **Binary count** | 2 (tmux, tmux-server) | 1 |
| **IPC protocol** | Custom binary | None |
| **Cross-platform** | Unix only | All platforms |
| **Debuggability** | Hard (daemon issues) | Easy (one process) |

---

## Implementation

### Files

- **pkg/shux/snapshot.go** - Serialize/deserialize SessionSnapshot
- **pkg/shux/resurrect.go** - Restore session from snapshot
- **pkg/shux/session.go** - Session lifecycle, auto-save timer

### Key Behaviors

1. **Auto-save:** Every 30 seconds when session active
2. **On detach:** Immediate save before exit
3. **On attach:** Check for snapshot, restore if exists
4. **Graceful degradation:** If restore fails, start fresh

### Platform Notes

- **Linux:** Native support
- **macOS:** Native support  
- **Windows:** Via WSL2 (Linux environment)
- **Future:** Could add native Windows if needed

---

## Philosophy

**Hardware is fast. Complexity is expensive. Ship simple.**

Modern SSDs make disk-only feel instant. We trade 200ms for:
- Debuggable code
- Single binary
- Cross-platform
- No daemon headaches

If 200ms reattach is too slow, we made a mistake. But we think it's fast enough.

---

## Migration from tmux

| tmux | shux |
|------|-------|
| `tmux new -s foo` | `shux foo` |
| `tmux attach -t foo` | `shux foo` (same command) |
| `Ctrl+B D` | `Ctrl+A D` |
| `tmux ls` | `shux list` |

**Key difference:** Detach stops processes. Reattach restarts them (fast, but not instant).

---

## Files

- `AGENTS.md` - Development guidelines
- `ARCHITECTURE.md` - This document
- `ROADMAP.md` - Development roadmap
- `pkg/shux/snapshot.go` - Snapshot implementation
- `pkg/shux/resurrect.go` - Session restore
- `pkg/shux/session.go` - Session management
