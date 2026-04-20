# Using New Gotor Features in Gomux

## Available Features

### 1. Actor Registry (Erlang-style)

**Use case:** Named sessions, global lookups, debugging

```go
// In session creation:
actor.Register("session:myproject", sessionRef)

// Later, from anywhere:
ref := actor.WhereIs("session:myproject")
if ref != nil {
    ref.Send(SwitchWindow{Delta: 1})
}

// Cleanup on exit:
actor.Unregister("session:myproject")
```

**Where to use in gomux:**
- `cmd/gomux/main.go`: Register session by name for external access
- Session list command: List all registered sessions with `actor.Registered()`
- Attach to existing session: Look up by name

### 2. Timers

**Use case:** Auto-save snapshots, inactivity detection, periodic tasks

```go
// Auto-save every 30 seconds
var saveTimer *actor.Timer

func (s *SessionActor) Receive(msg any) {
    switch m := msg.(type) {
    case StartAutoSave:
        saveTimer = actor.SendInterval(
            actor.Self(), 
            AutoSave{}, 
            30*time.Second,
        )
    case StopAutoSave:
        if saveTimer != nil {
            saveTimer.Stop()
        }
    case AutoSave:
        s.saveSnapshot()
    }
}
```

**Where to use in gomux:**
- `pkg/gomux/session_actor.go`: Auto-save snapshots (Phase 1 persistence)
- `pkg/gomux/term.go`: Inactivity timeout detection
- `pkg/gomux/ui.go`: Periodic status bar updates

### 3. Lifecycle Callbacks

**Use case:** Resource cleanup, logging, initialization

```go
type Term struct {
    // ... fields
    pty *PTY
}

func (t *Term) Init() error {
    // Initialize logging, set up PTY
    gomux.Infof("term %d: initialized", t.id)
    return nil
}

func (t *Term) Terminate(reason error) {
    // Always cleanup, even on crash
    if t.pty != nil {
        t.pty.Close()
    }
    if t.term != nil {
        t.term.Close()
    }
    gomux.Infof("term %d: terminated (%v)", t.id, reason)
}

// Usage:
ref := actor.Spawn(actor.WithLifecycle(t), 10)
```

**Where to use in gomux:**
- `pkg/gomux/term.go`: PTY cleanup, resource management
- `pkg/gomux/session_actor.go`: Session lifecycle logging
- `pkg/gomux/window_actor.go`: Window lifecycle tracking

### 4. Context (Self/Parent)

**Already using this** - shows current pattern:

```go
func (s *SessionActor) createWindow(rows, cols int) {
    // Use Self() to pass as parent to child
    ref := SpawnWindowActor(s.windowID, actor.Self())
    
    // Use Parent() to send message upward
    if parent := actor.Parent(); parent != nil {
        parent.Send(SessionReady{ID: s.id})
    }
}
```

## Recommended Integration Points

### Immediate Wins:

1. **Session registry** - Add session names, allow external tools to send commands
2. **Auto-save timer** - Implement snapshot persistence with periodic saves
3. **Term lifecycle** - Proper PTY cleanup, better crash handling

### Example: Auto-save in SessionActor

```go
// pkg/gomux/messages.go
type StartAutoSave struct{}
type StopAutoSave struct{}
type AutoSave struct{}

// pkg/gomux/session_actor.go
type SessionActor struct {
    id       uint32
    windows  map[uint32]*actor.Ref
    active   uint32
    windowID uint32
    saveTimer *actor.Timer
}

func (s *SessionActor) Receive(msg any) {
    switch m := msg.(type) {
    case CreateWindow:
        s.createWindow(m.Rows, m.Cols)
        // Start auto-save on first window
        if s.saveTimer == nil {
            s.saveTimer = actor.SendInterval(
                actor.Self(),
                AutoSave{},
                30*time.Second,
            )
        }
    case AutoSave:
        s.saveSnapshot()
    case KillSession:
        if s.saveTimer != nil {
            s.saveTimer.Stop()
        }
        s.saveSnapshot() // Final save
    // ... rest of Receive
    }
}

func (s *SessionActor) saveSnapshot() {
    // Phase 1: serialize to disk
    Infof("session %d: saving snapshot", s.id)
    // ... gob.Marshal and write to ~/.local/share/gomux/session/snapshot.gob
}
```

### Example: Named Sessions in main.go

```go
// cmd/gomux/main.go
func main() {
    gomux.InitLogger()
    
    sessionName := os.Args[1] // e.g., "myproject"
    
    // Check if session already exists
    if existing := actor.WhereIs("session:" + sessionName); existing != nil {
        // Attach to existing session
        runUI(existing)
        return
    }
    
    // Create new session
    supervisor := &SupervisorActor{}
    supervisorRef := actor.Spawn(supervisor, 10)
    sessionRef := gomux.SpawnSessionActor(1, supervisorRef)
    
    // Register for global lookup
    if err := actor.Register("session:"+sessionName, sessionRef); err != nil {
        log.Fatal(err)
    }
    defer actor.Unregister("session:" + sessionName)
    
    runUI(sessionRef)
}
```

## Next Steps

1. Add session naming support to CLI
2. Implement auto-save with snapshots (Phase 1)
3. Add term lifecycle for proper cleanup
4. Consider selective receive for complex UI interactions
