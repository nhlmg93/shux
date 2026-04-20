package main

import (
	"fmt"
	"os"

	"gomux/pkg/gomux"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nhlmg93/gotor/actor"
)

func main() {
	// Initialize logging
	if err := gomux.InitLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to init logger: %v\n", err)
	}

	// Create supervisor
	supervisor := &SupervisorActor{}
	supervisorRef := actor.Spawn(supervisor, 10)

	// Create session with supervisor as parent
	sessionRef := gomux.SpawnSessionActor(1, supervisorRef)

	// Run Bubble Tea program
	model := gomux.NewModel(sessionRef)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Shutdown supervisor
	supervisorRef.Shutdown()
}

// SupervisorActor handles top-level coordination
type SupervisorActor struct{}

func (s *SupervisorActor) Receive(msg any) {
	// Handle supervisor messages if needed
	_ = msg
}
