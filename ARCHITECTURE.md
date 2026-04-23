# Simple Shux Architecture Plan

## Goal

Keep it **simple as fuck**:

- one background Go process
- one TCP port
- many clients can attach/detach
- sessions/windows/panes live in one shared in-memory tree
- virtual terminals are goroutines
- cross-platform by avoiding Unix socket vs named pipe complexity

---

## Core idea

Treat it more like a tiny app server than `tmux` internals:

- the **server** owns all session state
- a **client** connects over TCP
- local terminals are just clients
- remote access is optional later

Default mental model:

```text
terminal client --> TCP --> one Go daemon --> sessions/windows/panes/vts
```

---

## Why this direction

### Simplicity

We do **not** want to deal with:

- Unix sockets on Linux/macOS
- named pipes / `go-winio` on Windows
- per-platform IPC differences
- socket-path discovery logic

Instead:

- one host
- one port
- one code path

### Cross-platform

Go supports TCP everywhere.

So this works on:

- Linux
- macOS
- Windows

with the same transport layer.

### Easy attach/detach

A client disconnecting does not kill the daemon.
Another client can reconnect later.

---

## Default networking model

Bind only to localhost by default:

```text
127.0.0.1:23234
```

That means:

- local-only by default
- no accidental remote exposure
- very simple safety story

---

## Optional remote access later

If we want remote connections later, we can allow:

```text
0.0.0.0:23234
```

but only if explicitly enabled in config.

### Remote safety plan

If non-local access is enabled, require at least:

- explicit config flag
- IP allowlist
- auth token / password

Example config shape:

```toml
bind = "127.0.0.1:23234"
allow_remote = false
allowed_ips = ["192.168.1.50"]
auth_token = "change-me"
```

---

## Process model

### One daemon process

The daemon owns everything:

- sessions
- windows
- panes
- virtual terminals
- client connections

### Many client connections

Each attached terminal is just another client.

A client can:

- attach to a session
- switch windows
- switch panes
- write input
- detach

---

## State model

Single in-memory tree:

```text
Server
└── Sessions
    ├── Session demo
    │   ├── Window 1
    │   │   ├── Pane 1 -> VT goroutine
    │   │   ├── Pane 2 -> VT goroutine
    │   │   ├── Pane 3 -> VT goroutine
    │   │   └── Pane 4 -> VT goroutine
    │   ├── Window 2
    │   └── ...
    └── Session work
```

### Important separation

- **server state** is shared
- **client UI state** is per-client

So two clients can both attach to the same session but have different views/focus.

Example:

- Client A looking at window 2 pane 3
- Client B looking at window 1 pane 1
- both connected to same session

---

## Virtual terminal model

For now, keep it mocked.

Each pane has a goroutine that:

- wakes up periodically
- appends fake output to its buffer

Later we can replace mock VTs with real PTYs.

### Phase 1

Mock VT:

- fake ticker output
- fake input writes
- simple buffer

### Phase 2

Real PTY/ConPTY backend:

- Unix PTY on Linux/macOS
- ConPTY on Windows

The rest of the architecture stays the same.

---

## Transport protocol

Keep it dead simple.

Client connects and sends line-based commands.

Examples:

```text
attach demo
window 2
pane 3
write hello
detach
```

Server sends rendered text view back.

This is enough for the prototype.

Later we can upgrade to a cleaner message protocol if needed.

---

## Rendering model

Prototype rendering:

- server sends full text redraws
- client terminal just prints them

Later:

- smarter diff rendering
- proper TUI
- alternate screen
- keyboard shortcuts

But not yet.

---

## Startup model

### Client startup flow

1. client tries to connect to TCP port
2. if server exists, attach
3. if server does not exist, spawn it
4. retry connection
5. attach

This gives the UX we want:

```bash
go run main.go demo
```

If server is already running, it attaches.
If not, it starts the server and then attaches.

---

## Why not tmux-style Unix socket architecture

Because right now the product goal is:

- simplest implementation
- easiest cross-platform story
- one transport
- one mental model

Unix socket + Windows named pipe is more "native," but more annoying.

TCP localhost is simpler.

---

## Security stance

### Default

Safe enough by default if we bind to localhost only.

### If remote is enabled

Then security becomes a real concern.
At minimum, add:

- allowlist
- auth token
- clear warning in config/docs

If we later want strong remote access, we can put SSH/Wish in front of it.

---

## Planned phases

### Phase 1: simple prototype

- one daemon
- one localhost TCP port
- mocked sessions/windows/panes
- mocked virtual terminals
- multiple attach/detach clients

### Phase 2: better local UX

- proper client UI state
- multiple sessions
- shortcuts
- better rendering

### Phase 3: real terminals

- PTY on Unix
- ConPTY on Windows

### Phase 4: optional remote mode

- config-driven remote bind
- allowlist
- auth token
- maybe SSH/Wish later

---

## Final decision

For now we choose:

- **one daemon**
- **one TCP port**
- **localhost by default**
- **spawn-if-missing attach flow**
- **mock virtual terminals first**

Because it is:

- simple
- cross-platform
- easy to understand
- easy to prototype
- easy to evolve later
