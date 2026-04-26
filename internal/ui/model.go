package ui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"shux/internal/actor"
	"shux/internal/protocol"
)

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
		Title:      "shux",
		SessionID:  sessionID,
		WindowID:   windowID,
		PaneID:     paneID,
		Layout:     EmptyLayoutSnapshot(sessionID, windowID),
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

func (m Model) View() tea.View {
	return tea.NewView(m.viewString())
}

// viewString builds terminal output: Lip Gloss borders/styles; content driven by LayoutSnapshot
// (Bubble Tea still owns the program loop and View contract).
func (m Model) viewString() string {
	w := m.Layout.WindowCols
	if w < 1 {
		w = 80
	}
	titleText := m.Layout.Title
	if titleText == "" {
		titleText = m.Title
	}
	titleBar := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Width(w).Render(titleText)
	var body string
	if len(m.Layout.Panes) == 0 {
		bordered := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8"))
		inner := strings.Join([]string{
			fmt.Sprintf("session: %s", m.SessionID),
			fmt.Sprintf("window: %s", m.WindowID),
			fmt.Sprintf("pane: %s", m.PaneID),
		}, "\n")
		body = bordered.Width(w).Render(inner)
	} else {
		blocks := make([]string, 0, len(m.Layout.Panes))
		for i, p := range m.Layout.Panes {
			active := p.PaneID == m.Layout.ActivePane
			if m.Layout.ActivePane == "" && i == 0 {
				active = true
			}
			st := lipgloss.NewStyle().Border(lipgloss.NormalBorder())
			if active {
				st = st.BorderForeground(lipgloss.Color("205"))
			} else {
				st = st.BorderForeground(lipgloss.Color("8"))
			}
			inner := fmt.Sprintf("%s  %d×%d  @%d,%d", p.PaneID, p.Cols, p.Rows, p.Col, p.Row)
			blocks = append(blocks, st.Width(w).Render(inner))
		}
		body = lipgloss.JoinVertical(lipgloss.Left, blocks...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body)
}
