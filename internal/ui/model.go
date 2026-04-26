package ui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"shux/internal/actor"
	"shux/internal/protocol"
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
	Title      string
	SessionID  protocol.SessionID
	WindowID   protocol.WindowID
	PaneID     protocol.PaneID
	Layout     LayoutSnapshot
	Supervisor actor.Ref[protocol.Command]
	Ctx        context.Context
}

func NewModel(sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID) Model {
	return Model{
		Title:     "shux",
		SessionID: sessionID,
		WindowID:  windowID,
		PaneID:    paneID,
		Layout:    EmptyLayoutSnapshot(sessionID, windowID),
	}
}

// NewModelWithSupervisor attaches the supervisor command ref and context so
// terminal resizes become CommandWindowResize on the backend.
func NewModelWithSupervisor(sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID, sup actor.Ref[protocol.Command], ctx context.Context) Model {
	m := NewModel(sessionID, windowID, paneID)
	m.Supervisor = sup
	m.Ctx = ctx
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case HubEvent:
		switch e := msg.E.(type) {
		case protocol.EventWindowLayoutChanged:
			snap := LayoutSnapshotFromEvent(e)
			snap.Title = m.Layout.Title
			snap.Status = m.Layout.Status
			if snap.Title == "" {
				snap.Title = m.Title
			}
			m.Layout = snap
		}
		return m, nil
	case tea.WindowSizeMsg:
		// Invariant: resize forwarding is all-or-nothing. Partial wiring is a bug.
		switch {
		case m.Supervisor.Valid() && m.Ctx == nil:
			panic("ui: nil context with valid supervisor ref")
		case !m.Supervisor.Valid() && m.Ctx != nil:
			panic("ui: context set without valid supervisor ref")
		case !m.Supervisor.Valid():
			return m, nil
		}
		// Below: untrusted TTY/WM input; bound by ignoring rather than panicking.
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
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.Supervisor.Valid() && m.Ctx != nil {
			switch msg.String() {
			case "tab":
				return m, m.sendWindowCycleFocus()
			case "v", "ctrl+1":
				return m, m.sendPaneSplit(protocol.SplitVertical)
			case "s", "ctrl+2":
				return m, m.sendPaneSplit(protocol.SplitHorizontal)
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

func (m Model) sendPaneSplit(dir protocol.SplitDirection) tea.Cmd {
	return func() tea.Msg {
		_ = m.Supervisor.Send(m.Ctx, protocol.CommandPaneSplit{
			SessionID: m.SessionID,
			WindowID:  m.WindowID,
			Direction: dir,
		})
		return nil
	}
}

func (m Model) sendWindowCycleFocus() tea.Cmd {
	return func() tea.Msg {
		_ = m.Supervisor.Send(m.Ctx, protocol.CommandWindowCycleFocus{
			SessionID: m.SessionID,
			WindowID:  m.WindowID,
		})
		return nil
	}
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
	w := m.Layout.WindowCols
	if w < 1 {
		w = 80
	}
	var body string
	if len(m.Layout.Panes) == 0 {
		bordered := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8"))
		inner := fmt.Sprintf("%s  waiting for layout", m.PaneID)
		body = bordered.Width(w).Height(max(1, m.Layout.WindowRows)).Render(inner)
	} else {
		blocks := make([]string, 0, len(m.Layout.Panes))
		for i, p := range m.Layout.Panes {
			active := p.PaneID == m.Layout.ActivePane
			if m.Layout.ActivePane == "" && i == 0 {
				active = true
			}
			st := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width(max(1, p.Cols)).Height(max(1, p.Rows))
			if active {
				st = st.BorderForeground(lipgloss.Color("205"))
			} else {
				st = st.BorderForeground(lipgloss.Color("8"))
			}
			inner := fmt.Sprintf("%s  %d×%d  @%d,%d", p.PaneID, p.Cols, p.Rows, p.Col, p.Row)
			blocks = append(blocks, st.Render(inner))
		}
		if len(m.Layout.Panes) == 2 && m.Layout.Panes[0].Row == m.Layout.Panes[1].Row {
			body = lipgloss.JoinHorizontal(lipgloss.Top, blocks...)
		} else {
			body = lipgloss.JoinVertical(lipgloss.Left, blocks...)
		}
	}
	return body
}
