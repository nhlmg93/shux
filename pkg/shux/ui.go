package shux

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nhlmg93/gotor/actor"
)

// Package-level update channel for actor→UI communication
var uiUpdateCh chan struct{}

// SetUpdateChannel sets the channel actors use to notify UI of content changes
func SetUpdateChannel(ch chan struct{}) {
	uiUpdateCh = ch
}

type Model struct {
	session       *actor.Ref
	width         int
	height        int
	content       []string
	prefixMode    bool
	cursorRow     int
	cursorCol     int
	initialized   bool
	updateCh      chan struct{} // Receives updates from actor system
}

func (m Model) Init() tea.Cmd {
	return m.waitForUpdate()
}

// waitForUpdate blocks until the actor system notifies us of new content
func (m Model) waitForUpdate() tea.Cmd {
	return func() tea.Msg {
		<-m.updateCh
		return UpdateMsg{}
	}
}

// UpdateMsg signals that pane content has changed and UI should redraw
type UpdateMsg struct{}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		Infof("ui: window size %dx%d", msg.Width, msg.Height)
		if !m.initialized {
			m.initialized = true
			Infof("ui: creating initial window %dx%d", msg.Height, msg.Width)
			m.session.Send(CreateWindow{Rows: msg.Height, Cols: msg.Width})
		} else {
			Infof("ui: resizing to %dx%d", msg.Height, msg.Width)
			m.session.Send(ResizeMsg{Rows: msg.Height, Cols: msg.Width})
		}
		return m, m.waitForUpdate()

	case tea.KeyMsg:
		if m.handleKey(msg) {
			return m, tea.Quit
		}
		// After key press, refresh content immediately
		return m, func() tea.Msg { return UpdateMsg{} }

	case UpdateMsg:
		reply := m.session.Ask(GetPaneContent{})
		result := <-reply
		if result != nil {
			content := result.(*PaneContent)
			if content != nil && len(content.Lines) > 0 {
				m.content = content.Lines
				m.cursorRow = content.CursorRow
				m.cursorCol = content.CursorCol
			}
		}
		return m, m.waitForUpdate()

	default:
		return m, m.waitForUpdate()
	}
}

func (m *Model) handleKey(key tea.KeyMsg) bool {
	if m.prefixMode {
		m.prefixMode = false
		switch key.String() {
		case "q":
			return true
		case "w":
			m.session.Send(CreateWindow{Rows: m.height, Cols: m.width})
			return false
		case "n":
			m.session.Send(SwitchWindow{Delta: 1})
			return false
		case "p":
			m.session.Send(SwitchWindow{Delta: -1})
			return false
		}
		m.sendToTerm([]byte{0x02})
		m.sendKeyToTerm(key)
		return false
	}

	if key.Type == tea.KeyCtrlB {
		m.prefixMode = true
		return false
	}

	m.sendKeyToTerm(key)
	return false
}

func (m *Model) sendKeyToTerm(key tea.KeyMsg) {
	var data []byte

	switch key.Type {
	case tea.KeyEnter:
		data = []byte{'\r'}
	case tea.KeyBackspace:
		data = []byte{0x7F}
	case tea.KeyTab:
		data = []byte{0x09}
	case tea.KeyEsc:
		data = []byte{0x1B}
	case tea.KeySpace:
		data = []byte{' '}
	case tea.KeyUp:
		data = []byte{0x1B, '[', 'A'}
	case tea.KeyDown:
		data = []byte{0x1B, '[', 'B'}
	case tea.KeyRight:
		data = []byte{0x1B, '[', 'C'}
	case tea.KeyLeft:
		data = []byte{0x1B, '[', 'D'}
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
		data = []byte{0x08}
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
	default:
		if key.Alt {
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
	reply := m.session.Ask(GetActivePane{})
	paneRef := <-reply
	if paneRef != nil {
		paneRef.(*actor.Ref).Send(WriteToPane{Data: data})
	}
}

func (m Model) View() string {
	if m.session == nil {
		return "Error: no session"
	}
	if m.width == 0 || m.height == 0 {
		return "Error: window size not received"
	}
	if !m.initialized {
		return "Error: initialization failed"
	}

	reply := m.session.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		return "Error: no active window"
	}

	content := result.(*PaneContent)
	if content == nil {
		return ""
	}

	// Always render full UI dimensions (m.width x m.height)
	// Content may differ during resize - stretch/compress to fit
	width, height := m.width, m.height
	var rows []string

	for i := 0; i < height; i++ {
		var rowBuilder strings.Builder
		
		if i < len(content.Lines) && i < len(content.Cells) {
			cells := content.Cells[i]
			
			for j := 0; j < width && j < len(cells); j++ {
				cell := cells[j]
				char := string(cell.Char)
				
				// Draw cursor as █ at the cursor position
				if !content.CursorHidden && i == content.CursorRow && j == content.CursorCol {
					char = "█"
				}
				
				// For now, skip styling to debug UTF-8 rendering
				_ = lipgloss.NewStyle() // Keep import
				rowBuilder.WriteString(char)
			}
			
			// Pad remaining cells in row
			for j := len(cells); j < width; j++ {
				rowBuilder.WriteString(" ")
			}
		} else {
			// Empty row
			rowBuilder.WriteString(strings.Repeat(" ", width))
		}
		
		rows = append(rows, rowBuilder.String())
	}
	
	output := strings.Join(rows, "\n")

	if m.prefixMode {
		return output + "\n[prefix]"
	}
	return output
}

func NewModel(session *actor.Ref, updateCh chan struct{}) Model {
	return Model{
		session: session,
		content: make([]string, 0),
		updateCh: updateCh,
	}
}

func RunUI(session *actor.Ref, updateCh chan struct{}) {
	p := tea.NewProgram(NewModel(session, updateCh), tea.WithAltScreen())
	
	// Goroutine to wake Bubble Tea when actors notify us
	go func() {
		for range updateCh {
			p.Send(UpdateMsg{})
		}
	}()
	
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
		os.Exit(1)
	}
}
