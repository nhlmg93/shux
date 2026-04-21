# Shux durable persistence and supervision design proposal

## Status

Proposed

## Summary

Move shux toward a clean split between:

1. **Live continuity**
   - owner process keeps PTYs alive
   - clients are disposable
   - detach means disconnect UI only
   - reattach reconnects to the same live owner over a per-session socket

2. **Durable fallback**
   - if the owner dies or the machine reboots, restore from durable snapshot
   - optionally replay saved VT/screen state so restore feels continuous
   - this restores structure and visual context, not arbitrary live process memory

This keeps the tmux-like fast path for same-machine reattach while preserving shux's snapshot-first philosophy for cold recovery.

---

## Goals

- Preserve real shell/app continuity across detach/reattach on the same running machine
- Make pane, window, and session controller crashes recoverable without killing child PTYs
- Separate **live irreplaceable state** from **reconstructible orchestration state**
- Improve cold restore UX with durable screen/playback state
- Keep the architecture compatible with shux's single-process owner model for v1/v2

---

## Non-goals

- Preserving arbitrary running process state across owner crash in v1/v2
- Preserving arbitrary running process state across machine reboot
- Adding a global tmux-style daemon
- Adding a PTY keeper subprocess in the initial implementation

Note: true continuity across owner-process death requires PTY ownership to survive outside the crashed process. That is a possible later phase, not part of this proposal's core v1/v2 path.

---

## Core model

### Live continuity

Detach/reattach on the same machine should work like this:

1. Owner process starts and owns session PTYs
2. UI client connects to owner via per-session Unix socket
3. Detach disconnects only the client
4. Later attach reconnects to the same owner
5. Running shells/apps continue uninterrupted

### Durable fallback

If the owner is gone or the machine rebooted:

1. Load durable snapshot
2. Restore session/window/pane topology
3. Restore saved screen/playback state if available
4. Start replacement shells or reconnect external workloads where possible

This restores:
- windows/panes/layout
- cwd
- shell command
- active window/pane
- last known visible content

This does **not** restore:
- live interactive process memory state
- shell job tables
- arbitrary in-memory app state such as vim unless the app persisted it itself

---

## Design principles

### 1. State gravity downward

State should live at the lowest layer that makes sense:

- **UI client**: presentation only, disposable
- **Session controller**: navigation/orchestration
- **Window controller**: layout/composition
- **Pane controller**: pane-local coordination
- **Pane runtime**: PTY, child process, VT/playback state, valuable live state
- **Durable store**: snapshot/playback fallback

The lower the layer, the more durable it should be.

### 2. Parent crash must not destroy child runtime ownership

This is the key recovery rule.

If a pane controller crashes, the PTY must survive.
If a window controller crashes, its pane runtimes must survive.
If a session controller crashes, its windows/pane runtimes must survive.

Only explicit kill paths should destroy child runtimes.

### 3. Separate live irreplaceable state from reconstructible orchestration state

#### Live irreplaceable state
- PTY master FD
- child process
- VT state / replayable output

#### Reconstructible orchestration state
- layout tree
- active pane/window
- focus
- subscriptions
- UI bindings
- mouse/resize controller state

If these are separated cleanly, controller crashes become restart/recomposition events instead of continuity loss.

---

## Target supervision tree

Inside one session owner process:

```text
SessionOwner
├─ IPCServer
│  └─ attached client(s)
├─ SessionSupervisor
│  ├─ SessionController
│  │  ├─ WindowController[1..N]
│  │  │  ├─ PaneController[1..M]
│  │  │  └─ layout/focus/mouse state
│  │  └─ session-wide navigation state
│  └─ RuntimeRegistry
│     ├─ PaneRuntime[stable]
│     ├─ PaneScreenState[stable-ish]
│     └─ PlaybackBuffer[bounded]
└─ SnapshotManager
   ├─ live metadata
   ├─ structural snapshots
   └─ visual playback snapshots
```

### Restartability by layer

#### Restartable
- UI client
- pane controller
- window controller
- session controller

#### Must not be destroyed by controller panic
- pane runtime
- PTY master
- child process
- VT/playback state

---

## Component model

## PaneRuntime

Owns real process continuity.

Responsibilities:
- PTY master FD
- child process PID / wait handling
- read/write IO pumps
- current terminal size
- current cwd/pid metadata where observable
- bounded VT byte ring and/or last rendered screen state
- optionally Ghostty terminal state directly, or enough replay state to rebuild it

Properties:
- stable identity by pane ID
- survives pane/window/session controller restarts
- destroyed only by explicit kill or actual child exit

## PaneController

Owns pane coordination, not pane life.

Responsibilities:
- input routing to PaneRuntime
- asks/replies for pane content
- subscriptions
- focus glue
- interaction with parent window
- translating runtime state into pane-facing API

Properties:
- restartable
- if it panics, restart around the same PaneRuntime

## WindowController

Owns composition only.

Responsibilities:
- pane membership
- split tree/layout
- active pane
- resize distribution
- mouse hit testing
- divider drag state

Properties:
- does not own PTY lifetime
- restartable from pane IDs + layout snapshot + active pane + size

## SessionController

Owns session orchestration only.

Responsibilities:
- window order
- active window
- session-wide commands
- subscribers
- detach/kill semantics

Properties:
- restartable from live registry + structural state
- does not implicitly destroy windows/panes on crash

## SnapshotManager

Owns durability.

Responsibilities:
- write live owner metadata
- write structural snapshots
- write pane visual recovery state
- load/validate stale vs live state
- cold restore path

---

## Lifecycle rules

These rules should become explicit and enforced.

### Attach
- validate live owner metadata
- connect to socket
- subscribe for updates
- render current state immediately

### Detach
- disconnect client only
- owner remains alive
- PTYs remain alive
- update durable snapshot and live metadata
- no session teardown

### Client crash
- treated like ungraceful detach
- owner remains alive
- session unaffected

### Pane controller panic
- do not close PTY
- do not kill child process
- restart pane controller around same runtime
- rebind subscriptions
- refresh UI

### Window controller panic
- do not kill pane runtimes
- rebuild window from pane membership + layout + active pane + size
- refresh UI

### Session controller panic
- do not kill windows/panes
- rebuild session from registry + last structural state
- refresh UI

### Pane child exits normally
- pane runtime is genuinely dead
- remove pane from window
- if last pane exits, window becomes empty
- persist updated structure

### Explicit kill-pane
- kill child process / close PTY
- destroy pane runtime
- remove pane from topology
- persist snapshot

### Explicit kill-window
- explicitly kill contained pane runtimes
- remove window from session
- persist snapshot

### Explicit kill-session
- explicitly kill all pane runtimes
- stop owner
- clear live metadata
- persist final dead snapshot

### Owner crash / panic
- live continuity is lost in the v1/v2 single-process design
- next start falls back to durable snapshot + playback restore

### Machine reboot
- same as owner death, but colder
- restore structure + playback only

---

## Persistence model

Split persistence into three layers.

## A. Live metadata

Purpose: fast live-owner discovery.

Suggested fields:
- session name
- owner PID
- owner start time
- socket path
- attach token if needed
- generation/version
- last update timestamp

Used for:
- live attach
- stale owner detection

## B. Structural snapshot

Purpose: restore topology.

Suggested fields:
- session ID/name
- active window
- window order
- windows
- panes
- split/layout tree
- pane shell/cwd/rows/cols
- active pane per window
- future restore hints

Used for:
- cold restore after owner death/reboot
- controller rebuild fallback if needed

## C. Visual recovery state

Purpose: restore visual continuity.

Per pane suggested fields:
- last rendered screen content
- cursor position/visibility
- title
- alt-screen flag
- bell count if useful
- optional bounded scrollback snapshot
- optional bounded VT byte ring / replay log
- last update timestamp

Used for:
- better resurrect UX
- immediate view before new process output arrives

---

## Snapshot schema direction

Current `SessionSnapshot` is too monolithic for the target architecture. Move toward:

### `LiveSessionRecord`
```go
// conceptual only
 type LiveSessionRecord struct {
     Version        int
     SessionName    string
     OwnerPID       int
     OwnerStartTime uint64
     SocketPath     string
     Generation     uint64
     UpdatedAtUnix  int64
 }
```

### `SessionStateSnapshot`
```go
// conceptual only
 type SessionStateSnapshot struct {
     Version      int
     SessionName  string
     ID           uint32
     Shell        string
     ActiveWindow uint32
     WindowOrder  []uint32
     Windows      []WindowStateSnapshot
 }
```

### `PaneVisualSnapshot`
```go
// conceptual only
 type PaneVisualSnapshot struct {
     Version        int
     PaneID         uint32
     Title          string
     CursorRow      int
     CursorCol      int
     CursorVisible  bool
     InAltScreen    bool
     Lines          []string
     UpdatedAtUnix  int64
     // later: cells, scrollback, vt bytes
 }
```

Migration can keep the existing gob format initially, but the design should move toward these separate concerns.

---

## Recovery contracts

These contracts make recovery composable.

## Pane rebuild contract

Given:
- pane ID
- runtime handle
- screen/playback state
- size metadata

Create a fresh `PaneController` without touching the underlying runtime.

## Window rebuild contract

Given:
- window ID
- pane membership
- split tree/layout
- active pane ID
- rows/cols

Create a fresh `WindowController` without killing/recreating panes.

## Session rebuild contract

Given:
- session name/ID
- window order
- active window
- window controller references

Create a fresh `SessionController` without killing/recreating windows.

---

## Recovery algorithms

## 1. Live attach path

1. Load live metadata
2. Validate PID exists
3. Validate PID start time matches
4. Validate socket is reachable
5. If valid, attach to owner
6. Owner sends current state immediately
7. Client subscribes for incremental updates

## 2. In-owner controller healing path

1. Supervisor observes panic in pane/window/session controller
2. Supervisor classifies crash scope
3. Look up live children in runtime/controller registry
4. Rebuild crashed controller from registry + structural state
5. Rebind subscriptions
6. Trigger UI refresh

## 3. Cold restore path

1. No valid live owner found
2. Load structural snapshot
3. Load pane visual snapshots
4. Recreate owner process state
5. Recreate windows/panes/layout
6. Show saved visual content immediately
7. Start replacement shells or reconnect workloads
8. UI attaches normally

---

## VT/playback strategy

Ghostty playback is valuable for visual continuity, but should not be treated as true process continuity.

Recommended approach:

### Save both:
- normalized last-rendered screen snapshot
- optional bounded VT byte ring for replay

### Restore preference order:
1. live runtime if available
2. VT replay if available
3. normalized saved screen snapshot
4. blank shell as last resort

This gives good UX without overcommitting to library-specific serialization.

---

## Suggested on-disk layout

```text
~/.local/share/shux/sessions/<name>/
  live.gob
  state.gob
  panes/
    <pane-id>.screen.gob
    <pane-id>.vtlog   # optional later
```

Initial implementation can stop at:
- `live.gob`
- `state.gob`
- `panes/<id>.screen.gob`

---

## Migration plan

## Phase 0: stabilize live attach foundation

Before deeper supervision changes, fix live owner/client correctness.

Work:
- fix IPC request/reply multiplexing with correlation IDs
- ensure initial view delivery is reliable
- ensure fresh remote sessions can create initial windows
- remove startup races around owner creation/attach
- make live owner metadata updates correct and atomic

Files likely involved:
- `cmd/shux/main.go`
- `pkg/shux/ipc.go`
- `pkg/shux/ipc_types.go`
- `pkg/shux/remote_session.go`
- `pkg/shux/live_session.go`

Exit criteria:
- attach-live path is reliable
- detach leaves owner alive
- reattach preserves running shells/apps

## Phase 1: split runtime from controller ownership

Refactor pane lifecycle first.

Work:
- introduce `PaneRuntime`
- introduce `PaneController`
- move PTY/process ownership into runtime
- make pane controller restartable
- stop controller panic cleanup from closing PTY by default

Files likely added:
- `pkg/shux/pane_runtime.go`
- `pkg/shux/pane_controller.go`

Files likely changed:
- `pkg/shux/pane.go`
- `pkg/shux/window.go`

Exit criteria:
- pane controller panic does not kill shell

## Phase 2: stop downward crash cascades

Work:
- make `WindowController` crash non-destructive to pane runtimes
- make `SessionController` crash non-destructive to windows/panes
- separate explicit kill from controller stop

Likely changes:
- replace overloaded `Stop/Shutdown` semantics with explicit paths
  - controller stop
  - runtime kill
  - session kill

Files likely changed:
- `pkg/shux/window.go`
- `pkg/shux/session.go`
- `pkg/shux/loop.go`
- message/action definitions

Exit criteria:
- window controller panic does not kill panes
- session controller panic does not kill windows

## Phase 3: add supervision and registries

Work:
- add supervisor tree inside owner
- add stable registries for pane runtimes and controllers
- add one-for-one restart policy for controllers
- add panic reporting with restart context

Files likely added:
- `pkg/shux/supervisor.go`
- `pkg/shux/registry.go`
- `pkg/shux/restart_policy.go`

Exit criteria:
- injected controller panics self-heal in-process

## Phase 4: explicit rebuild contracts

Work:
- define window rebuild contract from pane membership + layout
- define session rebuild contract from window registry + active state
- ensure stable IDs are preserved
- persist minimal orchestration state needed for reconstruction

Files likely changed:
- `pkg/shux/window.go`
- `pkg/shux/session.go`
- snapshot/state types

Exit criteria:
- deliberate window-controller panic recovers with shells intact
- deliberate session-controller panic recovers with windows intact

## Phase 5: durable visual playback

Work:
- persist pane visual state separately from structure
- optionally add bounded VT log persistence
- load playback state during cold restore
- render saved state before new process output arrives

Files likely added:
- `pkg/shux/playback.go`
- `pkg/shux/store.go`

Files likely changed:
- `pkg/shux/snapshot.go`
- `pkg/shux/resurrect.go`
- pane rendering code

Exit criteria:
- owner death/reboot restore shows prior visible content

## Phase 6: recovery manager polish

Work:
- unify attach-live, in-owner healing, and cold restore state machine
- improve logging/observability by recovery cause
- document semantics clearly

Recovery causes to classify:
- client detach
- client disconnect
- pane controller panic
- window controller panic
- session controller panic
- pane child exit
- owner death
- stale live metadata

Exit criteria:
- recovery behavior is deterministic and observable

## Optional Phase 7: PTY keeper subprocess

Only if later required.

Work:
- introduce tiny per-session helper that owns PTY masters
- allow owner process to restart and rebind to existing PTYs

Trade-off:
- improves continuity across owner-process death
- weakens pure single-process simplicity
- should remain optional and delayed

---

## Code-level guidance

### Current code behaviors that must change

#### Pane termination must stop closing PTY on all crashes
Current direction in `pkg/shux/pane.go` ties pane termination to runtime closure. That blocks pane healing and must be split.

#### Window termination must stop shutting down panes on crash
Current direction in `pkg/shux/window.go` cascades shutdown to panes. That blocks window healing.

#### Session termination must stop shutting down windows on crash
Current direction in `pkg/shux/session.go` cascades shutdown to windows. That blocks session healing.

### Semantic split to introduce

Use explicit terms:
- `Crash` / `ControllerPanic`
- `RestartController`
- `KillPaneRuntime`
- `KillWindow`
- `KillSession`
- `DetachClient`

Avoid overloaded meanings of `Stop()` and `Shutdown()` where possible.

---

## Testing plan

Add tests for each recovery level.

### Live continuity
- detach keeps shell alive
- reattach sees same running shell state
- stale live metadata falls back to cold restore

### Pane healing
- inject pane-controller panic
- verify child PID/PTY continuity remains
- verify pane recovers around same runtime

### Window healing
- inject window-controller panic
- verify pane runtimes survive
- verify layout and active pane are restored

### Session healing
- inject session-controller panic
- verify windows survive
- verify active window/session state restored

### Cold restore
- owner death -> restart from structural snapshot
- visual state shown immediately from playback snapshot
- shell/process continuity is not falsely claimed

### Reboot-style restore
- no live owner metadata valid
- structural + visual restore still works

---

## Success criteria

The architecture is correct when all of these are true:

1. **Detach / reattach** preserves real running shells/apps on the same machine
2. **Pane controller panic** does not kill the shell in that pane
3. **Window controller panic** does not kill panes in that window
4. **Session controller panic** does not kill windows in that session
5. **Owner death** restores topology + last visible content cleanly
6. **Machine reboot** restores topology + last visible content cleanly
7. The code clearly distinguishes:
   - live continuity
   - controller healing
   - cold restore

---

## Recommended implementation order

1. Stabilize live owner/client attach correctness
2. Split pane runtime from pane controller
3. Stop window/session crash cleanup from cascading downward
4. Add supervision + registries
5. Implement pane/window/session rebuild contracts
6. Add durable pane visual snapshots/playback
7. Polish recovery manager and observability
8. Consider PTY keeper only later if owner-crash continuity becomes necessary

---

## Final recommendation

The single most important move is this:

> Refactor shux so PTY/process ownership lives below pane/window/session controller logic, and only explicit kill paths destroy it.

That one change makes:
- pane healing possible
- window healing inherit naturally
- session healing inherit naturally
- durable fallback cleaner and more honest

This should be the guiding architectural direction for shux.
