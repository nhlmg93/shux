# gomux Architecture: Hybrid Persistence Model

## Overview

A modern alternative to tmux's client/server architecture using:
1. **Single-process mode** when attached (UI + server in one)
2. **Fork-on-detach** to preserve running processes
3. **Snapshotting** for persistence across reboots

This combines the simplicity of single-process design with the power of true persistence.

## The Problem with tmux's Architecture

**tmux (2007):**
- Always runs as client/server
- Complex socket protocol
- Binary split (tmux vs tmux-server)
- Custom serialization format

**Why it made sense then:**
- Hardware was slower
- Process forking was expensive
- Needed to minimize overhead

**Why it's overkill now:**
- SSDs make snapshot/restore instant
- CPUs handle forking easily
- Modern serialization (JSON/gob/msgpack) is fast

## The Hybrid Approach

### Mode 1: Single-Process (Attached)

```
┌─────────────────────────────────────┐
│           gomux mysession           │
│                                     │
│  ┌──────────────┐ ┌─────────────┐ │
│  │   UI Thread   │ │ Server Thread│ │
│  │  (Bubble Tea) │ │  (PTY/Grid) │ │
│  │              │ │             │ │
│  │ Renders      │ │ Manages     │ │
│  │ user input   │ │ shells      │ │
│  │ → sends to   │ │ → updates   │ │
│  │ server       │ │ grid        │ │
│  └──────┬───────┘ └──────┬──────┘ │
│         │                │         │
│         └───────┬────────┘         │
│                 ▼                  │
│           libghostty               │
│                 ▼                  │
│      PTY → /bin/sh, vim, etc       │
└─────────────────────────────────────┘
```

**Benefits:**
- No IPC overhead
- Simple debugging (one process)
- No socket file to manage
- Atomic state access

### Mode 2: Fork-on-Detach

When user presses `Ctrl+A D` (detach):

```
Before detach:
┌──────────────────────────────┐
│         gomux                │
│    UI + Server (1 process)   │
└──────────────────────────────┘

After detach:
┌────────────┐  ┌────────────────────────────┐
│   (exit)   │  │      gomux --daemon        │
│  UI Thread │  │        (forked child)      │
│            │  │                            │
│            │  │  ┌──────────────────────┐  │
│            │  │  │  Server Thread       │  │
│            │  │  │  - Keeps PTYs open   │  │
│            │  │  │  - Shells running    │  │
│            │  │  │  - Periodic snapshot │  │
│            │  │  └──────────────────────┘  │
└────────────┘  └────────────────────────────┘

Snapshot written: ~/.local/share/gomux/mysession/snapshot.gob
```

**Benefits:**
- True persistence (shells keep running)
- Same binary, just forked
- No socket protocol needed (can use stdin/stdout of forked process)
- Clean separation when needed

### Mode 3: Snapshot Restore (After Reboot)

When computer restarts and user runs `gomux mysession`:

```
1. Check: Is daemon running?
   └─ Yes → Connect (reattach)
   └─ No  → Continue to restore

2. Load snapshot: ~/.local/share/gomux/mysession/snapshot.gob

3. Restore:
   ┌────────────────────────────────────────────┐
   │  gomux mysession (restored)                │
   │                                            │
   │  Window Layout: [Pane1][Pane2]             │
   │                                            │
   │  Pane1: Respawned shell in /home/user/proj │
   │         (new PTY, same CWD)                │
   │                                            │
   │  Pane2: Restored vim foo.txt               │
   │         (replays: vim foo.txt)             │
   │                                            │
   │  Scrollback: Replayed from disk cache      │
   │                                            │
   │  State: "Fast restore" - not true          │
   │         persistence, but feels instant     │
   └────────────────────────────────────────────┘
```

**Trade-offs:**
- Not same PIDs (new processes)
- Some state lost (running compiles, unsaved buffers)
- But: instant "attach", clean architecture

## Comparison: tmux vs gomux

| Aspect | tmux | gomux (Hybrid) |
|--------|------|----------------|
| **Normal mode** | Client/server | Single process |
| **IPC** | Custom binary protocol | Go channels (shared memory) |
| **Detach** | Client disconnects | Fork server, UI exits |
| **Reattach** | Connect socket | Connect to daemon OR restore |
| **Cross-reboot** | Automatic (server survives) | Snapshot restore |
| **Binary count** | 2 (tmux, tmux-server) | 1 (gomux) |
| **Protocol** | Custom | None (shared mem) or stdio |
| **Serialization** | Custom format | gob/msgpack/JSON |

## Snapshot Format

```go
type SessionSnapshot struct {
    Version     int
    Timestamp   time.Time
    SessionName string
    
    Windows []struct {
        ID       uint32
        Name     string
        Active   bool
        Layout   LayoutType // horizontal, vertical, etc.
        
        Panes []struct {
            ID          uint32
            Cwd         string              // Current working directory
            Env         map[string]string   // Environment variables
            Command     string              // Running command (to replay)
            Shell       string              // Shell type
            Scrollback  []string            // Last N lines (configurable)
            CursorPos   Point               // Row, Col
            // Grid state from libghostty (if serializable)
            GridState   []byte              
        }
    }
    
    GlobalState struct {
        ActiveWindow uint32
        PrefixKey    string // "Ctrl+A", etc.
    }
}
```

**Storage:**
- Primary: `~/.local/share/gomux/{session}/snapshot.gob`
- Scrollback: `~/.local/share/gomux/{session}/panes/{id}/scrollback.txt`
- Auto-snapshot: Every 30 seconds when detached

## Why This Is Simpler

### tmux Complexity
```
1. Design custom binary protocol
2. Handle client/server handshake
3. Manage socket files, permissions
4. Serialize state in custom format
5. Handle reconnection edge cases
6. Deal with TTY stealing, terminal modes
```

### gomux Simplicity
```
1. Single process: Go channels (no protocol!)
2. Detach: Just fork(), stdio for control
3. Cross-reboot: gob.Marshal() to disk
4. Restore: gob.Unmarshal(), re-spawn shells
5. Modern Go libraries handle TTY details
```

## Open Questions

1. **libghostty grid serialization:** Can we extract and restore full terminal state, or just replay scrollback?

2. **Command replay accuracy:** How to restore "vim foo.txt" correctly? Store full command line?

3. **Network transparency:** Should we support attaching from different machine (SSH)? If so, need socket protocol.

4. **State consistency:** What happens if snapshot is taken mid-command? (Acceptable inconsistency vs ACID?)

## Decision: Keep It Local-First

**For v1:**
- Local machine only (no network attach)
- Fork-on-detach for true persistence
- Snapshot for reboots
- Simple gob serialization

**Future:**
- Add socket protocol only if network attach needed
- Keep local path optimized (no unnecessary IPC)

## Trade-off Summary

**We accept:**
- Fork overhead on detach (negligible on modern hardware)
- Snapshot disk usage (~MBs)
- Not same PIDs after reboot

**We gain:**
- 90% single-process simplicity
- No custom protocol to maintain
- One binary
- Fast restore from snapshot (SSD speed)
- Debuggable (gob files are inspectable)

## Implementation Plan

1. **Phase 1:** Single-process mode (working now)
2. **Phase 2:** Fork-on-detach mechanism
3. **Phase 3:** Snapshot serialization
4. **Phase 4:** Restore on startup
5. **Phase 5:** Polish edge cases

## Files

- `AGENTS.md` - Development guidelines
- `ARCHITECTURE.md` - This document
- `pkg/gomux/session.go` - Session persistence logic
- `pkg/gomux/snapshot.go` - Snapshot serialization
- `pkg/gomux/daemon.go` - Fork/daemon mode
