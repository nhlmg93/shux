package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/nhlmg93/gotor/actor"
	"shux/pkg/shux"
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
		if err := run(existing); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Create new session
	supervisor := &Supervisor{}
	supervisorRef := actor.Spawn(supervisor, 10)
	sessionRef := shux.SpawnSession(1, supervisorRef)

	// Register session globally
	if err := actor.Register("session:"+sessionName, sessionRef); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to register session: %v\n", err)
		os.Exit(1)
	}

	shux.Infof("created new session: %s", sessionName)
	err := run(sessionRef)

	// Cleanup
	actor.Unregister("session:" + sessionName)
	supervisorRef.Shutdown()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(sessionRef *actor.Ref) error {
	model := shux.NewModel(sessionRef)
	p := tea.NewProgram(model)

	bridgeRef := actor.Spawn(&UIBridge{program: p}, 32)
	sessionRef.Send(shux.SubscribeUpdates{Subscriber: bridgeRef})
	defer func() {
		sessionRef.Send(shux.UnsubscribeUpdates{Subscriber: bridgeRef})
		bridgeRef.Shutdown()
	}()

	_, err := p.Run()
	return err
}

// UIBridge forwards updates into the Bubble Tea program.
type UIBridge struct {
	program interface{ Send(tea.Msg) }
}

func (u *UIBridge) Receive(msg any) {
	switch msg.(type) {
	case shux.PaneContentUpdated, shux.SessionEmpty:
		u.program.Send(shux.UpdateMsg{})
	}
}

// Supervisor handles top-level coordination.
type Supervisor struct{}

func (s *Supervisor) Receive(msg any) {
	// Handle supervisor messages if needed.
	_ = msg
}
