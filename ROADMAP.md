# gomux Development Roadmap

## Phase 1: Disk-Only Persistence (Current Goal)

**Hypothesis:** Modern SSDs are fast enough that snapshot/restore feels instant. Maybe we don't need daemon fork-on-detach at all.

**Implementation:**
```
Attach:   gomux mysession
          └─> Load snapshot
          └─> Restore layout
          └─> Re-spawn shells (new PTYs)
          └─> Replay scrollback
          
Work:     (use terminal normally)

Detach:   Ctrl+A D
          └─> Snapshot current state to disk
          └─> Exit process

Reattach: gomux mysession
          └─> Same as "Attach" above
          └─> If snapshot exists: restore
          └─> If no snapshot: fresh session
```

**Key difference from tmux:**
- tmux: Detach leaves daemon running (fast reattach)
- gomux P1: Detach just saves to disk (slower reattach, but simpler)

**Success criteria:**
- [ ] Attach/detach feels fast enough (< 500ms on SSD)
- [ ] Scrollback preserved
- [ ] Window layout preserved
- [ ] CWD/environment preserved
- [ ] Command replay works (vim foo.txt → restores to vim)

**Questions to answer:**
1. Is 500ms acceptable for reattach?
2. Do we miss true persistence (running processes)?
3. Is complexity of daemon worth it?

---

## Phase 2: Hot/Cold Sessions (If Phase 1 is too slow)

**Add daemon only if Phase 1 feels too slow:**

```
Cold start:  gomux mysession
             └─> No daemon running
             └─> Load snapshot from disk (slow)
             
Hot attach:  gomux mysession
             └─> Daemon running
             └─> Connect instantly (fast)
             
Detach:      Ctrl+A D
             └─> Fork daemon
             └─> Keep PTYs open
             └─> Also write snapshot (for safety)
```

**This is the hybrid model from ARCHITECTURE.md.**

---

## Phase 3: Network Attach (Future)

**Only if needed:**
- Attach from different machine (SSH)
- Would require socket protocol
- Keep local path optimized

---

## Current Focus: Phase 1

### Files to create/modify:

1. **pkg/gomux/snapshot.go**
   - Define SessionSnapshot struct
   - Serialize/deserialize (gob or msgpack)
   - Write to ~/.local/share/gomux/{session}/

2. **pkg/gomux/resurrect.go**
   - Restore session from snapshot
   - Re-spawn shells in correct CWD
   - Restore window layout
   - Replay scrollback

3. **Modify pkg/gomux/term.go**
   - On detach: write snapshot
   - On spawn: check for existing snapshot

4. **Modify pkg/gomux/session_actor.go**
   - Session-level snapshot coordination

### Open questions for Phase 1:

1. **libghostty serialization:** Can we save/restore full terminal state, or just replay scrollback?

2. **Command replay:** How to restore "vim foo.txt" correctly?
   - Option A: Store full command line, replay on restore
   - Option B: Just restore CWD, let user re-run command
   - Option C: tmux-style "save running command, re-exec on restore"

3. **Pane identification:** How to map saved pane to new PTY?

### Decision: Start Simple

**Phase 1 MVP:**
- Save: window layout, pane CWDs, scrollback
- Restore: layout, cd to CWD, show scrollback
- Don't try to restore exact process state yet
- See how it feels

**If good enough:** Ship it!
**If too slow/missing features:** Add daemon in Phase 2

---

## Build/Test Cycle

```bash
# Build
make

# Start fresh session
./gomux mysession

# Work, detach
Ctrl+A D

# Check snapshot exists
ls ~/.local/share/gomux/mysession/

# Reattach (measure time)
time ./gomux mysession

# Feel: Is it fast enough?
```

---

## Notes

**Why start with disk-only:**
- Simpler code (no fork, no daemon management)
- Test core concepts first
- Can always add daemon later
- Maybe SSDs are fast enough!

**Why fork-on-detach might still be needed:**
- Long-running compiles (don't want to restart)
- Unsaved work in editors
- Database connections, SSH sessions, etc.

**But:** Let's measure first, then decide.
