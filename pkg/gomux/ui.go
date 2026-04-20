package gomux

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nhlmg93/gotor/actor"
)

// Model implements tea.Model for the gomux TUI
type Model struct {
	session     *actor.Ref
	width       int
	height      int
	content     []string
	prefixMode  bool
	cursorRow   int
	cursorCol   int
	initialized bool // true after first window size received
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return m.listenForUpdates()
}

// listenForUpdates returns a command that listens for actor updates
func (m Model) listenForUpdates() tea.Cmd {
	return func() tea.Msg {
		// This is a placeholder - in a real implementation,
		// we'd set up a channel to receive updates from the actor system
		// For now, just poll periodically
		time.Sleep(100 * time.Millisecond)
		return updateMsg{}
	}
}

// updateMsg is sent when the UI should refresh
type updateMsg struct{}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.initialized {
			// First size message - create initial window with correct size
			m.initialized = true
			m.session.Send(CreateWindowWithSize{Rows: msg.Height, Cols: msg.Width})
		} else {
			// Subsequent resize - resize active terminal
			m.resizeActiveTerm(msg.Height, msg.Width)
		}
		return m, m.listenForUpdates()

	case tea.KeyMsg:
		if m.handleKey(msg) {
			return m, tea.Quit
		}
		return m, m.listenForUpdates()

	case updateMsg:
		// Refresh content from session
		reply := m.session.Ask(GetTermContent{})
		result := <-reply
		if result != nil {
			content := result.(*TermContent)
			if content != nil && len(content.Lines) > 0 {
				m.content = content.Lines
				m.cursorRow = content.CursorRow
				m.cursorCol = content.CursorCol
			}
		}
		return m, m.listenForUpdates()

	default:
		return m, m.listenForUpdates()
	}
}

// handleKey processes key input. Returns true if should quit.
func (m *Model) handleKey(key tea.KeyMsg) bool {
	if m.prefixMode {
		m.prefixMode = false
		switch key.String() {
		case "q":
			return true
		case "w":
			m.session.Send(CreateWindow{})
			return false
		case "n":
			m.session.Send(SwitchWindow{Delta: 1})
			return false
		case "p":
			m.session.Send(SwitchWindow{Delta: -1})
			return false
		}
		// Unknown prefix command - send prefix+key to term
		m.sendToTerm([]byte{0x02}) // Ctrl+B
		m.sendKeyToTerm(key)
		return false
	}

	// Check for prefix key (Ctrl+B)
	if key.Type == tea.KeyCtrlB {
		m.prefixMode = true
		return false
	}

	// Normal key - forward to term
	m.sendKeyToTerm(key)
	return false
}

// sendKeyToTerm converts key to appropriate byte sequence and sends to terminal
func (m *Model) sendKeyToTerm(key tea.KeyMsg) {
	var data []byte

	switch key.Type {
	// Special keys
	case tea.KeyEnter:
		data = []byte{'\r'}
	case tea.KeyBackspace:
		data = []byte{0x7F} // DEL (most modern terminals)
	case tea.KeyTab:
		data = []byte{0x09}
	case tea.KeyEsc:
		data = []byte{0x1B}
	case tea.KeySpace:
		data = []byte{' '}

	// Arrow keys - CSI sequences
	case tea.KeyUp:
		data = []byte{0x1B, '[', 'A'}
	case tea.KeyDown:
		data = []byte{0x1B, '[', 'B'}
	case tea.KeyRight:
		data = []byte{0x1B, '[', 'C'}
	case tea.KeyLeft:
		data = []byte{0x1B, '[', 'D'}

	// Navigation keys
	case tea.KeyHome:
		data = []byte{0x1B, '[', 'H'}
	case tea.KeyEnd:
		data = []byte{0x1B, '[', 'F'}
	case tea.KeyPgUp:
		data = []byte{0x1B, '[', '5', '~'}
	case tea.KeyPgDown:
		data = []byte{0x1B, '[', '6', '~'}
	case tea.KeyDelete:
		data = []byte{0x1B, '[', '3', '~'}
	case tea.KeyInsert:
		data = []byte{0x1B, '[', '2', '~'}

	// Function keys
	case tea.KeyF1:
		data = []byte{0x1B, 'O', 'P'}
	case tea.KeyF2:
		data = []byte{0x1B, 'O', 'Q'}
	case tea.KeyF3:
		data = []byte{0x1B, 'O', 'R'}
	case tea.KeyF4:
		data = []byte{0x1B, 'O', 'S'}
	case tea.KeyF5:
		data = []byte{0x1B, '[', '1', '5', '~'}
	case tea.KeyF6:
		data = []byte{0x1B, '[', '1', '7', '~'}
	case tea.KeyF7:
		data = []byte{0x1B, '[', '1', '8', '~'}
	case tea.KeyF8:
		data = []byte{0x1B, '[', '1', '9', '~'}
	case tea.KeyF9:
		data = []byte{0x1B, '[', '2', '0', '~'}
	case tea.KeyF10:
		data = []byte{0x1B, '[', '2', '1', '~'}
	case tea.KeyF11:
		data = []byte{0x1B, '[', '2', '3', '~'}
	case tea.KeyF12:
		data = []byte{0x1B, '[', '2', '4', '~'}

	// Ctrl+Letter (ASCII control codes 1-26, except Ctrl+B which is prefix)
	// Note: Ctrl+I (Tab), Ctrl+J (LF), Ctrl+M (CR) already handled above
	case tea.KeyCtrlA:
		data = []byte{0x01}
	case tea.KeyCtrlC:
		data = []byte{0x03}
	case tea.KeyCtrlD:
		data = []byte{0x04}
	case tea.KeyCtrlE:
		data = []byte{0x05}
	case tea.KeyCtrlF:
		data = []byte{0x06}
	case tea.KeyCtrlG:
		data = []byte{0x07}
	case tea.KeyCtrlH:
		data = []byte{0x08} // Same as Backspace in some terminals
	case tea.KeyCtrlK:
		data = []byte{0x0B}
	case tea.KeyCtrlL:
		data = []byte{0x0C}
	case tea.KeyCtrlN:
		data = []byte{0x0E}
	case tea.KeyCtrlO:
		data = []byte{0x0F}
	case tea.KeyCtrlP:
		data = []byte{0x10}
	case tea.KeyCtrlQ:
		data = []byte{0x11}
	case tea.KeyCtrlR:
		data = []byte{0x12}
	case tea.KeyCtrlS:
		data = []byte{0x13}
	case tea.KeyCtrlT:
		data = []byte{0x14}
	case tea.KeyCtrlU:
		data = []byte{0x15}
	case tea.KeyCtrlV:
		data = []byte{0x16}
	case tea.KeyCtrlW:
		data = []byte{0x17}
	case tea.KeyCtrlX:
		data = []byte{0x18}
	case tea.KeyCtrlY:
		data = []byte{0x19}
	case tea.KeyCtrlZ:
		data = []byte{0x1A}

	// Regular printable characters and Alt combinations
	default:
		if key.Alt {
			// Alt+key sends ESC followed by the key
			data = append([]byte{0x1B}, []byte(string(key.Runes))...)
		} else if len(key.Runes) > 0 {
			data = []byte(string(key.Runes))
		}
	}

	if len(data) > 0 {
		m.sendToTerm(data)
	}
}

func (m *Model) sendToTerm(data []byte) {
	reply := m.session.Ask(GetActiveTerm{})
	termRef := <-reply
	if termRef != nil {
		termRef.(*actor.Ref).Send(WriteToTerm{Data: data})
	}
}

// resizeActiveTerm sends resize message to the active terminal
func (m *Model) resizeActiveTerm(rows, cols int) {
	reply := m.session.Ask(GetActiveTerm{})
	termRef := <-reply
	if termRef != nil {
		termRef.(*actor.Ref).Send(ResizeTerm{Rows: rows, Cols: cols})
	}
}

// View implements tea.Model
func (m Model) View() string {
	if m.session == nil {
		return "Error: no session"
	}
	// Default size if not yet received WindowSizeMsg
	width, height := m.width, m.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	// Get content from active term through the chain
	reply := m.session.Ask(GetTermContent{})
	result := <-reply
	if result == nil {
		if !m.initialized {
			return "Starting..."
		}
		return "No window - press ctrl+b then w to create one"
	}

	content := result.(*TermContent)
	if content == nil {
		return "Loading..."
	}

	// Render term content with cursor
	var output strings.Builder
	maxRows := height
	if maxRows > len(content.Lines) {
		maxRows = len(content.Lines)
	}

	for i := 0; i < maxRows; i++ {
		row := content.Lines[i]
		if len(row) > width {
			row = row[:width]
		}
		// Add cursor on cursor row (use visible block character)
		if i == content.CursorRow && content.CursorCol < len(row) {
			cursorChar := "█"
			row = row[:content.CursorCol] + cursorChar + row[content.CursorCol+1:]
		}
		output.WriteString(row)
		if i < maxRows-1 {
			output.WriteString("\n")
		}
	}

	// Show prefix mode indicator
	if m.prefixMode {
		return output.String() + "\n[prefix]"
	}
	return output.String()
}

// NewModel creates a new UI model with the given session
func NewModel(session *actor.Ref) Model {
	return Model{
		session: session,
		content: make([]string, 0),
	}
}

// Run starts the Bubble Tea UI
func RunUI(session *actor.Ref) {
	p := tea.NewProgram(NewModel(session), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
		os.Exit(1)
	}
}
