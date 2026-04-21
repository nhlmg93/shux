# shux v0.1.0 Roadmap

## Release Goal

Ship a **minimal, daily-drivable terminal multiplexer** that feels familiar to tmux users while staying true to shux's design:

- **single-process**
- **full terminal emulation via Ghostty**
- **simple, explicit loop-based concurrency**
- **disk-backed detach/restore**
- **small, understandable configuration surface**
- **Linux-first scope for v0.1.0**

v0.1.0 does **not** need to be a full tmux clone. It does need to cover the core workflows people expect when they try to live in it all day.

## Release Principles

1. **Session persistence is already good enough for v0.1.0.**
   Keep it stable, do not expand it much further right now.
2. **Copy tmux where it buys instant usability.**
   Default keys and command names should preserve muscle memory when practical.
3. **Prefer a narrow, polished feature set over broad partial compatibility.**
4. **No client/server split.**
   Multi-client tmux semantics remain out of scope.
5. **Every new user-facing feature should land through a stable action layer.**
   Avoid adding more hardcoded key handling directly in the UI.
6. **Target Linux only for now.**
   Broader multiplatform support can come later, closer to v1.

---

## Current State (Implemented)

### Core architecture
| Feature | Status | Notes |
|---|---|---|
| Single-process event-loop architecture | ✅ | Session → Window → Pane loop model |
| Ghostty terminal emulation | ✅ | Full-screen apps, colors, unicode, mouse-capable terminal core |
| PTY lifecycle management | ✅ | Shell spawning and pane-local CWD tracking |
| Disk-backed detach/restore | ✅ | Gob snapshots saved and restored by session name |
| Lua config loading | ✅ | `init.lua`, shell/session config, plugin hooks |

### Window management
| Feature | Status | Notes |
|---|---|---|
| Create window | ✅ | Currently bound in UI |
| Next/previous window | ✅ | Already implemented |
| Window cleanup | ✅ | Empty windows are cleaned up |

### Pane management
| Feature | Status | Notes |
|---|---|---|
| Horizontal split | ✅ | Already implemented |
| Vertical split | ✅ | Already implemented |
| Pane navigation | ✅ | Already implemented |
| Binary split tree layout | ✅ | Good enough for v0.1.0 |
| Active pane border highlight | ✅ | Basic visual focus works |

### Session persistence
| Feature | Status | Notes |
|---|---|---|
| Detach and save | ✅ | Existing foundation |
| Restore on startup | ✅ | Existing foundation |
| Window/pane layout restore | ✅ | Existing foundation |
| Per-pane CWD restore | ✅ | Existing foundation |

---

## What v0.1.0 Must Feel Like

A user should be able to:

- start or restore a named session
- create, select, rename, and kill windows
- split, move around, resize, zoom, swap, and kill panes
- detach safely and come back later
- discover and attach saved sessions from the CLI
- use a tmux-like **`prefix + :`** command prompt for core actions
- jump around with a **minimal tree mode** for sessions/windows
- rely on a **tasteful minimalist status line**
- customize keybindings through Lua config while the shipped defaults stay tmux-compatible

If those flows are solid, shux is plausibly shippable even without broader tmux compatibility.

---

## Phase 0: Input, Keymap, and Prompt Foundation

This phase should come first. It prevents later features from being wired in as more one-off UI branches.

### 0.1 Introduce a named action layer
Create a central action registry so UI input resolves to actions rather than directly to ad hoc message sends.

This is the foundation for removing today's hardcoded key handling from `pkg/shux/ui.go`. The UI should stop deciding that specific literals like `ctrl+b`, `c`, `n`, `p`, `w`, `s`, `:`, or `d` always mean particular operations. Instead, it should ask the keymap layer to resolve input into a named action, then dispatch that action.

Examples:
- `new_window`
- `next_window`
- `prev_window`
- `select_window_0` ... `select_window_9`
- `split_horizontal`
- `split_vertical`
- `select_pane_left/right/up/down`
- `kill_pane`
- `kill_window`
- `rename_window`
- `rename_session`
- `last_window`
- `zoom_pane`
- `resize_pane_left/right/up/down`
- `swap_pane_up/down`
- `detach`
- `command_prompt`
- `choose_tree_sessions`
- `choose_tree_windows`
- `show_help`

### 0.2 Ship tmux defaults as the default keymap
Default bindings for v0.1.0 should simply be **tmux defaults** for every action shux implements. No alternate shux-specific default layout.

#### Recommended default keymap for v0.1.0
| Action | tmux default | shux v0.1.0 default | Notes |
|---|---|---|---|
| Prefix | `C-b` | `C-b` | Configurable, but default stays tmux |
| New window | `c` | `c` | Match tmux |
| Next window | `n` | `n` | Match tmux |
| Previous window | `p` | `p` | Match tmux |
| Select window by index | `0-9` | `0-9` | Match tmux |
| Last window | `l` | `l` | Match tmux |
| Rename window | `,` | `,` | Match tmux |
| Rename session | `$` | `$` | Match tmux |
| Kill window | `&` | `&` | Match tmux |
| Choose session tree | `s` | `s` | Match tmux |
| Choose window tree | `w` | `w` | Match tmux |
| Split top/bottom | `"` | `"` | tmux `split-window -v` |
| Split left/right | `%` | `%` | tmux `split-window -h` |
| Select pane | arrows | arrows | Match tmux |
| Kill pane | `x` | `x` | Match tmux |
| Zoom pane | `z` | `z` | Match tmux |
| Resize pane | `C-arrows` | `C-arrows` | Match tmux |
| Swap pane up/down | `{` / `}` | `{` / `}` | Match tmux |
| Detach | `d` | `d` | Match tmux |
| Command prompt | `:` | `:` | Match tmux |
| Help / list keys | `?` | `?` | Match tmux |

Users can add non-tmux bindings in config, but the shipped defaults should be straight tmux muscle memory.

### 0.3 Move hardcoded keymaps into Lua-backed config
The immediate goal is to take the bindings currently hardcoded in the UI and move them behind a config-backed keymap API.

#### Design rules
- shipped defaults are tmux defaults
- Lua config can override or remove bindings
- the UI consumes a resolved keymap, not raw hardcoded switch cases
- all feature bindings go through named actions
- prompt mode and tree mode may still have their own local key handling, but the top-level prefix table should be config-driven

#### Minimal config surface for v0.1.0
- set prefix
- override bindings for named actions
- unbind actions
- optionally add extra bindings for the same action

No tmux-style command language is needed here. Keep it declarative and small.

#### Configuration shape
Keep the API simple and explicit. For example, the config layer should be able to express something close to:

```lua
return require("shux").config({
  keys = {
    prefix = "C-b",
    bind = {
      ["c"] = "new_window",
      ["n"] = "next_window",
      ["p"] = "prev_window",
      ["w"] = "choose_tree_windows",
      ["s"] = "choose_tree_sessions",
      [":"] = "command_prompt",
    },
    unbind = {
      -- optional
    },
  },
})
```

The exact Lua shape can change, but the behavior should be:
- load tmux-default keymap first
- apply user overrides second
- validate the final resolved map before the UI starts

### 0.4 Add prompt/input mode
Build a small reusable bottom-line prompt widget for:
- command prompt (`prefix + :`)
- rename window (`prefix + ,`)
- rename session (`prefix + $`)
- future confirmation prompts if desired

The important part is reusing one clean input path instead of inventing custom mini-UIs per feature.

### 0.5 Add a minimal command registry
Implement a small command parser and registry. It does **not** need full tmux syntax.

#### v0.1.0 command set
| Command | Purpose |
|---|---|
| `new-window` | Create a window |
| `select-window <index>` | Select window by index |
| `last-window` | Toggle to previous window |
| `kill-window` | Kill current window |
| `rename-window <name>` | Rename current window |
| `split-window -h` | Horizontal split |
| `split-window -v` | Vertical split |
| `kill-pane` | Kill active pane |
| `resize-pane -L/-R/-U/-D [n]` | Resize active pane |
| `swap-pane -U/-D` | Swap pane position |
| `resize-pane -Z` | Zoom/unzoom active pane |
| `rename-session <name>` | Rename session |
| `list-sessions` | List saved sessions |
| `attach-session <name>` | Attach named session |
| `choose-tree -s` | Open minimal session tree |
| `choose-tree -w` | Open minimal window tree |
| `detach` | Detach and save |
| `list-keys` | Show current bindings/help view |

### 0.6 Validate config and binding conflicts
Startup should fail loudly or warn clearly when:
- prefix is invalid
- a binding is malformed
- two bindings collide unexpectedly
- a config refers to an unknown action

### Phase 0 file impact
**Existing files likely touched**
- `cmd/shux/config.go`
- `cmd/shux/config_lua.go`
- `cmd/shux/main.go`
- `pkg/shux/ui.go`

**Likely new files**
- `pkg/shux/keymap.go`
- `pkg/shux/actions.go`
- `pkg/shux/commands.go`
- `pkg/shux/prompt.go`

#### Specific refactor target
- `cmd/shux/config*.go` should parse and validate keymap config
- `cmd/shux/main.go` should pass resolved config into the app/model startup path
- `pkg/shux/ui.go` should dispatch actions from a resolved keymap rather than hardcoded key literals
- `pkg/shux/keymap.go` should own tmux defaults, merge logic, normalization, and conflict detection

### Phase 0 acceptance criteria
- no feature-critical binding is hardcoded only in `ui.go`
- tmux defaults are the shipped keymap baseline
- prefix is configurable from Lua
- at least one binding can be overridden in Lua without code changes
- unbinding an action works
- `prefix + :` opens a prompt and can execute at least one real command
- rename flows use the same prompt component
- tree-mode entry actions exist in the action/keymap layer even if the UI lands in a later phase

---

## Phase 1: Essential Window and Session Operations

These are the missing basics that make shux feel incomplete compared to tmux.

### 1.1 Select window by index
- **User value:** fast access to windows without cycling
- **tmux reference:** `prefix 0-9` → `select-window -t:=N`
- **Implementation shape:** `SelectWindow{Index int}` message handled in session

### 1.2 Kill current window
- **User value:** obvious lifecycle control
- **tmux reference:** `prefix &` → `kill-window`
- **Implementation shape:** `KillWindow{}` message, active window cleanup, next-active-window selection
- **Behavior rule:** killing the final window should end the session cleanly according to current empty-session behavior

### 1.3 Rename current window
- **User value:** persistent labels beyond shell-provided titles
- **tmux reference:** `prefix ,` → `rename-window`
- **Implementation shape:** prompt-backed rename action, stored on window state

### 1.4 Last window toggle
- **User value:** fast two-window workflow
- **tmux reference:** `prefix l` → `last-window`
- **Implementation shape:** session tracks previous active window ID

### 1.5 Rename session
- **User value:** saved sessions need stable, human-friendly names
- **tmux reference:** `prefix $` → `rename-session`
- **Implementation shape:** rename in memory plus atomic snapshot rename on disk

### Phase 1 file impact
**Existing files likely touched**
- `pkg/shux/messages.go`
- `pkg/shux/session.go`
- `pkg/shux/window.go`
- `pkg/shux/snapshot.go`
- `pkg/shux/ui.go`

### Phase 1 acceptance criteria
- window selection by number works reliably
- rename window/session works from both keybinding and prompt
- kill-window never leaves broken session state
- last-window always toggles between the right two windows

---

## Phase 2: Essential Pane Operations

Pane control is the heart of daily multiplexer use. These features should be treated as release blockers.

### 2.1 Kill pane
- **User value:** explicit pane lifecycle control
- **tmux reference:** `prefix x` → `kill-pane`
- **Implementation shape:** wire the existing capability into the action/keymap layer

### 2.2 Zoom/unzoom pane
- **User value:** temporarily focus one pane without changing the long-term layout
- **tmux reference:** `prefix z` → `resize-pane -Z`
- **Implementation shape:** window-level zoom flag and saved pre-zoom layout state or render rule
- **Persistence rule:** zoom state can be treated as transient for v0.1.0 if that simplifies snapshot compatibility

### 2.3 Resize pane
- **User value:** basic layout control
- **tmux reference:** `prefix C-Arrow` / `M-Arrow`
- **Implementation shape:** resize message updates split geometry or weights

### 2.4 Swap panes
- **User value:** fix layout mistakes without respawning shells
- **tmux reference:** `prefix {` / `prefix }`
- **Implementation shape:** swap pane IDs or leaf nodes in the split tree, then relayout

### Phase 2 file impact
**Existing files likely touched**
- `pkg/shux/messages.go`
- `pkg/shux/window.go`
- `pkg/shux/ui.go`

### Phase 2 acceptance criteria
- pane kill, zoom, resize, and swap all work without corrupting layout
- restored sessions still load after these features land
- full-screen terminal apps still behave correctly in split and zoomed states

---

## Phase 3: Session Discovery and CLI Basics

Persistence already exists, but users also need a clean way to discover and reattach sessions.

### 3.1 `shux list`
- **User value:** see restorable sessions
- **tmux reference:** `tmux ls`
- **Implementation shape:** scan snapshot directory and print a concise table

### 3.2 `shux attach <name>`
- **User value:** explicit attach flow for named sessions
- **tmux reference:** `tmux attach -t <name>`
- **Implementation shape:** restore and launch the requested snapshot by name

### 3.3 Snapshot utilities
Support the session operations above with small snapshot helpers:
- list snapshots
- inspect snapshot metadata if needed
- atomically rename snapshot file for session rename

### Phase 3 file impact
**Existing files likely touched**
- `cmd/shux/root.go`
- `cmd/shux/main.go`
- `pkg/shux/snapshot.go`
- `pkg/shux/session.go`

**Likely new files**
- `cmd/shux/list.go`
- `cmd/shux/attach.go`

### Phase 3 acceptance criteria
- users can discover saved sessions without inspecting filesystem state manually
- attach works for named sessions
- rename-session keeps snapshot naming consistent

---

## Phase 4: Minimal Tree Mode

This should be intentionally small: a practical chooser, not a full replica of tmux tree mode.

### 4.1 Session tree
- **User value:** quickly jump among saved/live sessions without remembering exact names
- **tmux reference:** `prefix s` → `choose-tree -Zs`
- **Implementation shape:** overlay or temporary full-screen list showing sessions as a simple tree root
- **Minimum interactions:** up/down, optional `j/k`, enter to choose, escape to cancel

### 4.2 Window tree
- **User value:** jump directly to windows instead of cycling through many of them
- **tmux reference:** `prefix w` → `choose-tree -Zw`
- **Implementation shape:** current-session window list, optionally showing panes beneath each window if cheap
- **Minimum interactions:** up/down, optional `j/k`, enter to choose, escape to cancel

### 4.3 Keep scope deliberately narrow
For v0.1.0, tree mode does **not** need:
- previews
- tagging
- filtering
- search
- kill actions from inside tree mode
- pane marking
- reorder operations inside tree mode

If panes can be shown cheaply as leaf rows, great. If not, sessions and windows alone are enough for the first release.

### 4.4 Shared picker component
Tree mode should reuse the same input/rendering foundation where practical:
- prompt/status state awareness
- overlay rendering rules
- action dispatch on selection

### Phase 4 file impact
**Existing files likely touched**
- `pkg/shux/ui.go`
- `pkg/shux/session.go`
- `pkg/shux/window.go`

**Likely new files**
- `pkg/shux/tree_mode.go`

### Phase 4 acceptance criteria
- `prefix + s` opens a session chooser
- `prefix + w` opens a window chooser
- selecting an entry performs the right attach/switch action
- tree mode can be cancelled cleanly without side effects

---

## Phase 5: Minimalist Status Line, Help, and Shipping UX

This phase matters even if the implementation stays intentionally restrained.

### 5.1 Tasteful minimalist status line
shux should ship with a **single-line, low-noise status bar** that helps users orient themselves without turning the UI into a dashboard.

#### Status line goals
- look calm and modern, not busy
- stay out of the way during normal terminal use
- provide just enough information for people who rely on a multiplexer bar
- be easy to disable later for users who do not want it

#### v0.1.0 status line contents
**Left side**
- session name
- prefix indicator when prefix mode is active
- prompt indicator when command/input mode is active

**Center / main body**
- window list in order
- active window clearly marked
- renamed windows shown by name
- unnamed windows fall back to shell/window title

**Right side**
- keep empty for v0.1.0 unless a tiny indicator is genuinely useful

#### Visual rules
- one line only
- bottom of screen by default
- use terminal default colors plus one accent color
- no clocks, battery widgets, format mini-language, or per-segment theming for v0.1.0
- subtle separators, not heavy boxes

#### Example shape
```text
 shux:work   [prefix]   1:shell   2:server*   3:logs
```

### 5.2 Help / key listing
- `prefix + ?` should show the current keymap or a concise help overlay
- this can be generated from the action/keymap registry rather than handwritten

### 5.3 Command prompt UX polish
- prompt should support basic editing
- escape cancels
- enter submits
- rename commands can prefill existing value where possible

### 5.4 Destructive-action safety
For v0.1.0, this can be lightweight. Options:
- immediate execution for simplicity, or
- tiny confirmation prompt for `kill-pane` / `kill-window`

If time is tight, ship immediate execution first and revisit confirmation in v0.1.1.

### Phase 5 file impact
**Existing files likely touched**
- `pkg/shux/ui.go`
- `pkg/shux/window.go`

**Likely new files**
- `pkg/shux/statusline.go`
- `pkg/shux/help.go`

### Phase 5 acceptance criteria
- status line is readable and visually restrained
- active window and session are always obvious
- prompt state is visible
- help output matches the actual shipped bindings

---

## Recommended Implementation Order

1. **Phase 0** — action layer, keymap, config, prompt, command registry
2. **Phase 1** — missing window/session basics
3. **Phase 2** — remaining pane essentials
4. **Phase 3** — list/attach session CLI
5. **Phase 4** — minimal tree mode
6. **Phase 5** — status line, help, final UX pass
7. **Stabilization** — tests, docs, release notes, keymap cleanup

This order keeps input architecture ahead of feature wiring and avoids having to rework everything later.

---

## Testing and Hardening Requirements

### Unit / integration coverage
Add or extend tests for:
- tmux default keymap loading
- config parsing and invalid-keymap handling
- prefix override behavior
- override precedence over defaults
- unbind behavior
- command parsing and dispatch
- window selection / last-window tracking
- rename window / rename session
- kill window / kill pane lifecycle
- pane zoom / resize / swap correctness
- snapshot rename and session listing

### End-to-end workflows
Before v0.1.0, verify at least these flows:
1. create session → split panes → rename window → detach → restore
2. create multiple windows → select by number → last-window toggle → kill one
3. resize and zoom panes while running full-screen terminal apps
4. load config with custom prefix and key overrides
5. use command prompt to perform at least the core window/session actions
6. `shux list` and `shux attach <name>` work from the CLI
7. open session tree and window tree, navigate, select, and cancel cleanly

### Documentation requirements
Before release, document:
- default keybindings
- how to change prefix in Lua
- how to override or disable bindings
- command prompt basics
- tree mode basics and keybindings
- session naming, list, and attach behavior
- status line behavior and whether it can be disabled

---

## Release Criteria

shux v0.1.0 is shippable when all of the following are true:

1. **Core window workflows are complete**
   - create, next/prev, select by number, rename, kill, last-window
2. **Core pane workflows are complete**
   - split, navigate, kill, zoom, resize, swap
3. **Persistence remains reliable**
   - detach/restore still works without regressions
4. **Config and keymap foundation exists**
   - hardcoded top-level keymaps replaced, prefix configurable, tmux defaults shipped, action keymap documented, invalid config handled well
5. **Command prompt exists**
   - `prefix + :` supports the minimal command set
6. **Session discovery works**
   - `shux list` and `shux attach <name>` are usable
7. **Minimal tree mode works**
   - users can choose sessions/windows without cycling manually
8. **Status line ships**
   - minimalist, tasteful, and useful rather than noisy
9. **Docs are good enough for first-time tmux users**
10. **Existing tests pass and new regressions are covered**

---

## Explicitly Deferred to Post-v0.1.0

These are useful, but should stay out of the critical path:

| Feature | Why deferred |
|---|---|
| Expanding session persistence further | Current foundation is already the right v0.1.0 baseline |
| Multi-client attach to one session | Conflicts with shux single-process design goals |
| Full tmux command language compatibility | Too large, too much parser and target syntax work |
| Full copy mode / search / clipboard stack | Ghostty scrollback and terminal tools cover a lot already |
| Mouse drag resizing | Nice-to-have, not core to MVP |
| Complex tmux layout families | Binary split tree is sufficient for first release |
| Runtime config reload | Extra complexity with limited immediate value |
| Rich theming / status-line mini-language | Easy to bloat the UI surface |
| Synchronize-panes | Secondary workflow |
| Full option system (`set-option`, scopes, inheritance) | Better after core UX is stable |

---

## Tmux Reference Points

Use `/tmp/tmux/key-bindings.c` as the primary reference for default bindings and user expectations.

Especially relevant ranges:
- **window/session bindings:** around lines `361-409`
- **tree mode bindings:** around lines `403-405`
- **pane navigation and resize:** around lines `406-438`
- **command prompt binding:** around line `381`
- **help / list keys:** around lines `370` and `384`
- **tree mode behavior:** `/tmp/tmux/tmux.1` around line `2768`

The goal is **behavioral familiarity**, not full implementation parity.

---

## Bottom Line

For shux v0.1.0, the highest-leverage work is:

1. build the **input/keymap/prompt foundation**
2. finish the **missing window/pane essentials**
3. expose **session discovery and attach** cleanly
4. ship a **minimal tree mode**, a **minimalist status line**, and a **small command prompt**
5. leave persistence expansion and broad tmux compatibility for later

If those pieces land cleanly, shux can ship as a credible minimal tmux alternative instead of just an interesting prototype.

---

*Last updated: 2026-04-20*