package shux

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/nhlmg93/gotor/actor"
)

type Model struct {
	session     *actor.Ref
	width       int
	height      int
	prefixMode  bool
	initialized bool
	content     *PaneContent
}

// SetUpdateChannel is kept as a compatibility no-op for older tests.
func SetUpdateChannel(ch chan struct{}) {
	_ = ch
}

func (m Model) Init() tea.Cmd {
	return nil
}

// UpdateMsg signals that pane content changed and the UI should refresh.
type UpdateMsg struct{}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width <= 0 || msg.Height <= 0 {
			Infof("ui: ignoring invalid window size %dx%d", msg.Width, msg.Height)
			return m, nil
		}
		m.width = msg.Width
		m.height = msg.Height
		Infof("ui: window size %dx%d", msg.Width, msg.Height)
		if !m.initialized {
			m.initialized = true
			reply := m.session.Ask(GetActiveWindow{})
			existing := <-reply
			if existing == nil {
				Infof("ui: creating initial window %dx%d", msg.Height, msg.Width)
				m.session.Send(CreateWindow{Rows: msg.Height, Cols: msg.Width})
			} else {
				Infof("ui: resizing existing session to %dx%d", msg.Height, msg.Width)
				m.session.Send(ResizeMsg{Rows: msg.Height, Cols: msg.Width})
			}
		} else {
			Infof("ui: resizing to %dx%d", msg.Height, msg.Width)
			m.session.Send(ResizeMsg{Rows: msg.Height, Cols: msg.Width})
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.handleKey(msg) {
			return m, tea.Quit
		}
		return m, nil

	case tea.PasteMsg:
		m.session.Send(WriteToPane{Data: []byte(msg.Content)})
		return m, nil

	case UpdateMsg:
		reply := m.session.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			m.content = nil
			return m, nil
		}
		m.content = result.(*PaneContent)
		return m, nil

	default:
		return m, nil
	}
}

func (m *Model) handleKey(key tea.KeyPressMsg) bool {
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
		case "d":
			Infof("ui: detach requested")
			reply := m.session.Ask(DetachSession{})
			if err, _ := (<-reply).(error); err != nil {
				Warnf("ui: detach failed: %v", err)
				return false
			}
			Infof("ui: detach completed")
			return true
		}
		m.sendKeyInput(ctrlBInput())
		m.sendKey(key)
		return false
	}

	if key.String() == "ctrl+b" {
		m.prefixMode = true
		return false
	}

	m.sendKey(key)
	return false
}

func (m *Model) sendKey(key tea.KeyPressMsg) {
	input, ok := normalizeKeyInput(key)
	if !ok {
		return
	}
	m.sendKeyInput(input)
}

func (m *Model) sendKeyInput(input KeyInput) {
	m.session.Send(input)
}

func (m Model) View() tea.View {
	content := m.renderContent()
	v := tea.NewView(content)
	v.AltScreen = true

	if m.content != nil {
		if title := m.content.Title; title != "" {
			v.WindowTitle = title
		}
		if !m.content.CursorHidden && m.content.CursorRow >= 0 && m.content.CursorCol >= 0 && m.content.CursorRow < m.height && m.content.CursorCol < m.width {
			v.Cursor = tea.NewCursor(m.content.CursorCol, m.content.CursorRow)
		}
	}

	return v
}

func (m Model) renderContent() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	lines := make([]string, m.height)
	for row := 0; row < m.height; row++ {
		if m.content != nil && row < len(m.content.Cells) {
			lines[row] = renderRow(m.content.Cells[row], m.width)
			continue
		}
		lines[row] = strings.Repeat(" ", m.width)
	}

	if m.prefixMode && len(lines) > 0 {
		prefix := "[prefix]"
		if len(prefix) < m.width {
			prefix += strings.Repeat(" ", m.width-len(prefix))
		} else if len(prefix) > m.width {
			prefix = prefix[:m.width]
		}
		lines[len(lines)-1] = prefix
	}

	return strings.Join(lines, "\n")
}

func NewModel(session *actor.Ref) Model {
	return Model{session: session}
}

func ctrlBInput() KeyInput {
	return KeyInput{Code: 'b', Mods: KeyModCtrl}
}

func normalizeKeyInput(msg tea.KeyPressMsg) (KeyInput, bool) {
	key := msg.Key()
	input := KeyInput{
		Code:        key.Code,
		Text:        key.Text,
		ShiftedCode: key.ShiftedCode,
		BaseCode:    key.BaseCode,
		Mods:        keyModsFromTea(key.Mod),
		IsRepeat:    key.IsRepeat,
	}

	switch key.Code {
	case tea.KeyUp:
		input.Code = KeyCodeUp
	case tea.KeyDown:
		input.Code = KeyCodeDown
	case tea.KeyRight:
		input.Code = KeyCodeRight
	case tea.KeyLeft:
		input.Code = KeyCodeLeft
	case tea.KeyHome:
		input.Code = KeyCodeHome
	case tea.KeyEnd:
		input.Code = KeyCodeEnd
	case tea.KeyPgUp:
		input.Code = KeyCodePageUp
	case tea.KeyPgDown:
		input.Code = KeyCodePageDown
	case tea.KeyInsert:
		input.Code = KeyCodeInsert
	case tea.KeyDelete:
		input.Code = KeyCodeDelete
	case tea.KeyEnter:
		input.Code = KeyCodeEnter
	case tea.KeyBackspace:
		input.Code = KeyCodeBackspace
	case tea.KeyTab:
		input.Code = KeyCodeTab
	case tea.KeyEscape:
		input.Code = KeyCodeEscape
	case tea.KeyF1:
		input.Code = KeyCodeF1
	case tea.KeyF2:
		input.Code = KeyCodeF2
	case tea.KeyF3:
		input.Code = KeyCodeF3
	case tea.KeyF4:
		input.Code = KeyCodeF4
	case tea.KeyF5:
		input.Code = KeyCodeF5
	case tea.KeyF6:
		input.Code = KeyCodeF6
	case tea.KeyF7:
		input.Code = KeyCodeF7
	case tea.KeyF8:
		input.Code = KeyCodeF8
	case tea.KeyF9:
		input.Code = KeyCodeF9
	case tea.KeyF10:
		input.Code = KeyCodeF10
	case tea.KeyF11:
		input.Code = KeyCodeF11
	case tea.KeyF12:
		input.Code = KeyCodeF12
	}

	if input.Code == 0 && input.Text == "" {
		return KeyInput{}, false
	}
	return input, true
}

func keyModsFromTea(mod tea.KeyMod) KeyMods {
	var result KeyMods
	if mod&tea.ModShift != 0 {
		result |= KeyModShift
	}
	if mod&tea.ModAlt != 0 {
		result |= KeyModAlt
	}
	if mod&tea.ModCtrl != 0 {
		result |= KeyModCtrl
	}
	if mod&tea.ModMeta != 0 {
		result |= KeyModMeta
	}
	if mod&tea.ModSuper != 0 {
		result |= KeyModSuper
	}
	return result
}
