package shux

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type Model struct {
	session            RemoteSession
	keymap             Keymap
	mouseEnabled       bool
	width              int
	height             int
	prefixMode         bool
	initialized        bool
	windowView         WindowView
	startupWarnings    []string
	startupWarningStep int
}

// SetUpdateChannel is kept as a compatibility no-op for older tests.
func SetUpdateChannel(ch chan struct{}) {
	_ = ch
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return tea.RequestWindowSize() },
		func() tea.Msg { return UpdateMsg{} },
	)
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
			existing, _ := askValue(m.session, GetActiveWindow{})
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
		return m, func() tea.Msg { return UpdateMsg{} }

	case tea.KeyPressMsg:
		if m.handleKey(msg) {
			return m, tea.Quit
		}
		return m, nil

	case tea.MouseMsg:
		m.handleMouse(msg)
		return m, nil

	case tea.PasteMsg:
		if m.hasStartupWarnings() {
			return m, nil
		}
		m.session.Send(WriteToPane{Data: []byte(msg.Content)})
		return m, nil

	case UpdateMsg:
		result, _ := askValue(m.session, GetWindowView{})
		if result == nil {
			m.windowView = WindowView{}
			return m, nil
		}
		view, ok := result.(WindowView)
		if !ok {
			m.windowView = WindowView{}
			return m, nil
		}
		m.windowView = view
		return m, nil

	case WindowView:
		m.windowView = msg
		return m, nil

	default:
		return m, nil
	}
}

func (m *Model) handleKey(key tea.KeyPressMsg) bool {
	if m.hasStartupWarnings() {
		if key.Code == tea.KeyEnter {
			m.advanceStartupWarning()
		}
		return false
	}

	keystroke := key.String()

	if m.prefixMode {
		m.prefixMode = false
		action, ok := m.keymap.ActionFor(keystroke)
		if !ok {
			// Unbound key after prefix - send the prefix then the key
			m.sendKeyInput(m.keymap.PrefixInput())
			m.sendKey(key)
			return false
		}
		// Handle send_prefix action inline (UI concern)
		if action == ActionSendPrefix {
			m.sendKeyInput(m.keymap.PrefixInput())
			return false
		}
		// Send ActionMsg to session for dispatch.
		binding, _ := m.keymap.BindingFor(keystroke)
		binding = binding.normalized()
		msg := ActionMsg{Action: action, Amount: binding.Amount}
		result, _ := askValue(m.session, msg)
		switch v := result.(type) {
		case ActionResult:
			if v.Err != nil {
				Warnf("ui: action %q failed: %v", action, v.Err)
				return false
			}
			return v.Quit
		case bool:
			return v
		default:
			return false
		}
	}

	if keystroke == m.keymap.Prefix() {
		m.prefixMode = true
		return false
	}

	m.sendKey(key)
	return false
}

func (m *Model) handleMouse(msg tea.MouseMsg) {
	if m.hasStartupWarnings() {
		return
	}
	if !m.mouseEnabled {
		return
	}
	m.prefixMode = false
	input, ok := normalizeMouseInput(msg)
	if !ok {
		return
	}
	m.session.Send(input)
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
	if m.mouseEnabled {
		v.MouseMode = tea.MouseModeAllMotion
	}

	if m.windowView.Title != "" {
		v.WindowTitle = m.windowView.Title
	}

	if m.windowView.CursorOn && m.windowView.CursorRow >= 0 && m.windowView.CursorCol >= 0 && m.windowView.CursorRow < m.height && m.windowView.CursorCol < m.width {
		v.Cursor = tea.NewCursor(m.windowView.CursorCol, m.windowView.CursorRow)
	}

	return v
}

func (m Model) renderContent() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	if m.hasStartupWarnings() {
		return m.renderStartupWarnings()
	}

	if m.windowView.Content == "" {
		lines := make([]string, m.height)
		for i := range lines {
			lines[i] = strings.Repeat(" ", m.width)
		}
		return m.renderStatusLine(lines)
	}

	viewLines := strings.Split(m.windowView.Content, "\n")
	if len(viewLines) < m.height {
		for i := len(viewLines); i < m.height; i++ {
			viewLines = append(viewLines, strings.Repeat(" ", m.width))
		}
	} else if len(viewLines) > m.height {
		viewLines = viewLines[:m.height]
	}

	return m.renderStatusLine(viewLines)
}

func (m Model) hasStartupWarnings() bool {
	return len(m.startupWarnings) > 0 && m.startupWarningStep < len(m.startupWarnings)
}

func (m *Model) advanceStartupWarning() {
	if !m.hasStartupWarnings() {
		return
	}
	m.startupWarningStep++
	if m.startupWarningStep >= len(m.startupWarnings) {
		m.clearStartupWarnings()
	}
}

func (m *Model) clearStartupWarnings() {
	if len(m.startupWarnings) == 0 {
		return
	}
	m.startupWarnings = nil
	m.startupWarningStep = 0
}

func (m Model) renderStartupWarnings() string {
	lines := make([]string, 0, m.height)
	current := m.startupWarnings[m.startupWarningStep]
	messages := []string{
		"Configuration errors detected.",
		"Shux is using safe tmux-style fallback defaults.",
		"",
		current,
		"",
		"Press Enter to continue.",
	}
	if len(m.startupWarnings) > 1 {
		messages = append(messages, "", fmt.Sprintf("Error %d of %d", m.startupWarningStep+1, len(m.startupWarnings)))
	}

	for _, line := range messages {
		if len(lines) >= m.height {
			break
		}
		if line == "" {
			lines = append(lines, strings.Repeat(" ", m.width))
			continue
		}
		for len(line) > 0 && len(lines) < m.height {
			chunk := line
			if len(chunk) > m.width {
				chunk = chunk[:m.width]
				line = line[m.width:]
			} else {
				line = ""
			}
			if len(chunk) < m.width {
				chunk += strings.Repeat(" ", m.width-len(chunk))
			}
			lines = append(lines, chunk)
		}
	}
	for len(lines) < m.height {
		lines = append(lines, strings.Repeat(" ", m.width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderStatusLine(lines []string) string {
	if len(lines) > 0 && m.prefixMode {
		status := "[prefix]"
		if len(status) < m.width {
			status += strings.Repeat(" ", m.width-len(status))
		} else if len(status) > m.width {
			status = status[:m.width]
		}
		lines[len(lines)-1] = status
	}
	return strings.Join(lines, "\n")
}

// SetInitialSize seeds the model with a known terminal size before Bubble Tea
// delivers its first WindowSizeMsg.
func (m *Model) SetInitialSize(width, height int) {
	if m == nil || width <= 0 || height <= 0 {
		return
	}
	m.width = width
	m.height = height
}

func NewModel(session RemoteSession) Model {
	return NewModelWithOptions(session, DefaultKeymap(), false)
}

func NewModelWithKeymap(session RemoteSession, keymap Keymap) Model {
	return NewModelWithOptions(session, keymap, false)
}

func NewModelWithOptions(session RemoteSession, keymap Keymap, mouseEnabled bool) Model {
	return NewModelWithStartupWarnings(session, keymap, mouseEnabled, nil)
}

func NewModelWithStartupWarnings(session RemoteSession, keymap Keymap, mouseEnabled bool, startupWarnings []string) Model {
	warnings := append([]string(nil), startupWarnings...)
	return Model{session: session, keymap: keymap, mouseEnabled: mouseEnabled, startupWarnings: warnings}
}

// RemoteSession is an interface for session operations, implemented by both
// SessionRef and RemoteSessionRef.
type RemoteSession interface {
	Send(msg any) bool
	Ask(msg any) chan any
	Stop()
	Shutdown()
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

func normalizeMouseInput(msg tea.MouseMsg) (MouseInput, bool) {
	mouse := msg.Mouse()
	input := MouseInput{
		Row:    mouse.Y,
		Col:    mouse.X,
		Button: mouseButtonFromTea(mouse.Button),
		Mods:   keyModsFromTea(mouse.Mod),
	}

	switch msg.(type) {
	case tea.MouseClickMsg, tea.MouseWheelMsg:
		input.Action = MouseActionPress
	case tea.MouseReleaseMsg:
		input.Action = MouseActionRelease
	case tea.MouseMotionMsg:
		input.Action = MouseActionMotion
	default:
		return MouseInput{}, false
	}
	return input, true
}

func mouseButtonFromTea(button tea.MouseButton) MouseButton {
	switch button {
	case tea.MouseLeft:
		return MouseButtonLeft
	case tea.MouseMiddle:
		return MouseButtonMiddle
	case tea.MouseRight:
		return MouseButtonRight
	case tea.MouseWheelUp:
		return MouseButtonWheelUp
	case tea.MouseWheelDown:
		return MouseButtonWheelDown
	case tea.MouseWheelLeft:
		return MouseButtonWheelLeft
	case tea.MouseWheelRight:
		return MouseButtonWheelRight
	case tea.MouseBackward:
		return MouseButtonBackward
	case tea.MouseForward:
		return MouseButtonForward
	case tea.MouseButton10:
		return MouseButtonButton10
	default:
		return MouseButtonNone
	}
}
