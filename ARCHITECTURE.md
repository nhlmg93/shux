# gomux Architecture

## Overview

A modern terminal multiplexer designed to replace tmux with a simpler architecture while maintaining full functionality.

**Key principles:**
- Single-process by default (simple, debuggable)
- Add daemon/fork complexity only when needed
- Modern serialization (gob/msgpack) vs custom protocols
- Cross-platform from day one

---

## Development Phases

### Phase 1: Disk-Only Persistence (Current)

**Hypothesis:** Modern SSDs are fast enough that snapshot/restore feels instant without needing a daemon.

**Architecture:**
```
┌─────────────────────────────────────┐
│        gomux mysession              │
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

Detach (Ctrl+A D):
  └─> Serialize state to disk
  └─> Exit process (everything stops)

Reattach:
  └─> Load snapshot from disk
  └─> Re-spawn shells
  └─> Restore layout
  └─> Replay scrollback
```

**State storage:**
```
~/.local/share/gomux/mysession/
├── snapshot.gob          # Session structure (layout, CWDs, env)
├── scrollback/          # Pane scrollback buffers
│   ├── pane-1.txt
│   └── pane-2.txt
└── last-active          # Timestamp of last detach
```

**Performance target:** < 500ms restore on SSD

**Trade-offs:**
- ✅ Simple code (no fork, no process management)
- ✅ Debuggable (one process, one stack trace)
- ❌ Processes restart on reattach (new PIDs)
- ❌ No true persistence (compiles/editors restart)
- ❌ Slower than daemon mode (disk read vs instant)

**Decision point:** If Phase 1 feels fast enough, we stop here. Ship it.

---

### Phase 2: Fork-on-Detach (When Phase 1 isn't enough)

**Add forking when we need true persistence of running processes.**

**Architecture:**
```
Normal operation (attached):
┌─────────────────────────────────────┐
│        gomux mysession              │
│                                     │
│  Single process (UI + libghostty)  │
│                                     │
│  Same as Phase 1                    │
└─────────────────────────────────────┘

Detach (Ctrl+A D):
  ┌─> fork() syscall
  │    ┌─────────────────────────────┐
  │    │ Parent (original)         │
  │    │ - Writes snapshot to disk │
  │    │ - Exits UI                │
  │    └─────────────────────────────┘
  │
  └─> Child becomes daemon
       ┌─────────────────────────────┐
       │ gomux --daemon mysession    │
       │ - Keeps PTYs open           │
       │ - Shells keep running       │
       │ - Periodic snapshots        │
       │ - Writes PID to file        │
       └─────────────────────────────┘

Reattach:
  Check: ~/.local/share/gomux/mysession/daemon.pid
  
  ├─ If daemon running:
  │   └─> Connect to daemon (instant, < 10ms)
  │   └─> True persistence (same PIDs, same processes)
  │
  └─ If daemon dead:
      └─> Phase 1 restore from disk (fallback)
```

**Key components:**
- **PID file:** `~/.local/share/gomux/mysession/daemon.pid`
- **Control mechanism:** stdin/stdout or Unix socket/named pipe
- **Handshake:** UI signals daemon, transfers terminal control

**Trade-offs:**
- ✅ True persistence (processes keep running)
- ✅ Instant reattach (no disk read)
- ❌ More complex (fork, process management, IPC)
- ❌ Platform differences (Unix fork vs Windows?)
- ❌ Zombie processes if daemon crashes

**Decision trigger:** Implement only if Phase 1 restore feels too slow (> 1s) or users need process persistence.

---

### Phase 3: Network Attach (Future)

**Attach from different machines (SSH).**

**Architecture:**
```
Server:   gomux --server --bind :1234 mysession
          └─> Listens on TCP or Unix socket
          └─> Runs in background (could be systemd service)

Client:   gomux --attach host:1234 mysession
          └─> Connects via simple JSON protocol
          └─> Renders remote session locally
```

**Complexity added:**
- Authentication (who can attach?)
- Encryption (TLS for TCP)
- Protocol design (JSON over socket)
- Network failures (reconnection logic)

**Decision:** Only if users want remote attach. Local-first for v1.

---

## Comparison Matrix

| Feature | tmux | gomux Phase 1 | gomux Phase 2 | gomux Phase 3 |
|---------|------|---------------|---------------|---------------|
| **Architecture** | Always daemon | Single process | Hybrid (fork on detach) | Client/server |
| **Reattach speed** | Instant | ~100-500ms | Instant if daemon | Network latency |
| **True persistence** | ✅ Yes | ❌ No | ✅ Yes | ✅ Yes |
| **Cross-reboot** | ✅ Yes | ✅ Restore | ✅ Restore | ✅ Restore |
| **Complexity** | High | Low | Medium | High |
| **Binary count** | 2 | 1 | 1 | 2 (opt) |
| **IPC protocol** | Custom binary | None | stdio/socket | JSON/TCP |
| **Process model** | Daemon always | Single process | Fork on detach | Separate server |

---

## Design Decisions

### 1. Fork vs Thread

**Q:** Why fork instead of thread for daemon?

**A:** Fork creates true process separation:
- Daemon survives parent exit (UI can close)
- OS-level isolation
- Can re-parent to init (true daemon)
- Threads die with process (no persistence)

### 2. Snapshot Format

**Q:** Why gob/msgpack instead of JSON?

**A:** 
- gob: Fast, native Go, type-safe
- msgpack: Compact, cross-language
- JSON: Human-readable but verbose
- tmux: Custom binary (hard to debug)

**Choice:** gob for Phase 1 (simple, fast), msgpack if we need Phase 3 (cross-language).

### 3. libghostty Serialization

**Q:** Can we save full terminal state or just scrollback?

**A:** To be determined:
- Ideal: Serialize libghostty Grid state, restore exactly
- Fallback: Save scrollback + CWD + command, replay on restore
- Research: Check if libghostty exposes serialization API

### 4. Why not always daemon?

**Q:** Why not just always run as daemon like tmux?

**A:**
- 90% of time you're attached (waste to have daemon+client)
- Single process is simpler (no IPC, no protocol)
- Fork-on-detach is lazy optimization (only pay when needed)
- Modern SSDs make disk restore "fast enough"

---

## Implementation Notes

### Phase 1 Files

- `pkg/gomux/snapshot.go` - Serialize/deserialize SessionSnapshot
- `pkg/gomux/resurrect.go` - Restore session from snapshot
- `pkg/gomux/session.go` - Session lifecycle (attach/detach/save)

### Phase 2 Files

- `pkg/gomux/fork.go` - Platform-specific fork (Unix syscall, Windows CreateProcess?)
- `pkg/gomux/daemon.go` - Daemon mode (minimal, no UI)
- `pkg/gomux/attach.go` - Connect to running daemon
- `pkg/gomux/pidfile.go` - PID file management (check, write, cleanup)

### Cross-Platform Concerns

**Unix (Linux/macOS):**
- `fork()` syscall works
- Unix domain sockets for IPC
- Signals for control (SIGUSR1, etc.)

**Windows:**
- No `fork()` - use `CreateProcess` with inheritance
- Named pipes instead of Unix sockets
- Different process model

**Strategy:** Start Unix-only, add Windows later (most tmux users are on Unix anyway).

---

## Migration from tmux

**For tmux users switching to gomux:**

| tmux habit | gomux equivalent |
|------------|------------------|
| `tmux new -s foo` | `gomux foo` (creates or attaches) |
| `tmux attach -t foo` | `gomux foo` (same command) |
| `Ctrl+B D` (detach) | `Ctrl+A D` (detach, Phase 1: save, Phase 2: fork) |
| `tmux ls` | `gomux list` |
| `tmux kill-session -t foo` | `gomux kill foo` |

**Key difference:** In Phase 1, detach means processes stop. In Phase 2, they keep running.

---

## Open Questions

1. **Command replay:** How accurate can we restore "vim foo.txt"? Store exact command line? Replay keystrokes?

2. **State consistency:** Snapshot mid-command = inconsistent state. Acceptable? ACID transactions? Last-known-good?

3. **Scrollback limits:** Store all scrollback? Truncate? Configurable per-pane?

4. **Compression:** Compress snapshots? gzip? Trade disk space vs CPU.

5. **Encryption:** Encrypt snapshots at rest? Contains shell history, env vars (may have secrets).

---

## Success Metrics

**Phase 1 success:**
- Attach < 500ms
- Detach < 100ms  
- Works for daily shell use
- No user complaints about speed

**Phase 2 trigger:**
- User complaints about slow restore
- Feature requests for "keep my vim open"
- Long-running compile jobs restarting

**Phase 3 trigger:**
- "Can I attach from my phone?"
- "I want to share my session with teammate"
- Remote development workflows

---

## Philosophy

**Start simple, add complexity only when measured need.**

Don't build Phase 2 until Phase 1 proves insufficient.
Don't build Phase 3 until users ask for it.

Hardware is fast. Users are patient. Complexity is expensive.

Ship Phase 1 first.
