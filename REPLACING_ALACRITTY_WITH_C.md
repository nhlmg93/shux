# Replacing Alacritty/Rust with C Library

## Option 1: libvte (Recommended)
The same engine GNOME Terminal, Tilix, and Terminator use.

**Pros:**
- Full terminal emulation (escapes, colors, attributes, scrollback)
- Widely used and tested
- GObject-based C API
- Handles Unicode, bidirectional text

**Cons:**
- Heavy dependency (pulls in GLib, GObject)
- Complex build

**Usage:**
```c
#include <vte/vte.h>

// Create terminal widget
VteTerminal *term = vte_terminal_new();

// Fork shell
vte_terminal_spawn_sync(term, VTE_PTY_DEFAULT, NULL, {"/bin/sh"}, NULL, 0, NULL, NULL, NULL, NULL, NULL);

// Get cell content
const VteCell *cell = vte_terminal_get_cell(term, row, col);
```

## Option 2: libtsm (Terminal State Machine)
By Lennart Poettering (systemd author). Smaller, focused.

**Pros:**
- Just the state machine + screen buffer (what we need)
- No widget/toolkit dependencies
- Small codebase (~2000 lines)
- C99, portable

**Cons:**
- Less feature-complete than VTE
- No built-in PTY management (we already have Go for this)

**Usage:**
```c
#include <libtsm.h>

struct tsm_screen *screen;
struct tsm_vte *vte;

tsm_screen_new(&screen, NULL, NULL);
tsm_vte_new(&vte, screen, NULL, NULL);

// Feed bytes
tsm_vte_input(vte, buffer, len);

// Get cell
const struct tsm_screen_attr *attr;
const tsm_symbol_t *symbol;
tsm_screen_get_cell(screen, x, y, &attr, &symbol, &len);
```

## Option 3: st (suckless terminal)
Modify the `st` codebase into a library. ~3000 lines of C.

**Pros:**
- Very small, understandable
- No dependencies

**Cons:**
- Not a library (need to extract)
- X11 focused (need to rip out rendering)

## Recommendation

**For gomux, use libtsm because:**
1. We already have PTY in Go (don't need VTE's PTY code)
2. We already have UI in Bubble Tea (don't need VTE's widget)
3. We just need: escape parsing + grid + cell attributes
4. libtsm provides exactly that
5. Much smaller than Alacritty dependency

## Migration Path

Replace Rust FFI with libtsm Cgo:

```go
package gomux

/*
#cgo pkg-config: libtsm
#include <libtsm.h>
*/
import "C"

type Term struct {
    screen *C.struct_tsm_screen
    vte    *C.struct_tsm_vte
    pty    *PTY
}

func (t *Term) readLoop() {
    buf := make([]byte, 4096)
    for {
        n, _ := t.pty.TTY.Read(buf)
        // Feed to libtsm
        C.tsm_vte_input(t.vte, (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(n))
    }
}
```

**Result:** Only Go + C (no Rust)
