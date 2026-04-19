package gomux

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nhlmg93/gotor/actor"
)

// UIMsg wraps actor messages for Bubble Tea
type UIMsg struct {
	Msg any
}

// Model implements tea.Model for the gomux UI
type Model struct {
	session    *actor.Ref
	uiChan     chan tea.Msg
	width      int
	height     int
	style      lipgloss.Style
	prefixMode bool
}

// NewModel creates a new UI model with the given session and UI channel
func NewModel(session *actor.Ref, uiChan chan tea.Msg) Model {
	return Model{
		session: session,
		uiChan:  uiChan,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	// Start listening first, then create window
	return m.listenForUpdates()
}

func (m Model) listenForUpdates() tea.Cmd {
	return func() tea.Msg {
		msg := <-m.uiChan
		return msg
	}
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.session != nil {
			m.session.Send(ResizeGrid{Width: m.width, Height: m.height})
		}
		// Create window on first resize
		return m, tea.Batch(
			func() tea.Msg {
				if m.session != nil {
					m.session.Send(CreateWindow{})
				}
				return nil
			},
			m.listenForUpdates(),
		)
	case UIMsg:
		// Actor message received - re-render and listen for more
		return m, m.listenForUpdates()
	case tea.KeyMsg:
		if m.handleKey(msg) {
			return m, tea.Quit
		}
		return m, m.listenForUpdates()
	}
	return m, m.listenForUpdates()
}

// handleKey processes key input. Returns true if should quit.
func (m *Model) handleKey(key tea.KeyMsg) bool {
	if m.prefixMode {
		m.prefixMode = false
		switch key.String() {
		case "q":
			return true
		}
		// Unknown prefix command - send prefix+key to term
		m.sendToTerm([]byte{1}) // Ctrl+A
		m.sendKeyToTerm(key)
		return false
	}
	if key.Type == tea.KeyCtrlA {
		m.prefixMode = true
		return false
	}
	// Normal key - forward to term
	m.sendKeyToTerm(key)
	return false
}

func (m *Model) sendKeyToTerm(key tea.KeyMsg) {
	switch key.Type {
	case tea.KeyEnter:
		m.sendToTerm([]byte{'\r'})
	case tea.KeyBackspace:
		// Send BS (0x08) to shell
		m.sendToTerm([]byte{0x08})
	case tea.KeyCtrlC:
		// Send Ctrl+C (0x03) to shell
		m.sendToTerm([]byte{0x03})
	case tea.KeyCtrlL:
		// Send Ctrl+L (0x0C) to shell
		m.sendToTerm([]byte{0x0c})
	default:
		if len(key.Runes) > 0 {
			m.sendToTerm([]byte(string(key.Runes)))
		}
	}
}



func (m *Model) sendToTerm(data []byte) {
	reply := m.session.Ask(GetActiveTerm{})
	termRef := <-reply
	if termRef != nil {
		termRef.(*actor.Ref).Send(WriteToTerm{Data: data})
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
		return "No window - press ctrl+a then w to create one"
	}

	content := result.(*TermContent)
	if content == nil {
		return "Loading..."
	}

	// Render term content with cursor
	var output string
	maxRows := height
	if maxRows > len(content.Lines) {
		maxRows = len(content.Lines)
	}
	for i := 0; i < maxRows; i++ {
		row := content.Lines[i]
		if len(row) > width {
			row = row[:width]
		}
		// Add cursor on cursor row
		if i == content.CursorRow && content.CursorCol < len(row) {
			cursorChar := "█"
			row = row[:content.CursorCol] + cursorChar + row[content.CursorCol+1:]
		}
		output += row + "\n"
	}

	// Show prefix mode indicator
	if m.prefixMode {
		return output + "[prefix]"
	}
	return output
}

// SubscribeToGridUpdates creates a command that listens for GridUpdated messages
func SubscribeToGridUpdates(session *actor.Ref) tea.Cmd {
	return func() tea.Msg {
		// This would need a proper subscription mechanism
		// For now, return nil
		return nil
	}
}
