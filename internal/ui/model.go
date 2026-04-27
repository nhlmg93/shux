package ui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"shux/internal/actor"
	"shux/internal/protocol"
)

const maxPendingCommands = 32

type initialRenderMsg struct{}

type pendingKind uint8

const (
	pendingPaneSplit pendingKind = iota + 1
	pendingPaneClose
	pendingWindowCreate
)

type pendingCommand struct {
	Kind pendingKind
}

type ExitIntent uint8

const (
	ExitDetach ExitIntent = iota
	ExitQuit
)

// HubEvent carries a hub fanout event into the Bubble Tea update loop.
type HubEvent struct {
	E protocol.Event
}

// ProgramEventSink implements protocol.EventSink and forwards to [tea.Program.Send].
type ProgramEventSink struct {
	P *tea.Program
}

func (s *ProgramEventSink) DeliverEvent(_ context.Context, e protocol.Event) error {
	if s == nil || s.P == nil {
		return nil
	}
	s.P.Send(HubEvent{E: e})
	return nil
}

// ModelConfig configures a Model. Supervisor and Ctx must be set together or
// both omitted; the model wires Bubble Tea events to backend commands when
// they're present and runs read-only otherwise.
type ModelConfig struct {
	ClientID   protocol.ClientID
	SessionID  protocol.SessionID
	WindowID   protocol.WindowID
	PaneID     protocol.PaneID
	Supervisor actor.Ref[protocol.Command]
	Ctx        context.Context
	OnExit     func(ExitIntent)
}

type Model struct {
	Title         string
	ClientID      protocol.ClientID
	SessionID     protocol.SessionID
	WindowID      protocol.WindowID
	PaneID        protocol.PaneID
	ActivePaneID  protocol.PaneID
	Pending       map[protocol.RequestID]pendingCommand
	NextRequest   protocol.RequestID
	Layout        LayoutSnapshot
	Screens       map[protocol.PaneID]protocol.EventPaneScreenChanged
	WindowIDs     []protocol.WindowID
	ClosedWindows map[protocol.WindowID]bool
	Layouts       map[protocol.WindowID]LayoutSnapshot
	WindowScreens map[protocol.WindowID]map[protocol.PaneID]protocol.EventPaneScreenChanged
	Supervisor    actor.Ref[protocol.Command]
	Ctx           context.Context
	OnExit        func(ExitIntent)
	Prefix        bool
}

// NewModel returns a Model wired from cfg. Supervisor and Ctx must be set
// together; passing one without the other is a programming bug.
func NewModel(cfg ModelConfig) Model {
	if cfg.Supervisor.Valid() && cfg.Ctx == nil {
		panic("ui: NewModel: supervisor without context")
	}
	if !cfg.Supervisor.Valid() && cfg.Ctx != nil {
		panic("ui: NewModel: context without supervisor")
	}
	return Model{
		Title:         "shux",
		ClientID:      cfg.ClientID,
		SessionID:     cfg.SessionID,
		WindowID:      cfg.WindowID,
		PaneID:        cfg.PaneID,
		ActivePaneID:  cfg.PaneID,
		Pending:       make(map[protocol.RequestID]pendingCommand),
		Layout:        EmptyLayoutSnapshot(cfg.SessionID, cfg.WindowID),
		Screens:       make(map[protocol.PaneID]protocol.EventPaneScreenChanged),
		WindowIDs:     []protocol.WindowID{cfg.WindowID},
		ClosedWindows: make(map[protocol.WindowID]bool),
		Layouts:       make(map[protocol.WindowID]LayoutSnapshot),
		WindowScreens: make(map[protocol.WindowID]map[protocol.PaneID]protocol.EventPaneScreenChanged),
		Supervisor:    cfg.Supervisor,
		Ctx:           cfg.Ctx,
		OnExit:        cfg.OnExit,
	}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return initialRenderMsg{} }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case HubEvent:
		return m.handleHubEvent(msg.E)
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	case tea.KeyReleaseMsg:
		if !m.Prefix {
			return m, m.dispatch(keyCommand(m.SessionID, m.WindowID, m.ActivePaneID, msg.Key(), protocol.KeyActionRelease))
		}
	case tea.PasteMsg:
		return m, m.dispatch(protocol.CommandPanePaste{
			SessionID: m.SessionID,
			WindowID:  m.WindowID,
			PaneID:    m.ActivePaneID,
			Data:      []byte(msg.Content),
		})
	case tea.MouseMsg:
		if cmd, ok := m.mouseCommand(msg); ok {
			return m, m.dispatch(cmd)
		}
	}
	return m, nil
}

func (m Model) handleHubEvent(e protocol.Event) (Model, tea.Cmd) {
	switch e := e.(type) {
	case protocol.EventSessionWindowsChanged:
		if e.SessionID != m.SessionID {
			return m, nil
		}
		return m.WithWindowIDs(e.Windows), nil
	case protocol.EventWindowCreated:
		if e.SessionID != m.SessionID {
			return m, nil
		}
		m = m.addWindowID(e.WindowID)
		if e.ClientID == m.ClientID {
			if pending, ok := m.Pending[e.RequestID]; ok && pending.Kind == pendingWindowCreate {
				delete(m.Pending, e.RequestID)
				m = m.switchWindow(e.WindowID)
			}
		}
		return m, nil
	case protocol.EventPaneCreated:
		if e.SessionID != m.SessionID || e.WindowID != m.WindowID {
			return m, nil
		}
		if !m.ActivePaneID.Valid() {
			m.ActivePaneID = e.PaneID
		}
		return m, nil
	case protocol.EventWindowClosed:
		if e.SessionID != m.SessionID {
			return m, nil
		}
		m = m.removeWindow(e.WindowID)
		if e.WindowID != m.WindowID {
			return m, nil
		}
		if len(m.WindowIDs) == 0 {
			if m.OnExit != nil {
				m.OnExit(ExitQuit)
			}
			return m, tea.Quit
		}
		m = m.switchWindow(m.WindowIDs[0])
		return m, m.currentWindowResizeCmd()
	case protocol.EventWindowLayoutChanged:
		if e.SessionID != m.SessionID {
			return m, nil
		}
		snap := LayoutSnapshotFromEvent(e)
		m.storeLayoutSnapshot(snap)
		if e.WindowID != m.WindowID {
			return m, nil
		}
		return m.applyLayoutSnapshot(snap), nil
	case protocol.EventPaneScreenChanged:
		if e.SessionID != m.SessionID {
			return m, nil
		}
		m.storePaneScreen(e)
		if e.WindowID != m.WindowID {
			return m, nil
		}
		return m.WithPaneScreen(e), nil
	case protocol.EventPaneClosed:
		if e.WindowID != m.WindowID {
			return m, nil
		}
		delete(m.Screens, e.PaneID)
		if m.ActivePaneID == e.PaneID {
			m.ActivePaneID = normalizeActivePane("", m.Layout.Panes)
		}
	case protocol.EventPaneCloseLastRequested:
		if e.ClientID != m.ClientID {
			return m, nil
		}
		delete(m.Pending, e.RequestID)
		if m.OnExit != nil {
			m.OnExit(ExitQuit)
		}
		return m, tea.Quit
	case protocol.EventPaneSplitCompleted:
		if e.ClientID != m.ClientID {
			return m, nil
		}
		if pending, ok := m.Pending[e.RequestID]; ok && pending.Kind == pendingPaneSplit {
			delete(m.Pending, e.RequestID)
			m.ActivePaneID = e.NewPaneID
			m.Layout.ActivePane = normalizeActivePane(m.ActivePaneID, m.Layout.Panes)
		}
	case protocol.EventCommandRejected:
		if e.ClientID == m.ClientID {
			delete(m.Pending, e.RequestID)
		}
	}
	return m, nil
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	if !m.Supervisor.Valid() {
		return m, nil
	}
	if msg.Width <= 0 || msg.Height <= 0 || msg.Width > 0xFFFF || msg.Height > 0xFFFF {
		return m, nil
	}
	m.Layout.WindowCols = int(msg.Width)
	m.Layout.WindowRows = int(msg.Height)
	return m, m.dispatch(protocol.CommandWindowResize{
		SessionID: m.SessionID,
		WindowID:  m.WindowID,
		Cols:      uint16(msg.Width),
		Rows:      uint16(msg.Height),
	})
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if !m.Prefix {
		if key == "ctrl+b" {
			m.Prefix = true
			return m, nil
		}
		return m, m.dispatch(keyCommand(m.SessionID, m.WindowID, m.ActivePaneID, msg.Key(), keyActionFromPress(msg)))
	}
	m.Prefix = false
	if !m.Supervisor.Valid() {
		return m, nil
	}
	switch key {
	case "d":
		if m.OnExit != nil {
			m.OnExit(ExitDetach)
		}
		return m, tea.Quit
	case "%":
		return m.startPaneSplit(protocol.SplitVertical)
	case "\"":
		return m.startPaneSplit(protocol.SplitHorizontal)
	case "o":
		m.ActivePaneID = cycleActivePane(m.ActivePaneID, m.Layout.Panes)
		m.Layout.ActivePane = m.ActivePaneID
		return m, nil
	case "x":
		return m.startPaneClose(m.ActivePaneID)
	case "q":
		if m.OnExit != nil {
			m.OnExit(ExitQuit)
		}
		return m, tea.Quit
	case "c":
		return m.startWindowCreate()
	case "n":
		m = m.switchWindowByOffset(1)
		return m, m.currentWindowResizeCmd()
	case "p":
		m = m.switchWindowByOffset(-1)
		return m, m.currentWindowResizeCmd()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m = m.switchWindowByNumber(int(key[0] - '0'))
		return m, m.currentWindowResizeCmd()
	case "0":
		m = m.switchWindowByNumber(10)
		return m, m.currentWindowResizeCmd()
	case "?":
		fmt.Fprintf(os.Stderr, "ui: prefix key %q not implemented yet\n", key)
		return m, nil
	}
	return m, nil
}

// dispatch returns a tea.Cmd that sends cmd to the supervisor when the model
// has a live supervisor+context. With no supervisor (test/read-only) it's a
// no-op tea.Cmd.
func (m Model) currentWindowResizeCmd() tea.Cmd {
	cols, rows := uint16OrZero(m.Layout.WindowCols), uint16OrZero(m.Layout.WindowRows)
	if cols == 0 || rows == 0 {
		return nil
	}
	return m.dispatch(protocol.CommandWindowResize{SessionID: m.SessionID, WindowID: m.WindowID, Cols: cols, Rows: rows})
}

func (m Model) dispatch(cmd protocol.Command) tea.Cmd {
	if !m.Supervisor.Valid() || m.Ctx == nil {
		return nil
	}
	sup, ctx := m.Supervisor, m.Ctx
	return func() tea.Msg {
		_ = sup.Send(ctx, cmd)
		return nil
	}
}

func (m Model) startWindowCreate() (Model, tea.Cmd) {
	if !m.ClientID.Valid() {
		return m, nil
	}
	req := m.rememberPending(pendingCommand{Kind: pendingWindowCreate})
	return m, m.dispatch(protocol.CommandCreateWindow{
		Meta:      protocol.CommandMeta{ClientID: m.ClientID, RequestID: req},
		SessionID: m.SessionID,
		Cols:      uint16OrZero(m.Layout.WindowCols),
		Rows:      uint16OrZero(m.Layout.WindowRows),
		AutoPane:  true,
	})
}

func uint16OrZero(v int) uint16 {
	if v <= 0 || v > 0xFFFF {
		return 0
	}
	return uint16(v)
}

func (m Model) startPaneClose(paneID protocol.PaneID) (Model, tea.Cmd) {
	if !m.ClientID.Valid() || !paneID.Valid() {
		return m, nil
	}
	req := m.rememberPending(pendingCommand{Kind: pendingPaneClose})
	return m, m.dispatch(protocol.CommandPaneClose{
		Meta:      protocol.CommandMeta{ClientID: m.ClientID, RequestID: req},
		SessionID: m.SessionID,
		WindowID:  m.WindowID,
		PaneID:    paneID,
	})
}

func (m Model) startPaneSplit(dir protocol.SplitDirection) (Model, tea.Cmd) {
	if !m.ClientID.Valid() {
		return m, nil
	}
	req := m.rememberPending(pendingCommand{Kind: pendingPaneSplit})
	return m, m.dispatch(protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: m.ClientID, RequestID: req},
		SessionID:    m.SessionID,
		WindowID:     m.WindowID,
		TargetPaneID: m.ActivePaneID,
		Direction:    dir,
	})
}

func keyActionFromPress(msg tea.KeyPressMsg) protocol.KeyAction {
	if msg.Key().IsRepeat {
		return protocol.KeyActionRepeat
	}
	return protocol.KeyActionPress
}

func keyCommand(sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID, key tea.Key, action protocol.KeyAction) protocol.CommandPaneKey {
	name := key.String()
	return protocol.CommandPaneKey{
		SessionID:   sessionID,
		WindowID:    windowID,
		PaneID:      paneID,
		Action:      action,
		Key:         normalizeKeyName(name),
		Text:        key.Text,
		Modifiers:   keyModifiers(key.Mod),
		BaseKey:     runeString(key.BaseCode),
		ShiftedKey:  runeString(key.ShiftedCode),
		PhysicalKey: runeString(key.Code),
	}
}

func normalizeKeyName(key string) string {
	if len(key) == 1 {
		return key
	}
	key = strings.TrimPrefix(key, "ctrl+")
	key = strings.TrimPrefix(key, "alt+")
	key = strings.TrimPrefix(key, "shift+")
	key = strings.TrimPrefix(key, "meta+")
	switch key {
	case " ":
		return "space"
	case "esc":
		return "escape"
	case "pgup":
		return "pageup"
	case "pgdown":
		return "pagedown"
	default:
		return key
	}
}

func keyModifiers(mod tea.KeyMod) protocol.InputModifiers {
	var mods protocol.InputModifiers
	if mod&tea.ModShift != 0 {
		mods |= protocol.ModifierShift
	}
	if mod&tea.ModCtrl != 0 {
		mods |= protocol.ModifierCtrl
	}
	if mod&tea.ModAlt != 0 {
		mods |= protocol.ModifierAlt
	}
	if mod&tea.ModMeta != 0 {
		mods |= protocol.ModifierMeta
	}
	return mods
}

func runeString(r rune) string {
	if r == 0 {
		return ""
	}
	return string(r)
}

func (m Model) mouseCommand(msg tea.MouseMsg) (protocol.CommandPaneMouse, bool) {
	mouse := msg.Mouse()
	pane, ok := m.paneAt(mouse.X, mouse.Y)
	if !ok {
		return protocol.CommandPaneMouse{}, false
	}
	action := protocol.MouseActionPress
	switch msg.(type) {
	case tea.MouseReleaseMsg:
		action = protocol.MouseActionRelease
	case tea.MouseMotionMsg:
		action = protocol.MouseActionMotion
	case tea.MouseWheelMsg:
		action = protocol.MouseActionWheel
	}
	return protocol.CommandPaneMouse{
		SessionID: m.SessionID,
		WindowID:  m.WindowID,
		PaneID:    pane.PaneID,
		Action:    action,
		Button:    mouseButton(mouse.Button),
		Modifiers: keyModifiers(mouse.Mod),
		CellCol:   mouse.X - pane.Col - 1,
		CellRow:   mouse.Y - pane.Row - 1,
	}, true
}

func (m Model) paneAt(col, row int) (LayoutPane, bool) {
	for _, pane := range m.Layout.Panes {
		if col > pane.Col && col < pane.Col+pane.Cols-1 && row > pane.Row && row < pane.Row+pane.Rows-1 {
			return pane, true
		}
	}
	return LayoutPane{}, false
}

func mouseButton(button tea.MouseButton) protocol.MouseButton {
	switch button {
	case tea.MouseLeft:
		return protocol.MouseButtonLeft
	case tea.MouseMiddle:
		return protocol.MouseButtonMiddle
	case tea.MouseRight:
		return protocol.MouseButtonRight
	case tea.MouseWheelUp:
		return protocol.MouseButtonWheelUp
	case tea.MouseWheelDown:
		return protocol.MouseButtonWheelDown
	case tea.MouseWheelLeft:
		return protocol.MouseButtonWheelLeft
	case tea.MouseWheelRight:
		return protocol.MouseButtonWheelRight
	default:
		return protocol.MouseButtonNone
	}
}

// rememberPending registers a pending command and returns its assigned request id.
// The caller embeds the returned RequestID in the outgoing command's Meta.
func (m *Model) rememberPending(pending pendingCommand) protocol.RequestID {
	if m.Pending == nil {
		m.Pending = make(map[protocol.RequestID]pendingCommand)
	}
	if len(m.Pending) >= maxPendingCommands {
		var oldest protocol.RequestID
		for id := range m.Pending {
			if oldest == 0 || id < oldest {
				oldest = id
			}
		}
		delete(m.Pending, oldest)
	}
	m.NextRequest++
	m.Pending[m.NextRequest] = pending
	return m.NextRequest
}

func (m Model) WithWindowIDs(ids []protocol.WindowID) Model {
	m.WindowIDs = m.WindowIDs[:0]
	for _, wid := range ids {
		if wid.Valid() && !m.ClosedWindows[wid] {
			m.WindowIDs = append(m.WindowIDs, wid)
		}
	}
	if len(m.WindowIDs) == 0 && m.WindowID.Valid() && !m.ClosedWindows[m.WindowID] {
		m.WindowIDs = []protocol.WindowID{m.WindowID}
	}
	return m
}

func (m Model) addWindowID(windowID protocol.WindowID) Model {
	if !windowID.Valid() || m.ClosedWindows[windowID] {
		return m
	}
	for _, wid := range m.WindowIDs {
		if wid == windowID {
			return m
		}
	}
	m.WindowIDs = append(m.WindowIDs, windowID)
	return m
}

func (m Model) removeWindow(windowID protocol.WindowID) Model {
	if m.ClosedWindows == nil {
		m.ClosedWindows = make(map[protocol.WindowID]bool)
	}
	m.ClosedWindows[windowID] = true
	for i, wid := range m.WindowIDs {
		if wid == windowID {
			m.WindowIDs = append(m.WindowIDs[:i], m.WindowIDs[i+1:]...)
			break
		}
	}
	delete(m.Layouts, windowID)
	delete(m.WindowScreens, windowID)
	return m
}

func (m *Model) storeLayoutSnapshot(snap LayoutSnapshot) {
	if m.Layouts == nil {
		m.Layouts = make(map[protocol.WindowID]LayoutSnapshot)
	}
	m.Layouts[snap.WindowID] = snap
}

func (m *Model) storePaneScreen(screen protocol.EventPaneScreenChanged) {
	if m.WindowScreens == nil {
		m.WindowScreens = make(map[protocol.WindowID]map[protocol.PaneID]protocol.EventPaneScreenChanged)
	}
	screens := m.WindowScreens[screen.WindowID]
	if screens == nil {
		screens = make(map[protocol.PaneID]protocol.EventPaneScreenChanged)
		m.WindowScreens[screen.WindowID] = screens
	}
	screens[screen.PaneID] = screen
}

func (m Model) switchWindow(windowID protocol.WindowID) Model {
	if !windowID.Valid() || windowID == m.WindowID {
		return m
	}
	cols, rows := m.Layout.WindowCols, m.Layout.WindowRows
	m.WindowID = windowID
	m.Layout = EmptyLayoutSnapshot(m.SessionID, windowID)
	if snap, ok := m.Layouts[windowID]; ok {
		m.Layout = snap
	}
	if cols > 0 {
		m.Layout.WindowCols = cols
	}
	if rows > 0 {
		m.Layout.WindowRows = rows
	}
	m.Screens = make(map[protocol.PaneID]protocol.EventPaneScreenChanged)
	if screens := m.WindowScreens[windowID]; screens != nil {
		for pid, screen := range screens {
			m.Screens[pid] = screen
		}
	}
	m.ActivePaneID = normalizeActivePane("", m.Layout.Panes)
	m.Layout.ActivePane = m.ActivePaneID
	return m
}

func (m Model) switchWindowByOffset(offset int) Model {
	if len(m.WindowIDs) == 0 {
		return m
	}
	idx := 0
	for i, wid := range m.WindowIDs {
		if wid == m.WindowID {
			idx = i
			break
		}
	}
	idx = (idx + offset) % len(m.WindowIDs)
	if idx < 0 {
		idx += len(m.WindowIDs)
	}
	return m.switchWindow(m.WindowIDs[idx])
}

func (m Model) switchWindowByNumber(number int) Model {
	index := number - 1
	if index < 0 || index >= len(m.WindowIDs) {
		return m
	}
	return m.switchWindow(m.WindowIDs[index])
}

func (m Model) applyLayoutSnapshot(snap LayoutSnapshot) Model {
	snap.Title = m.Layout.Title
	snap.Status = m.Layout.Status
	if snap.Title == "" {
		snap.Title = m.Title
	}
	m.ActivePaneID = normalizeActivePane(m.ActivePaneID, snap.Panes)
	snap.ActivePane = m.ActivePaneID
	m.Layout = snap
	return m
}

func (m Model) WithLayoutSnapshot(snap LayoutSnapshot) Model {
	m.storeLayoutSnapshot(snap)
	if snap.WindowID != m.WindowID {
		return m
	}
	return m.applyLayoutSnapshot(snap)
}

func (m Model) WithPaneScreen(screen protocol.EventPaneScreenChanged) Model {
	if m.Screens == nil {
		m.Screens = make(map[protocol.PaneID]protocol.EventPaneScreenChanged)
	}
	m.storePaneScreen(screen)
	if screen.WindowID != m.WindowID {
		return m
	}
	m.Screens[screen.PaneID] = screen
	return m
}

func normalizeActivePane(active protocol.PaneID, panes []LayoutPane) protocol.PaneID {
	if len(panes) == 0 {
		return ""
	}
	for _, p := range panes {
		if p.PaneID == active {
			return active
		}
	}
	return panes[0].PaneID
}

func cycleActivePane(active protocol.PaneID, panes []LayoutPane) protocol.PaneID {
	if len(panes) == 0 {
		return ""
	}
	active = normalizeActivePane(active, panes)
	for i, p := range panes {
		if p.PaneID == active {
			return panes[(i+1)%len(panes)].PaneID
		}
	}
	return panes[0].PaneID
}

func (m Model) paneScreen(paneID protocol.PaneID) protocol.EventPaneScreenChanged {
	screen, ok := m.Screens[paneID]
	if !ok {
		return protocol.EventPaneScreenChanged{}
	}
	return screen
}

func (m Model) activeCursor() *tea.Cursor {
	paneID := m.Layout.ActivePane
	if paneID == "" {
		paneID = m.ActivePaneID
	}
	screen, ok := m.Screens[paneID]
	if !ok || !screen.Cursor.Visible {
		return nil
	}
	for _, p := range m.Layout.Panes {
		if p.PaneID != paneID {
			continue
		}
		col := p.Col + 1 + screen.Cursor.Col
		row := p.Row + 1 + screen.Cursor.Row
		if col <= p.Col || col >= p.Col+p.Cols-1 || row <= p.Row || row >= p.Row+p.Rows-1 {
			return nil
		}
		cursor := tea.NewCursor(col, row)
		cursor.Blink = screen.Cursor.Blink
		return cursor
	}
	return nil
}

func (m Model) View() tea.View {
	v := tea.NewView(m.viewString())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.KeyboardEnhancements.ReportEventTypes = true
	v.KeyboardEnhancements.ReportAlternateKeys = true
	v.KeyboardEnhancements.ReportAllKeysAsEscapeCodes = true
	v.KeyboardEnhancements.ReportAssociatedText = true
	return v
}

// viewString builds terminal output: Lip Gloss borders/styles; content driven by LayoutSnapshot
// (Bubble Tea still owns the program loop and View contract). Pane lines are a logical preview
// of cell geometry, not a pixel-matched terminal partition.
func (m Model) viewString() string {
	cols := m.Layout.WindowCols
	rows := m.Layout.WindowRows
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	canvas := newRuneCanvas(cols, rows)
	if len(m.Layout.Panes) == 0 {
		canvas.drawText(0, 0, fmt.Sprintf("%s  waiting for layout", m.PaneID))
		return canvas.String()
	}
	for i, p := range m.Layout.Panes {
		active := p.PaneID == m.Layout.ActivePane
		if m.Layout.ActivePane == "" && i == 0 {
			active = true
		}
		canvas.drawPaneWithScreenEvent(p, active, m.paneScreen(p.PaneID))
	}
	return canvas.String()
}
