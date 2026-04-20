# shux Architecture

> you shouldn't have.

## Goal

Implement basic attach/detach using shux's single-process, disk-only persistence architecture:

- `shux <session>` = attach-or-create
- detach = snapshot session to disk, then exit process
- reattach = load snapshot, rebuild layout, respawn shells in saved CWDs

No daemon. No server/client split. No true process persistence.

---

## Core Idea

shux persists workspace state, not live processes.

Detach means:
- save session state to disk
- exit the process

Reattach means:
- load snapshot from disk
- recreate the actor tree
- respawn shells
- restore layout and active selections

This is the architecture.

---

## MVP Scope

The first working version should restore:

- window count and order
- pane count and order
- active window
- active pane
- pane rows and cols
- shell executable
- pane CWD

Optional after that:

- scrollback replay
- richer pane metadata

---

## Phase 1: Persistence primitives

### Add filesystem helpers

Create `pkg/shux/shuxdir.go`:

- `DataDir()`
- `SessionDir(name string)`
- `SessionSnapshotPath(name string)`

Use a layout like:

```text
~/.local/share/shux/<session>/snapshot.gob
```

### Add snapshot types and serialization

Create `pkg/shux/snapshot.go` with minimal types:

- `SessionSnapshot`
- `WindowSnapshot`
- `PaneSnapshot`

Basic fields:

- session name
- version
- timestamp
- window order
- active window
- pane order
- active pane
- pane rows / cols
- pane shell
- pane cwd

Functions:

- `SaveSnapshot(path string, snapshot *SessionSnapshot) error`
- `LoadSnapshot(path string) (*SessionSnapshot, error)`
- `DeleteSnapshot(path string) error`

---

## Phase 2: Capture state from actors

### Session state

Extend `pkg/shux/session.go` so it can expose or collect:

- ordered windows
- active window
- session-level shell/default shell if needed

### Window state

Extend `pkg/shux/window.go` so it can expose or collect:

- ordered panes
- active pane

### Pane state

Extend `pkg/shux/pane.go` so it can expose or collect:

- current shell
- rows / cols
- current cwd
- optional content / scrollback snapshot later

This likely needs new ask-style messages in `pkg/shux/messages.go`, for example:

- `GetSessionSnapshotData`
- `GetWindowSnapshotData`
- `GetPaneSnapshotData`

Or a single top-level session ask that recursively gathers everything.

---

## Phase 3: Track pane CWD

### Add pane CWD tracking

Create `pkg/shux/pane_cwd.go`.

For MVP, Linux-first is acceptable:

- inspect the child shell PID cwd via `/proc/<pid>/cwd`

In `Pane`:

- store latest known cwd
- refresh it when useful
  - after shell starts
  - periodically, or
  - immediately before snapshot collection

If a platform cannot support this yet, degrade cleanly.

---

## Phase 4: Restore path

### Add restore logic

Create `pkg/shux/resurrect.go`:

- `RestoreSessionFromSnapshot(...)`

Responsibilities:

- load snapshot
- create a fresh session actor tree
- recreate windows in saved order
- recreate panes in saved order
- respawn shells with saved shell and cwd
- restore active window and pane selection

This is not reattaching to old PTYs or processes.
It is rebuilding the workspace from saved context.

---

## Phase 5: Startup attach-or-create flow

### Change CLI startup

Modify `cmd/shux/main.go`.

New startup order:

1. resolve session name
2. check snapshot path on disk
3. if snapshot exists:
   - restore from disk
   - run restored session
4. else if in-process actor exists:
   - attach to it
5. else:
   - create fresh session

This preserves current in-process attach behavior while adding disk-backed attach.

---

## Phase 6: Detach flow

### Add detach message

In `pkg/shux/messages.go`, add something like:

- `DetachSession`

### Handle detach in session actor

Modify `pkg/shux/session.go`.

On detach:

1. gather snapshot data from windows and panes
2. write snapshot to disk
3. notify UI / parent if needed
4. stop the session tree and exit cleanly

This matches shux's intended behavior:

- detach saves
- process exits
- shells die with the process
- later attach respawns fresh shells from the snapshot

No disowning. No background process. No daemon behavior.

---

## Phase 7: UI keybinding

### Add detach key

Modify `pkg/shux/ui.go`.

In prefix mode, support detach:

- `Ctrl+B` then `d`

On detach key:

- send `DetachSession`
- quit the Bubble Tea program once save succeeds, or once shutdown begins

---

## Phase 8: Restore fidelity

### First restore target

The first complete version should restore:

- window count and order
- pane count and order
- active window and pane
- shell executable
- cwd
- pane size

### Defer if needed

If full terminal buffer persistence is awkward at first, defer it.

Layout + shell + cwd is more important than perfect scrollback in the first working version.

---

## Files to add

- `pkg/shux/shuxdir.go`
- `pkg/shux/snapshot.go`
- `pkg/shux/resurrect.go`
- `pkg/shux/pane_cwd.go`

## Files to modify

- `cmd/shux/main.go`
- `pkg/shux/messages.go`
- `pkg/shux/session.go`
- `pkg/shux/window.go`
- `pkg/shux/pane.go`
- `pkg/shux/ui.go`

---

## Tests to add

### Unit tests

Create `pkg/shux/snapshot_test.go`:

- snapshot round-trip
- version/path handling

Create `pkg/shux/resurrect_test.go`:

- restore session structure from snapshot
- window and pane ordering preserved
- active window and pane restored

### Integration tests

Add tests for:

- session detach saves snapshot
- attach with snapshot restores expected structure
- pane cwd survives detach/reattach
- missing or corrupt snapshot falls back to fresh session

### End-to-end test

Basic flow:

1. launch fresh session
2. create extra window and pane
3. `cd` in pane
4. detach
5. relaunch same session
6. verify restored layout and cwd

---

## Key risks and open questions

### CWD tracking

Need a reliable way to know the shell's current working directory.
Linux `/proc` is the easiest MVP path.

### Scrollback persistence

Need to decide whether to:

- serialize enough terminal state directly, or
- reconstruct from saved text later

For MVP, layout + cwd matters more than perfect buffer replay.

### Restore UX

Restore must feel fast enough that respawned shells are acceptable.
That is the core product bet.

### Snapshot schema evolution

Snapshots need a `Version` field from day one.

---

## Recommended MVP cut

Build the smallest complete loop first:

1. snapshot path helpers
2. gob snapshot structs
3. pane cwd capture
4. detach key saves snapshot and exits
5. startup attach-or-create from snapshot
6. restore windows, panes, shells, and cwds
7. tests for snapshot round-trip and restore

After that, add scrollback replay if needed.

---

## Decision

This is the architecture:

- single process
- disk-only persistence
- attach-or-create on startup
- detach saves and exits
- reattach restores from snapshot

No daemon.
No client/server split.
No phase 2 architecture change.
