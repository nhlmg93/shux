//go:build !ghostty
// +build !ghostty

package gomux

// Default: Pure Go terminal using tonistiigi/vt100
// Build normally: go build ./cmd/gomux
//
// Provides basic VT100 emulation:
// - Cursor movement
// - 16/256 colors  
// - Basic escape sequences
// - No scrollback
// - Good for shells, basic apps

import (
	"os/exec"

	"github.com/nhlmg93/gotor/actor"
	"github.com/tonistiigi/vt100"
)

// Term wraps vt100 Go library + Go PTY
type Term struct {
	id     uint32
	vt     *vt100.VT100  // Pure Go VT100 terminal
	pty    *PTY          // Go PTY with shell process
	parent *actor.Ref
	self   *actor.Ref
}

// New creates terminal with shell using Go PTY + Go VT100
func New(id uint32, rows, cols int, shell string, parent *actor.Ref) *Term {
	// Create Go VT100 terminal
	vt := vt100.NewVT100(rows, cols)
	if vt == nil {
		return nil
	}
	
	// Create Go PTY with shell
	cmd := exec.Command(shell)
	pty, err := Start(cmd)
	if err != nil {
		return nil
	}
	
	return &Term{
		id:     id,
		vt:     vt,
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

// readLoop reads from PTY and feeds bytes to VT100
func (t *Term) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := t.pty.TTY.Read(buf)
		if err != nil {
			t.notifyExited()
			return
		}
		if n > 0 {
			// Feed bytes to VT100 parser
			t.vt.Write(buf[:n])
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
			Lines: make([]string, t.vt.Height),
		}
		
		// Convert VT100 Content ([][]rune) to []string
		for row := 0; row < t.vt.Height; row++ {
			line := make([]rune, t.vt.Width)
			for col := 0; col < t.vt.Width; col++ {
				if col < len(t.vt.Content[row]) {
					line[col] = t.vt.Content[row][col]
				} else {
					line[col] = ' '
				}
			}
			content.Lines[row] = string(line)
		}
		
		// Get cursor position
		content.CursorRow = t.vt.Cursor.Y
		content.CursorCol = t.vt.Cursor.X
		
		envelope.Reply <- content
	default:
		envelope.Reply <- nil
	}
}

// Resize updates terminal size
func (t *Term) Resize(rows, cols int) {
	t.vt.Resize(rows, cols)
}
