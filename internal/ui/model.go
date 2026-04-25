package ui

import (
	tea "charm.land/bubbletea/v2"
	"shux/internal/protocol"
)

type Model struct {
	Title     string
	SessionID protocol.SessionID
	WindowID  protocol.WindowID
	PaneID    protocol.PaneID
}

func NewModel(sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID) Model {
	return Model{
		Title:     "shux",
		SessionID: sessionID,
		WindowID:  windowID,
		PaneID:    paneID,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() tea.View {
	return tea.NewView(m.Title + "\n" +
		"session: " + string(m.SessionID) + "\n" +
		"window: " + string(m.WindowID) + "\n" +
		"pane: " + string(m.PaneID) + "\n")
}
