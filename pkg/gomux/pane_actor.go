package gomux

import (
	"os/exec"

	"github.com/nhlmg93/gotor/actor"
)

// PaneActor manages a single PTY and its process
type PaneActor struct {
	id     uint32
	pty    *PTY
	parent *actor.Ref
	self   *actor.Ref
	grid   *Grid
}

// Receive handles messages for the pane
func (p *PaneActor) Receive(msg any) {
	switch m := msg.(type) {
	case WriteToPane:
		if p.pty != nil {
			p.pty.TTY.Write(m.Data)
		}
	case KillPane:
		if p.pty != nil {
			p.pty.Close()
		}
		p.notifyExited()
	case ResizeGrid:
		p.grid.Resize(m.Width, m.Height)
	case actor.AskEnvelope:
		p.handleAsk(m)
	}
}

func (p *PaneActor) handleAsk(envelope actor.AskEnvelope) {
	switch envelope.Msg.(type) {
	case GetGrid:
		envelope.Reply <- p.grid
	default:
		envelope.Reply <- nil
	}
}

func (p *PaneActor) GetRef() *actor.Ref {
	return p.self
}

func (p *PaneActor) notifyExited() {
	if p.parent != nil {
		p.parent.Send(PaneExited{ID: p.id})
	}
}

func SpawnPaneActor(id uint32, cmd *exec.Cmd, parent *actor.Ref) (*actor.Ref, error) {
	pty, err := Start(cmd)
	if err != nil {
		return nil, err
	}

	pane := &PaneActor{
		id:     id,
		pty:    pty,
		parent: parent,
		grid:   NewGrid(80, 24),
	}

	ref := actor.Spawn(pane, 10)
	pane.self = ref
	go pane.readLoop()

	return ref, nil
}

func (p *PaneActor) readLoop() {
	buf := make([]byte, 1024)
	for {
		n, err := p.pty.TTY.Read(buf)
		if err != nil {
			p.notifyExited()
			return
		}
		p.processBytes(buf[:n])
		if p.parent != nil {
			p.parent.Send(GridUpdated{ID: p.id})
		}
	}
}

func (p *PaneActor) processBytes(data []byte) {
	inEscape := false
	var escBuf []byte
	for _, b := range data {
		if inEscape {
			escBuf = append(escBuf, b)
			// Check if sequence is complete
			if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == 0x07 || b == 0x9c {
				inEscape = false
				// Handle specific escape sequences
				if len(escBuf) >= 2 && escBuf[len(escBuf)-1] == 'J' {
					// ESC[...J - clear screen (2J = clear all, 0J = clear to end, 1J = clear to start)
					p.grid.Clear()
					p.grid.CursorX = 0
					p.grid.CursorY = 0
				}
				escBuf = nil
			}
			continue
		}
		switch b {
		case '\r':
			p.grid.CursorX = 0
		case '\n':
			p.grid.NewLine()
		case 0x08: // BS - backspace
			if p.grid.CursorX > 0 {
				p.grid.CursorX--
				p.grid.Cells[p.grid.CursorY][p.grid.CursorX].Char = ' '
			}
		case 0x03: // Ctrl+C - interrupt
			// Interrupt not visual, handled by PTY/process
		case 0x0c: // Ctrl+L - form feed, clear screen
			p.grid.Clear()
			p.grid.CursorX = 0
			p.grid.CursorY = 0
		case 0x1b: // ESC - start escape sequence
			inEscape = true
			escBuf = []byte{b}
		default:
			if b >= 32 && b < 127 {
				p.grid.WriteChar(rune(b))
			}
		}
	}
}
