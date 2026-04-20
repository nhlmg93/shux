package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/nhlmg93/gotor/actor"
	"shux/pkg/shux"
)

func main() {
	if err := shux.InitLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to init logger: %v\n", err)
	}

	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runApp(opts RunOptions) error {
	sessionName := opts.SessionName

	if existing := actor.WhereIs("session:" + sessionName); existing != nil {
		shux.Infof("attaching to existing session: %s", sessionName)
		return runSession(existing)
	}

	supervisor := &Supervisor{}
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Shutdown()

	sessionRef := shux.SpawnSessionWithShell(1, opts.Shell, supervisorRef)
	if err := actor.Register("session:"+sessionName, sessionRef); err != nil {
		return fmt.Errorf("failed to register session: %w", err)
	}
	defer actor.Unregister("session:" + sessionName)

	shux.Infof("created new session: %s (shell=%s)", sessionName, opts.Shell)
	return runSession(sessionRef)
}

func runSession(sessionRef *actor.Ref) error {
	model := shux.NewModel(sessionRef)
	opts := []tea.ProgramOption{}
	if os.Getenv("COLORTERM") == "truecolor" || os.Getenv("COLORTERM") == "24bit" {
		opts = append(opts, tea.WithColorProfile(colorprofile.TrueColor))
	}
	p := tea.NewProgram(model, opts...)

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
	_ = msg
}
