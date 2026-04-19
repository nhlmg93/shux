package gomux

/*
#cgo CFLAGS: -I${SRCDIR}/../../gomux-term/build
#cgo LDFLAGS: ${SRCDIR}/../../gomux-term/build/libgomux_term.a
#include <stdlib.h>
#include "gomux_term.h"
*/
import "C"
import (
	"os/exec"
	"unsafe"

	"github.com/nhlmg93/gotor/actor"
)

// Term wraps simple C terminal emulator + Go PTY
type Term struct {
	id     uint32
	term   C.GomuxTerm  // C handle
	pty    *PTY          // Go PTY with shell process
	parent *actor.Ref
	self   *actor.Ref
}

// New creates terminal with shell using Go PTY + C terminal emulator
func New(id uint32, rows, cols int, shell string, parent *actor.Ref) *Term {
	// Create C terminal
	cTerm := C.gomux_term_new(C.uint(rows), C.uint(cols))
	if cTerm == nil {
		return nil
	}
	
	// Create Go PTY with shell
	cmd := exec.Command(shell)
	pty, err := Start(cmd)
	if err != nil {
		C.gomux_term_free(cTerm)
		return nil
	}
	
	return &Term{
		id:     id,
		term:   cTerm,
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

// readLoop reads from PTY and feeds bytes to C terminal
func (t *Term) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := t.pty.TTY.Read(buf)
		if err != nil {
			t.notifyExited()
			return
		}
		if n > 0 {
			// Feed bytes to C terminal
			C.gomux_term_process(t.term, (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(n))
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
		C.gomux_term_free(t.term)
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

// MarkRendered resets dirty flag
func (t *Term) MarkRendered() {
	C.gomux_pane_mark_rendered(t.term)
}

// Resize updates terminal size
func (t *Term) Resize(rows, cols int) {
	C.gomux_term_resize(t.term, C.uint(rows), C.uint(cols))
}
