package ui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"shux/internal/actor"
	"shux/internal/protocol"
)

const maxPendingCommands = 32

type initialRenderMsg struct{}

type pendingKind uint8

const (
	pendingPaneSplit pendingKind = iota + 1
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

type Model struct {
	Title        string
	ClientID     protocol.ClientID
	SessionID    protocol.SessionID
	WindowID     protocol.WindowID
	PaneID       protocol.PaneID
	ActivePaneID protocol.PaneID
	Pending      map[protocol.RequestID]pendingCommand
	NextRequest  protocol.RequestID
	Layout       LayoutSnapshot
	Supervisor   actor.Ref[protocol.Command]
	Ctx          context.Context
	OnExit       func(ExitIntent)
	Prefix       bool
}

func NewModel(sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID) Model {
	return NewModelForClient("", sessionID, windowID, paneID)
}

func NewModelForClient(clientID protocol.ClientID, sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID) Model {
	return Model{
		Title:        "shux",
		ClientID:     clientID,
		SessionID:    sessionID,
		WindowID:     windowID,
		PaneID:       paneID,
		ActivePaneID: paneID,
		Pending:      make(map[protocol.RequestID]pendingCommand),
		Layout:       EmptyLayoutSnapshot(sessionID, windowID),
	}
}

// NewModelWithSupervisor attaches the supervisor command ref and context so
// terminal resizes become CommandWindowResize on the backend.
func NewModelWithSupervisor(clientID protocol.ClientID, sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID, sup actor.Ref[protocol.Command], ctx context.Context) Model {
	m := NewModelForClient(clientID, sessionID, windowID, paneID)
	m.Supervisor = sup
	m.Ctx = ctx
	return m
}

func NewModelWithSupervisorAndExit(clientID protocol.ClientID, sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID, sup actor.Ref[protocol.Command], ctx context.Context, onExit func(ExitIntent)) Model {
	m := NewModelWithSupervisor(clientID, sessionID, windowID, paneID, sup, ctx)
	m.OnExit = onExit
	return m
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return initialRenderMsg{} }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case HubEvent:
		switch e := msg.E.(type) {
		case protocol.EventWindowLayoutChanged:
			if e.SessionID != m.SessionID || e.WindowID != m.WindowID {
				return m, nil
			}
			m = m.applyLayoutSnapshot(LayoutSnapshotFromEvent(e))
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
	case tea.WindowSizeMsg:
		switch {
		case m.Supervisor.Valid() && m.Ctx == nil:
			panic("ui: nil context with valid supervisor ref")
		case !m.Supervisor.Valid() && m.Ctx != nil:
			panic("ui: context set without valid supervisor ref")
		case !m.Supervisor.Valid():
			return m, nil
		}
		if msg.Width <= 0 || msg.Height <= 0 {
			return m, nil
		}
		if msg.Width > 0xFFFF || msg.Height > 0xFFFF {
			return m, nil
		}
		m.Layout.WindowCols = int(msg.Width)
		m.Layout.WindowRows = int(msg.Height)
		cols, rows := uint16(msg.Width), uint16(msg.Height)
		return m, m.sendWindowResize(cols, rows)
	case tea.KeyPressMsg:
		key := msg.String()
		if !m.Prefix {
			if key == "ctrl+b" {
				m.Prefix = true
			}
			return m, nil
		}
		m.Prefix = false
		if m.Supervisor.Valid() && m.Ctx != nil {
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
			case "q":
				if m.OnExit != nil {
					m.OnExit(ExitQuit)
				}
				return m, tea.Quit
			case "c":
				panic("ui: create window not implemented")
			case "n":
				panic("ui: next window not implemented")
			case "p":
				panic("ui: previous window not implemented")
			case "?":
				panic("ui: list key bindings not implemented")
			case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
				panic("ui: select window by index not implemented")
			}
		}
	}

	return m, nil
}

func (m Model) sendWindowResize(cols, rows uint16) tea.Cmd {
	return func() tea.Msg {
		_ = m.Supervisor.Send(m.Ctx, protocol.CommandWindowResize{
			SessionID: m.SessionID,
			WindowID:  m.WindowID,
			Cols:      cols,
			Rows:      rows,
		})
		return nil
	}
}

func (m Model) startPaneSplit(dir protocol.SplitDirection) (Model, tea.Cmd) {
	if !m.ClientID.Valid() {
		return m, nil
	}
	req := m.nextRequestID()
	m.rememberPending(req, pendingCommand{Kind: pendingPaneSplit})
	return m, m.sendPaneSplit(req, m.ActivePaneID, dir)
}

func (m Model) nextRequestID() protocol.RequestID {
	return m.NextRequest + 1
}

func (m *Model) rememberPending(req protocol.RequestID, pending pendingCommand) {
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
	m.NextRequest = req
	m.Pending[req] = pending
}

func (m Model) sendPaneSplit(req protocol.RequestID, target protocol.PaneID, dir protocol.SplitDirection) tea.Cmd {
	return func() tea.Msg {
		_ = m.Supervisor.Send(m.Ctx, protocol.CommandPaneSplit{
			Meta: protocol.CommandMeta{
				ClientID:  m.ClientID,
				RequestID: req,
			},
			SessionID:    m.SessionID,
			WindowID:     m.WindowID,
			TargetPaneID: target,
			Direction:    dir,
		})
		return nil
	}
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
	return m.applyLayoutSnapshot(snap)
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

func (m Model) View() tea.View {
	v := tea.NewView(m.viewString())
	v.AltScreen = true
	return v
}

// viewString builds terminal output: Lip Gloss borders/styles; content driven by LayoutSnapshot
// (Bubble Tea still owns the program loop and View contract). Pane lines are a logical preview
// of cell geometry, not a pixel-matched terminal partition.
func (m Model) viewString() string {
	cols := max(1, m.Layout.WindowCols)
	rows := max(1, m.Layout.WindowRows)
	if cols == 1 && m.Layout.WindowCols == 0 {
		cols = 80
	}
	if rows == 1 && m.Layout.WindowRows == 0 {
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
		canvas.drawPane(p, active)
	}
	return canvas.String()
}
