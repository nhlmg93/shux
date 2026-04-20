package gomux

// Term wraps libghostty (Ghostty terminal emulator) + Go PTY
//
// Leverages libghostty features:
//   - Full VT220/xterm emulation
//   - 256 colors + true color (24-bit RGB)
//   - Scrollback buffer
//   - Mouse support
//   - Kitty graphics protocol
//   - Unicode/emoji
//   - Hyperlinks (OSC 8)
//   - Terminal effects (bell, title changes)
//   - Cursor position tracking
//   - Cell styling (bold, italic, colors)
//   - Mode queries (alt screen, cursor visible)

import (
	"os/exec"

	"github.com/mitchellh/go-libghostty"
	"github.com/nhlmg93/gotor/actor"
)

// TermCell represents a cell with full styling
type TermCell struct {
	Char      rune
	FgColor   libghostty.ColorRGB
	BgColor   libghostty.ColorRGB
	Bold      bool
	Italic    bool
	Underline bool
	Blink     bool
	Reverse   bool
	HasHyperlink bool
	HyperlinkURL string
}

// Term represents a terminal pane with full emulation and styling
type Term struct {
	id          uint32
	term        *libghostty.Terminal // Ghostty terminal handle
	renderState *libghostty.RenderState // Cached render state for performance
	pty         *PTY                     // Go PTY with shell process
	parent      *actor.Ref
	self        *actor.Ref
	rows        int
	cols        int
	windowTitle string // Track terminal title from shell
}

// New creates a new terminal with the given dimensions and shell
func New(id uint32, rows, cols int, shell string, parent *actor.Ref) *Term {
	// Create Ghostty terminal with full feature set
	ghosttyTerm, err := libghostty.NewTerminal(
		libghostty.WithSize(uint16(cols), uint16(rows)),
		libghostty.WithMaxScrollback(10000),
		// Handle terminal title changes (shell sets title via OSC)
		libghostty.WithTitleChanged(func(t *libghostty.Terminal) {
			// Could emit message to update window title
			// For now, we just track that it changed
		}),
		// Handle bell (visual/audible notification)
		libghostty.WithBell(func(t *libghostty.Terminal) {
			// Could flash screen or play sound
		}),
	)
	if err != nil {
		return nil
	}

	// Create cached render state for cursor/styling queries
	renderState, err := libghostty.NewRenderState()
	if err != nil {
		ghosttyTerm.Close()
		return nil
	}

	// Create Go PTY with shell
	cmd := exec.Command(shell)
	pty, err := Start(cmd)
	if err != nil {
		ghosttyTerm.Close()
		renderState.Close()
		return nil
	}

	return &Term{
		id:          id,
		term:        ghosttyTerm,
		renderState: renderState,
		pty:         pty,
		parent:      parent,
		rows:        rows,
		cols:        cols,
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
		if t.renderState != nil {
			t.renderState.Close()
		}
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
			Cells: make([][]TermCell, t.rows),
		}

		// Update render state once per frame
		t.renderState.Update(t.term)

		// Get cursor position from cached RenderState
		if hasValue, _ := t.renderState.CursorViewportHasValue(); hasValue {
			if x, err := t.renderState.CursorViewportX(); err == nil {
				content.CursorCol = int(x)
			}
			if y, err := t.renderState.CursorViewportY(); err == nil {
				content.CursorRow = int(y)
			}
		}

		// Traverse Ghostty grid with full styling
		for row := 0; row < t.rows; row++ {
			line := make([]rune, t.cols)
			cells := make([]TermCell, t.cols)

			for col := 0; col < t.cols; col++ {
				cell := t.getCell(row, col)
				cells[col] = cell
				line[col] = cell.Char
			}
			content.Lines[row] = string(line)
			content.Cells[row] = cells
		}

		// Check if we're in alternate screen (vim, less, etc.)
		if altScreen, _ := t.term.ModeGet(libghostty.ModeAltScreen); altScreen {
			content.InAltScreen = true
		}

		// Check cursor visibility
		if cursorVisible, _ := t.term.ModeGet(libghostty.ModeCursorVisible); !cursorVisible {
			content.CursorHidden = true
		}

		envelope.Reply <- content
	default:
		envelope.Reply <- nil
	}
}

// getCell retrieves a cell with full styling information
func (t *Term) getCell(row, col int) TermCell {
	ref, err := t.term.GridRef(libghostty.Point{
		Tag: libghostty.PointTagActive,
		X:   uint16(col),
		Y:   uint32(row),
	})
	if err != nil {
		return TermCell{Char: ' '}
	}

	cell, err := ref.Cell()
	if err != nil {
		return TermCell{Char: ' '}
	}

	result := TermCell{Char: ' '}

	// Get character
	hasText, _ := cell.HasText()
	if hasText {
		cp, _ := cell.Codepoint()
		result.Char = rune(cp)
	}

	// Get styling
	style, err := ref.Style()
	if err == nil {
		result.Bold = style.Bold()
		result.Italic = style.Italic()
		result.Underline = style.Underline() != libghostty.UnderlineNone
		result.Blink = style.Blink()
		result.Reverse = style.Inverse()

		// Get colors
		fgColor := style.FgColor()
		if fgColor.Tag == libghostty.StyleColorRGB {
			result.FgColor = fgColor.RGB
		}

		bgColor := style.BgColor()
		if bgColor.Tag == libghostty.StyleColorRGB {
			result.BgColor = bgColor.RGB
		}
	}

	// Check for hyperlinks
	result.HasHyperlink, _ = cell.HasHyperlink()
	// Note: Getting actual URL requires additional API calls

	return result
}

// Resize updates terminal dimensions
func (t *Term) Resize(rows, cols int) {
	t.term.Resize(uint16(cols), uint16(rows), 0, 0)
	t.rows = rows
	t.cols = cols
}

// IsAltScreen returns true if terminal is in alternate screen (vim, less, etc.)
func (t *Term) IsAltScreen() bool {
	alt, _ := t.term.ModeGet(libghostty.ModeAltScreen)
	return alt
}

// IsCursorVisible returns true if cursor should be visible
func (t *Term) IsCursorVisible() bool {
	visible, _ := t.term.ModeGet(libghostty.ModeCursorVisible)
	return visible
}
