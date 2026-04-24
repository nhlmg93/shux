package ui

import tea "charm.land/bubbletea/v2"

type Model struct {
	Title string
}

func NewModel() Model {
	return Model{
		Title: "shux",
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
	return tea.NewView(m.Title + "\n")
}
