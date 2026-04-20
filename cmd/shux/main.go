package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
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

	var (
		sessionRef *shux.SessionRef
		err        error
	)

	if shux.SessionSnapshotExists(sessionName) {
		shux.Infof("startup: session=%s mode=restore path=%s", sessionName, snapshotPath)
		sessionRef, err = shux.RestoreSessionFromSnapshot(sessionName, nil)
		if err != nil {
			return fmt.Errorf("failed to restore session: %w", err)
		}
		if err := shux.DeleteSnapshot(snapshotPath); err != nil {
			shux.Warnf("startup: session=%s failed to delete restored snapshot path=%s err=%v", sessionName, snapshotPath, err)
		} else {
			shux.Infof("startup: session=%s consumed snapshot path=%s", sessionName, snapshotPath)
		}
	} else {
		shux.Infof("startup: session=%s mode=create shell=%s", sessionName, opts.Shell)
		sessionRef = shux.StartNamedSessionWithShell(1, sessionName, opts.Shell, nil)
		shux.Infof("startup: session=%s created shell=%s", sessionName, opts.Shell)
	}
	defer sessionRef.Shutdown()

	return runSession(sessionName, sessionRef)
}

func runSession(sessionName string, sessionRef *shux.SessionRef) error {
	shux.Infof("ui: session=%s starting program", sessionName)
	model := shux.NewModel(sessionRef)
	opts := []tea.ProgramOption{}
	if os.Getenv("COLORTERM") == "truecolor" || os.Getenv("COLORTERM") == "24bit" {
		opts = append(opts, tea.WithColorProfile(colorprofile.TrueColor))
	}
	p := tea.NewProgram(model, opts...)

	updates := make(chan any, 32)
	sessionRef.Send(shux.SubscribeUpdates{Subscriber: updates})
	defer sessionRef.Send(shux.UnsubscribeUpdates{Subscriber: updates})

	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case msg := <-updates:
				switch msg.(type) {
				case shux.PaneContentUpdated, shux.SessionEmpty:
					p.Send(shux.UpdateMsg{})
				}
			}
		}
	}()

	_, err := p.Run()
	if err != nil {
		shux.Warnf("ui: session=%s program exited with err=%v", sessionName, err)
	} else {
		shux.Infof("ui: session=%s program exited cleanly", sessionName)
	}
	return err
}
