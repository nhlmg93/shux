package main

import (
	"fmt"
	"os"

	"shux/pkg/shux"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nhlmg93/gotor/actor"
)

func main() {
	// Initialize logging
	if err := shux.InitLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to init logger: %v\n", err)
	}

	// Get session name from args (default: "default")
	sessionName := "default"
	if len(os.Args) > 1 {
		sessionName = os.Args[1]
	}

	// Check if session already exists
	if existing := actor.WhereIs("session:" + sessionName); existing != nil {
		shux.Infof("attaching to existing session: %s", sessionName)
		
		// Create update channel for actor→UI communication
		updateCh := make(chan struct{}, 10)
		shux.SetUpdateChannel(updateCh)
		
		run(existing, updateCh)
		return
	}

	// Create new session
	supervisor := &SupervisorActor{}
	supervisorRef := actor.Spawn(supervisor, 10)
	sessionRef := shux.SpawnSession(1, supervisorRef)

	// Register session globally
	if err := actor.Register("session:"+sessionName, sessionRef); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to register session: %v\n", err)
		os.Exit(1)
	}

	shux.Infof("created new session: %s", sessionName)
	
	// Create update channel for actor→UI communication
	updateCh := make(chan struct{}, 10) // Buffered to prevent blocking
	shux.SetUpdateChannel(updateCh)
	
	run(sessionRef, updateCh)

	// Cleanup
	actor.Unregister("session:" + sessionName)
	supervisorRef.Shutdown()
}

func run(sessionRef *actor.Ref, updateCh chan struct{}) {
	model := shux.NewModel(sessionRef, updateCh)
	p := tea.NewProgram(model, tea.WithAltScreen())
	
	// Goroutine to forward actor updates to Bubble Tea
	go func() {
		for range updateCh {
			p.Send(shux.UpdateMsg{})
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// SupervisorActor handles top-level coordination
type SupervisorActor struct{}

func (s *SupervisorActor) Receive(msg any) {
	// Handle supervisor messages if needed
	_ = msg
}
