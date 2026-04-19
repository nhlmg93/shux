package main

import (
	"fmt"
	"os"

	"gomux/pkg/gomux"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nhlmg93/gotor/actor"
)

func main() {
	// Create channel for actor → UI communication
	uiChan := make(chan tea.Msg, 100) // Larger buffer for startup messages

	// Create supervisor with UI channel
	supervisor := NewSupervisorActor(uiChan)
	supervisorRef := actor.Spawn(supervisor, 10)

	// Create session with supervisor as parent
	sessionRef := gomux.SpawnSessionActor(1, supervisorRef)

	// Run Bubble Tea program with UI channel
	// CreateWindow will be sent after UI is ready
	model := gomux.NewModel(sessionRef, uiChan)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Shutdown supervisor
	supervisorRef.Shutdown()
}

// SupervisorActor handles top-level coordination and UI notifications
type SupervisorActor struct {
	uiChan chan tea.Msg
}

func NewSupervisorActor(uiChan chan tea.Msg) *SupervisorActor {
	return &SupervisorActor{
		uiChan: uiChan,
	}
}

func (s *SupervisorActor) Receive(msg any) {
	switch m := msg.(type) {
	case gomux.SessionEmpty:
		s.uiChan <- tea.Quit()
	case gomux.GridUpdated:
		s.uiChan <- gomux.UIMsg{Msg: m}
	}
}
