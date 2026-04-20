# shux Development Roadmap

> you shouldn't have.

## Phase 1: Disk-Only Persistence (Current)

**The only phase.** Modern hardware makes disk-only fast enough. No daemon, no fork, no complexity.

### Implementation

```
Attach:   shux mysession
          └─> Load snapshot from disk
          └─> Restore layout
          └─> Re-spawn shells (new PTYs)
          └─> Replay scrollback
          
Work:     (use terminal normally)
          ├─> Auto-save every 30 seconds
          └─> Scrollback accumulates

Detach:   Ctrl+A D
          └─> Snapshot current state to disk
          └─> Exit process

Reattach: shux mysession
          └─> Same as "Attach" above
          └─> If snapshot exists: restore
          └─> If no snapshot: fresh session
```

### Files to Create

1. **pkg/shux/snapshot.go**
   - Define SessionSnapshot struct
   - Serialize to gob format
   - Write to ~/.local/share/shux/{session}/
   - Auto-save timer (30 second intervals)

2. **pkg/shux/resurrect.go**
   - Restore session from snapshot
   - Re-spawn shells in saved CWDs
   - Restore window layout
   - Replay scrollback from disk

3. **pkg/shux/session.go**
   - Session lifecycle management
   - Coordinate snapshot/restore
   - Handle "attach or create" logic

### Open Questions

1. **libghostty serialization:** Can we extract full terminal state, or just scrollback?
   - Research: Check go-libghostty API for grid serialization
   - Fallback: Save scrollback + CWD + command replay

2. **Command replay:** How to restore "vim foo.txt" correctly?
   - Option A: Store full command line, re-run on restore
   - Option B: Just restore CWD, user re-runs command
   - Start with Option B (simpler), add A if needed

3. **Pane identification:** How to map saved pane to new PTY?
   - Use numeric IDs (pane 1, pane 2)
   - Stable across sessions

### MVP Scope

**Phase 1 MVP includes:**
- Save: window layout, pane CWDs, scrollback
- Restore: layout, cd to CWD, show scrollback
- Auto-save every 30s
- Manual save on detach

**Out of scope:**
- True process persistence (daemon mode)
- Command replay
- Network attach
- Encryption of snapshots

### Success Criteria

- [ ] Attach/detach < 500ms on SSD
- [ ] Attach/detach < 1s on HDD  
- [ ] Scrollback preserved
- [ ] Window layout preserved
- [ ] CWD/environment preserved
- [ ] Feels "fast enough" for daily use

### Build/Test Cycle

```bash
# Build
make

# Start fresh session
./shux mysession

# Work in it, create panes, cd around

# Detach
Ctrl+A D

# Check snapshot exists
ls ~/.local/share/shux/mysession/

# Reattach (measure time)
time ./shux mysession

# Feel: Is it fast enough?
```

### Decision: This Is The Architecture

**No Phase 2. No daemon. No fork.**

Disk-only is fast enough. Hardware solved the problem we thought we had.

Ship Phase 1. Done.

---

## Notes

**Why no daemon:**
- 200ms vs 10ms is acceptable
- Code is 10x simpler
- Single binary, no installation
- No process management headaches
- Cross-platform by default

**If users complain about speed:**
They're wrong or we have a bug. Measure first, optimize later.

**If users need true persistence:**
Use tmux. Or teach them that 200ms is fine.

**Philosophy:**
Ship simple. Hardware is fast. Users adapt.
