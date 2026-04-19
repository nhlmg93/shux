package gomux

/*
#cgo LDFLAGS: -L${SRCDIR}/../../gomux-term/target/release -lgomux_term -lpthread -ldl
#include <stdlib.h>
#include "../../gomux-term/gomux_term.h"
*/
import "C"
import (
	"unsafe"

	"github.com/nhlmg93/gotor/actor"
)

// TermActor replaces PaneActor - each actor IS an Alacritty terminal
type TermActor struct {
	id     uint32
	term   C.GomuxPane  // Rust handle
	parent *actor.Ref
	self   *actor.Ref
}

// NewTermActor creates terminal with shell
func NewTermActor(id uint32, rows, cols int, shell string, parent *actor.Ref) *TermActor {
	cShell := C.CString(shell)
	defer C.free(unsafe.Pointer(cShell))
	
	term := C.gomux_pane_new(C.uint(rows), C.uint(cols), cShell)
	if term == nil {
		return nil
	}
	
	return &TermActor{
		id:     id,
		term:   term,
		parent: parent,
	}
}

// SpawnTermActor creates and spawns a TermActor
func SpawnTermActor(id uint32, rows, cols int, shell string, parent *actor.Ref) *actor.Ref {
	t := NewTermActor(id, rows, cols, shell, parent)
	if t == nil {
		return nil
	}
	ref := actor.Spawn(t, 10)
	t.self = ref
	return ref
}

func (t *TermActor) Receive(msg any) {
	switch m := msg.(type) {
	case WriteToTerm:
		data := []byte(m.Data)
		if len(data) > 0 {
			C.gomux_pane_write(t.term, (*C.char)(unsafe.Pointer(&data[0])), C.uint(len(data)))
		}
	case KillTerm:
		C.gomux_pane_free(t.term)
		if t.parent != nil {
			t.parent.Send(TermExited{ID: t.id})
		}
	case GetTermContent:
		// Handled via Ask
	case actor.AskEnvelope:
		t.handleAsk(m)
	}
}

func (t *TermActor) handleAsk(envelope actor.AskEnvelope) {
	switch envelope.Msg.(type) {
	case GetTermContent:
		// Tick to process any pending PTY data
		C.gomux_pane_tick(t.term)
		
		content := &TermContent{
			Lines: make([]string, 24), // TODO: get actual size
		}
		
		buf := make([]byte, 1024)
		for row := 0; row < 24; row++ {
			n := C.gomux_pane_get_line(t.term, C.uint(row), (*C.char)(unsafe.Pointer(&buf[0])), 1024)
			if n > 0 {
				content.Lines[row] = string(buf[:n])
			}
		}
		
		var row, col C.uint
		C.gomux_pane_get_cursor(t.term, &row, &col)
		content.CursorRow = int(row)
		content.CursorCol = int(col)
		
		envelope.Reply <- content
	default:
		envelope.Reply <- nil
	}
}

// Tick should be called regularly to process PTY output
func (t *TermActor) Tick() {
	C.gomux_pane_tick(t.term)
}

// NeedsRender checks if content changed
func (t *TermActor) NeedsRender() bool {
	return C.gomux_pane_needs_render(t.term) == 1
}

// MarkRendered resets dirty flag
func (t *TermActor) MarkRendered() {
	C.gomux_pane_mark_rendered(t.term)
}
