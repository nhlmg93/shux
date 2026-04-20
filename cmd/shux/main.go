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
	snapshotPath := shux.SessionSnapshotPath(sessionName)
	shux.Infof("startup: session=%s shell=%s snapshot=%s", sessionName, opts.Shell, snapshotPath)

	// 1. Try restore from disk first (snapshot exists)
	if shux.SessionSnapshotExists(sessionName) {
		shux.Infof("startup: session=%s mode=restore path=%s", sessionName, snapshotPath)

		supervisor := &Supervisor{}
		supervisorRef := actor.Spawn(supervisor, 10)
		defer supervisorRef.Shutdown()

		sessionRef, err := shux.RestoreSessionFromSnapshot(sessionName, supervisorRef)
		if err != nil {
			return fmt.Errorf("failed to restore session: %w", err)
		}

		if err := actor.Register("session:"+sessionName, sessionRef); err != nil {
			sessionRef.Shutdown()
			return fmt.Errorf("failed to register restored session: %w", err)
		}
		defer func() {
			shux.Infof("startup: session=%s unregister restored session", sessionName)
			actor.Unregister("session:" + sessionName)
		}()

		if err := shux.DeleteSnapshot(snapshotPath); err != nil {
			shux.Warnf("startup: session=%s failed to delete restored snapshot path=%s err=%v", sessionName, snapshotPath, err)
		} else {
			shux.Infof("startup: session=%s consumed snapshot path=%s", sessionName, snapshotPath)
		}

		return runSession(sessionName, sessionRef)
	}

	// 2. Try attach to running (in-process)
	if existing := actor.WhereIs("session:" + sessionName); existing != nil {
		shux.Infof("startup: session=%s mode=attach ref=%p", sessionName, existing)
		return runSession(sessionName, existing)
	}

	// 3. Create fresh session
	shux.Infof("startup: session=%s mode=create shell=%s", sessionName, opts.Shell)
	supervisor := &Supervisor{}
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Shutdown()

	sessionRef := shux.SpawnNamedSessionWithShell(1, sessionName, opts.Shell, supervisorRef)
	if err := actor.Register("session:"+sessionName, sessionRef); err != nil {
		return fmt.Errorf("failed to register session: %w", err)
	}
	defer func() {
		shux.Infof("startup: session=%s unregister fresh session", sessionName)
		actor.Unregister("session:" + sessionName)
	}()

	shux.Infof("startup: session=%s created shell=%s", sessionName, opts.Shell)
	return runSession(sessionName, sessionRef)
}

func runSession(sessionName string, sessionRef *actor.Ref) error {
	shux.Infof("ui: session=%s starting program", sessionName)
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
	if err != nil {
		shux.Warnf("ui: session=%s program exited with err=%v", sessionName, err)
	} else {
		shux.Infof("ui: session=%s program exited cleanly", sessionName)
	}
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
