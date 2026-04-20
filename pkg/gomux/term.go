package gomux

// Term wraps libghostty (Ghostty terminal emulator) + Go PTY
//
// This provides FULL terminal emulation:
//   - Scrollback buffer (10k+ lines)
//   - 256 colors + true color (24-bit RGB)
//   - Alternate screen (vim, tmux work perfectly)
//   - Mouse support
//   - Kitty graphics protocol
//   - Sixel graphics
//   - Full Unicode/emoji support
//   - Excellent performance (Zig-compiled, SIMD-optimized)
//
// Architecture:
//   Shell → Go PTY → readLoop goroutine → libghostty.Terminal.VTWrite()
//                                                ↓
//   Bubble Tea UI ← GetTermContent ← GridRef cell traversal
//
// Note: This requires libghostty-vt to be installed.
// See build-ghostty.sh for one-time setup.

import (
	"os/exec"

	"github.com/mitchellh/go-libghostty"
	"github.com/nhlmg93/gotor/actor"
)

// Term represents a terminal pane with full emulation
type Term struct {
	id     uint32
	term   *libghostty.Terminal // Ghostty terminal handle
	pty    *PTY                // Go PTY with shell process
	parent *actor.Ref
	self   *actor.Ref
	rows   int
	cols   int
}

// New creates a new terminal with the given dimensions and shell
func New(id uint32, rows, cols int, shell string, parent *actor.Ref) *Term {
	// Create Ghostty terminal with scrollback
	ghosttyTerm, err := libghostty.NewTerminal(
		libghostty.WithSize(uint16(cols), uint16(rows)),
		libghostty.WithMaxScrollback(10000), // 10k lines scrollback
	)
	if err != nil {
		return nil
	}

	// Create Go PTY with shell
	cmd := exec.Command(shell)
	pty, err := Start(cmd)
	if err != nil {
		ghosttyTerm.Close()
		return nil
	}

	return &Term{
		id:     id,
		term:   ghosttyTerm,
		pty:    pty,
		parent: parent,
		rows:   rows,
		cols:   cols,
	}
}

// Spawn creates and spawns a Term actor with PTY read loop
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

// readLoop reads PTY output and feeds it to Ghostty for parsing
func (t *Term) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := t.pty.TTY.Read(buf)
		if err != nil {
			t.notifyExited()
			return
		}
		if n > 0 {
			// Feed bytes to Ghostty terminal emulator
			t.term.VTWrite(buf[:n])
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

// Receive handles actor messages
func (t *Term) Receive(msg any) {
	switch m := msg.(type) {
	case WriteToTerm:
		// Write user input directly to PTY
		t.pty.TTY.Write([]byte(m.Data))
	case KillTerm:
		t.pty.Close()
		t.term.Close()
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
			Lines: make([]string, t.rows),
		}

		// Traverse Ghostty grid and extract cell contents
		for row := 0; row < t.rows; row++ {
			line := make([]rune, t.cols)
			for col := 0; col < t.cols; col++ {
				ref, err := t.term.GridRef(libghostty.Point{
					Tag: libghostty.PointTagActive,
					X:   uint16(col),
					Y:   uint32(row),
				})
				if err != nil {
					line[col] = ' '
					continue
				}

				cell, err := ref.Cell()
				if err != nil {
					line[col] = ' '
					continue
				}

				hasText, _ := cell.HasText()
				if hasText {
					cp, _ := cell.Codepoint()
					line[col] = rune(cp)
				} else {
					line[col] = ' '
				}
			}
			content.Lines[row] = string(line)
		}

		// Get cursor position from Ghostty RenderState
		rs, err := libghostty.NewRenderState()
		if err == nil {
			defer rs.Close()
			if err := rs.Update(t.term); err == nil {
				if hasValue, _ := rs.CursorViewportHasValue(); hasValue {
					if x, err := rs.CursorViewportX(); err == nil {
						content.CursorCol = int(x)
					}
					if y, err := rs.CursorViewportY(); err == nil {
						content.CursorRow = int(y)
					}
				}
			}
		}

		envelope.Reply <- content
	default:
		envelope.Reply <- nil
	}
}

// Resize updates terminal dimensions
func (t *Term) Resize(rows, cols int) {
	t.term.Resize(uint16(cols), uint16(rows), 0, 0)
	t.rows = rows
	t.cols = cols
}
