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
		// Unknown prefix command - send prefix+key to pane
		m.sendToPane([]byte{1}) // Ctrl+A
		m.sendKeyToPane(key)
		return false
	}
	if key.Type == tea.KeyCtrlA {
		m.prefixMode = true
		return false
	}
	// Normal key - forward to pane
	m.sendKeyToPane(key)
	return false
}

func (m *Model) sendKeyToPane(key tea.KeyMsg) {
	switch key.Type {
	case tea.KeyEnter:
		m.sendToPane([]byte{'\r'})
	case tea.KeyBackspace:
		// Send BS (0x08) to shell
		m.sendToPane([]byte{0x08})
	case tea.KeyCtrlC:
		// Send Ctrl+C (0x03) to shell
		m.sendToPane([]byte{0x03})
	case tea.KeyCtrlL:
		// Send Ctrl+L (0x0C) to shell
		m.sendToPane([]byte{0x0c})
	default:
		if len(key.Runes) > 0 {
			m.sendToPane([]byte(string(key.Runes)))
		}
	}
}



func (m *Model) sendToPane(data []byte) {
	reply := m.session.Ask(GetActivePane{})
	paneRef := <-reply
	if paneRef != nil {
		paneRef.(*actor.Ref).Send(WriteToPane{Data: data})
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

	// Get grid from active pane through the chain
	reply := m.session.Ask(GetGrid{})
	result := <-reply
	if result == nil {
		return "No window - press ctrl+a then w to create one"
	}

	grid := result.(*Grid)

	// Render grid content with cursor, clipped to terminal size
	var content string
	maxRows := height
	if maxRows > grid.Height {
		maxRows = grid.Height
	}
	for i := 0; i < maxRows; i++ {
		row := grid.GetRow(i)
		if len(row) > width {
			row = row[:width]
		}
		// Add cursor on cursor row (use visible block character)
		if i == grid.CursorY && grid.CursorX < len(row) {
			cursorChar := "█"
			row = row[:grid.CursorX] + cursorChar + row[grid.CursorX+1:]
		}
		content += row + "\n"
	}

	// Show prefix mode indicator
	if m.prefixMode {
		return content + "[prefix]"
	}
	return content
}

// SubscribeToGridUpdates creates a command that listens for GridUpdated messages
func SubscribeToGridUpdates(session *actor.Ref) tea.Cmd {
	return func() tea.Msg {
		// This would need a proper subscription mechanism
		// For now, return nil
		return nil
	}
}
