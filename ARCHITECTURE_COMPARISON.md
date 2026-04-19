# gomux vs tmux Architecture Comparison

## High-Level Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ TMUX                                                                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────┐      socket       ┌─────────────────┐                  │
│  │   tmux client   │◄─────────────────►│   tmux server   │                  │
│  │   (ncurses UI)  │   (Unix domain)   │  (holds state)  │                  │
│  └─────────────────┘                   └────────┬────────┘                  │
│                                                  │                          │
│                    ┌─────────────────────────────┼──────────────────────┐   │
│                    │                             ▼                      │   │
│                    │  ┌─────────┐  ┌─────────┐  ┌─────────┐             │   │
│                    │  │ Session │─►│ Window  │─►│  Pane   │             │   │
│                    │  │         │  │         │  │ (pty+emu) │             │   │
│                    │  └─────────┘  └─────────┘  └─────────┘             │   │
│                    │                              │  │  │                  │   │
│                    │                              ▼  ▼  ▼                  │   │
│                    │                         ┌────────────┐               │   │
│                    │                         │ /bin/sh    │               │   │
│                    │                         │ vim, htop  │               │   │
│                    │                         └────────────┘               │   │
│                    └──────────────────────────────────────────────────────┘   │
│                                                                             │
│  Key: Client/Server model with socket communication                         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│ GOMUX                                                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                     SINGLE PROCESS (no client/server)                   │ │
│  │                                                                         │ │
│  │   ┌─────────────┐                                                       │ │
│  │   │  Bubble Tea │  ← TUI Framework (renders UI)                         │ │
│  │   │    UI       │                                                       │ │
│  │   └──────┬──────┘                                                       │ │
│  │          │                                                              │ │
│  │          ▼                                                              │ │
│  │   ┌─────────────┐    actor messages    ┌─────────────┐                 │ │
│  │   │  Supervisor │◄────────────────────►│   Session   │                 │ │
│  │   │             │                       │   Actor     │                 │ │
│  │   └─────────────┘                       └──────┬──────┘                 │ │
│  │                                                │                        │ │
│  │                       ┌────────────────────────┼────────────────────┐  │ │
│  │                       │                        ▼                    │  │ │
│  │                       │  ┌─────────────┐    ┌─────────────┐       │  │ │
│  │                       │  │   Window    │───►│    Term     │       │  │ │
│  │                       │  │   Actor     │    │   Actor     │       │  │ │
│  │                       │  └─────────────┘    └──────┬──────┘       │  │ │
│  │                       │                           │   │  │         │  │ │
│  │                       │                           ▼   ▼  ▼         │  │ │
│  │                       │  ┌────────────┐    ┌──────────────┐         │  │ │
│  │                       │  │  Go PTY   │    │ tonistiigi   │         │  │ │
│  │                       │  │ (creack)  │    │   /vt100     │         │  │ │
│  │                       │  └─────┬──────┘    └──────────────┘         │  │ │
│  │                       │        │                                  │  │ │
│  │                       │        ▼                                  │  │ │
│  │                       │   ┌────────────┐                            │  │ │
│  │                       │   │  /bin/sh   │                            │  │ │
│  │                       │   │  vim, etc  │                            │  │ │
│  │                       │   └────────────┘                            │  │ │
│  │                       └─────────────────────────────────────────────┘  │ │
│  │                                                                         │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
│  Key: Actor model with message passing (gotor framework)                  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Detailed Component Comparison

| Aspect | tmux | gomux |
|--------|------|-------|
| **Process Model** | Client/Server (can detach) | Single process |
| **Concurrency** | libevent (C) | Actor model (Go goroutines) |
| **UI Framework** | ncurses | Bubble Tea |
| **Terminal Emulation** | Custom/libevent term | tonistiigi/vt100 (Go) |
| **PTY Management** | Direct (C) | creack/pty (Go) |
| **Communication** | Unix domain socket | Go channels (in-process) |
| **Persistence** | Server keeps state | State lost on exit |

## Data Flow Comparison

### tmux:
```
User types key
    ↓
tmux client captures input
    ↓
Sends command over socket to server
    ↓
tmux server routes to appropriate pane
    ↓
Writes to pane's PTY
    ↓
Shell process receives input
    ↓
Shell outputs text
    ↓
Pane's terminal emulator parses escapes
    ↓
Grid updated
    ↓
Server marks pane as dirty
    ↓
Client polls/redraws dirty panes
    ↓
ncurses renders to terminal
```

### gomux:
```
User types key
    ↓
Bubble Tea captures input (tea.KeyMsg)
    ↓
UI sends WriteToTerm message
    ↓
Actor system routes to Term actor
    ↓
Term writes to PTY directly
    ↓
Shell process receives input
    ↓
Shell outputs text
    ↓
PTY readLoop receives bytes
    ↓
vt100.Write() parses escapes
    ↓
Grid (Content[][]) updated
    ↓
GridUpdated message sent to UI
    ↓
Bubble Tea re-renders (tea.Model.Update/View)
    ↓
ANSI escapes written to stdout
    ↓
User's terminal displays
```

## Key Architectural Decisions

### tmux chose Client/Server because:
- **Detach/reattach**: Close terminal, come back later
- **Persistence**: Sessions survive client crashes
- **Multiple clients**: Attach same session from different terminals
- **Remote access**: Can attach over SSH

### gomux chose Single Process because:
- **Simplicity**: No socket protocol to design
- **Go idiomatic**: Channels vs sockets
- **Development**: Easier to debug single process
- **Trade-off**: No detach/reattach (yet)

### tmux chose libevent because:
- **C ecosystem**: Efficient event loop
- **Mature**: Battle-tested, handles edge cases

### gomux chose Actor Model because:
- **Go natural**: Goroutines + channels
- **Composability**: Actors are self-contained
- **Fault isolation**: Actor crashes don't kill system
- **Hot reload**: Could replace actors dynamically

## Terminal Emulation Comparison

| Feature | tmux | gomux (vt100) |
|---------|------|---------------|
| **Base** | VT220 + extensions | VT100 |
| **256 colors** | ✅ Yes | ✅ Yes |
| **True color (24-bit)** | ✅ Yes | ❌ No |
| **Scrollback buffer** | ✅ Yes | ❌ No |
| **Mouse support** | ✅ Yes | ❌ No |
| **Unicode** | ✅ Full | ✅ UTF-8 |
| **Alt screen** | ✅ Yes | ❌ No |
| **Bracketed paste** | ✅ Yes | ❌ No |
| **DCS sequences** | ✅ Many | ❌ Limited |

## When to Use Which

### Use tmux if:
- You need detach/reattach
- You run long processes (compile, train ML)
- You need scrollback
- You use complex apps (vim with plugins, tmux-resurrect)
- You want ecosystem (tmuxinator, tpm, etc.)

### Use gomux if:
- You want to learn how multiplexers work
- You want hackable code (learning project)
- You prefer Go/CGO-free
- You want to experiment with architecture
- You're building a custom tool

## Future Convergence

To make gomux production-ready like tmux:

1. **Add client/server split** (hardest)
   - Define protocol (gRPC? Cap'n Proto?)
   - Handle reconnection
   - State serialization

2. **Better terminal emulation**
   - Replace vt100 with libtsm (C) or custom
   - Add scrollback buffer
   - Add true color

3. **Add features**
   - Copy mode
   - Search
   - Plugins

4. **Or stay simple**
   - Keep as learning tool
   - Don't try to replace tmux
