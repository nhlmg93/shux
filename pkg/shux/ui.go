package shux

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nhlmg93/gotor/actor"
)

type Model struct {
	session     *actor.Ref
	width       int
	height      int
	content     []string
	prefixMode  bool
	cursorRow   int
	cursorCol   int
	initialized bool
}

func (m Model) Init() tea.Cmd {
	return m.listenForUpdates()
}

func (m Model) listenForUpdates() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(100 * time.Millisecond)
		return updateMsg{}
	}
}

type updateMsg struct{}

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
		return m, m.listenForUpdates()

	case tea.KeyMsg:
		if m.handleKey(msg) {
			return m, tea.Quit
		}
		return m, m.listenForUpdates()

	case updateMsg:
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
		return m, m.listenForUpdates()

	default:
		return m, m.listenForUpdates()
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
		return "Loading..."
	}

	// Always render full UI dimensions (m.width x m.height)
	// Content may differ during resize - stretch/compress to fit
	width, height := m.width, m.height
	contentWidth := 0
	if len(content.Lines) > 0 && len(content.Lines[0]) > 0 {
		contentWidth = len(content.Lines[0])
	}
	Debugf("ui: View() ui=%dx%d content=%dx%d", width, height, len(content.Lines), contentWidth)
	var output strings.Builder

	for i := 0; i < height; i++ {
		var row string
		if i < len(content.Lines) {
			row = content.Lines[i]
			if len(row) > width {
				row = row[:width]
			}
			// Pad short rows to full UI width
			if len(row) < width {
				row += strings.Repeat(" ", width-len(row))
			}
			// Draw cursor
			if i == content.CursorRow && content.CursorCol < len(row) {
				cursorChar := "█"
				row = row[:content.CursorCol] + cursorChar + row[content.CursorCol+1:]
			}
		} else {
			// Empty row beyond content - fill with spaces
			row = strings.Repeat(" ", width)
		}
		output.WriteString(row)
		if i < height-1 {
			output.WriteString("\n")
		}
	}

	if m.prefixMode {
		return output.String() + "\n[prefix]"
	}
	return output.String()
}

func NewModel(session *actor.Ref) Model {
	return Model{
		session: session,
		content: make([]string, 0),
	}
}

func RunUI(session *actor.Ref) {
	p := tea.NewProgram(NewModel(session), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
		os.Exit(1)
	}
}
