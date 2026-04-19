package gomux

/*
#cgo LDFLAGS: -L${SRCDIR}/../../gomux-term/target/release -lgomux_term -lpthread -ldl
#include <stdlib.h>
#include "../../gomux-term/gomux_term.h"
*/
import "C"
import (
	"os/exec"
	"unsafe"

	"github.com/nhlmg93/gotor/actor"
)

// Term replaces PaneActor - wraps Alacritty FFI + Go PTY
type Term struct {
	id     uint32
	term   C.GomuxPane  // Alacritty FFI handle
	pty    *PTY          // Go PTY with shell process
	parent *actor.Ref
	self   *actor.Ref
}

// New creates terminal with shell using Go PTY + Alacritty Grid
func New(id uint32, rows, cols int, shell string, parent *actor.Ref) *Term {
	// Create Alacritty Grid
	cShell := C.CString(shell)
	defer C.free(unsafe.Pointer(cShell))
	
	alacrittyTerm := C.gomux_pane_new(C.uint(rows), C.uint(cols), cShell)
	if alacrittyTerm == nil {
		return nil
	}
	
	// Create Go PTY with shell
	cmd := exec.Command(shell)
	pty, err := Start(cmd)
	if err != nil {
		C.gomux_pane_free(alacrittyTerm)
		return nil
	}
	
	return &Term{
		id:     id,
		term:   alacrittyTerm,
		pty:    pty,
		parent: parent,
	}
}

// Spawn creates and spawns a Term with PTY read loop
func Spawn(id uint32, rows, cols int, shell string, parent *actor.Ref) *actor.Ref {
	t := New(id, rows, cols, shell, parent)
	if t == nil {
		return nil
	}
	ref := actor.Spawn(t, 10)
	t.self = ref
	
	// Start PTY read loop in goroutine
	go t.readLoop()
	
	return ref
}

// readLoop reads from PTY and feeds bytes to Alacritty
func (t *Term) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := t.pty.TTY.Read(buf)
		if err != nil {
			// PTY closed or error
			t.notifyExited()
			return
		}
		if n > 0 {
			// Feed bytes to Alacritty FFI
			C.gomux_pane_write(t.term, (*C.char)(unsafe.Pointer(&buf[0])), C.uint(n))
			// Notify parent that content changed
			if t.parent != nil {
				t.parent.Send(GridUpdated{ID: t.id})
			}
		}
	}
}

func (t *Term) notifyExited() {
	if t.parent != nil {
		t.parent.Send(TermExited{ID: t.id})
	}
}

func (t *Term) Receive(msg any) {
	switch m := msg.(type) {
	case WriteToTerm:
		// Write user input directly to PTY
		t.pty.TTY.Write([]byte(m.Data))
	case KillTerm:
		t.pty.Close()
		C.gomux_pane_free(t.term)
		if t.parent != nil {
			t.parent.Send(TermExited{ID: t.id})
		}
	case actor.AskEnvelope:
		t.handleAsk(m)
	}
}

func (t *Term) handleAsk(envelope actor.AskEnvelope) {
	switch envelope.Msg.(type) {
	case GetTermContent:
		// Process any pending bytes (though readLoop handles most)
		C.gomux_pane_tick(t.term)
		
		content := &TermContent{
			Lines: make([]string, 24), // TODO: get actual size from Alacritty
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

// MarkRendered resets dirty flag (optional optimization)
func (t *Term) MarkRendered() {
	C.gomux_pane_mark_rendered(t.term)
}

// Resize updates terminal size
func (t *Term) Resize(rows, cols int) {
	// TODO: Resize Alacritty grid and PTY
	_ = rows
	_ = cols
}
