package main

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/sys/unix"

	"shux/pkg/shux"
)

// runClient connects to a live owner and runs the UI.
func runClient(sessionName, socketPath string, opts RunOptions) error {
	logger := shux.DefaultShuxLogger()

	remoteRef, err := shux.NewRemoteSessionRef(socketPath, sessionName, logger)
	if err != nil {
		return fmt.Errorf("failed to connect to session owner: %w", err)
	}
	defer remoteRef.Shutdown()

	shux.Infof("client: session=%s connected", sessionName)
	return runSessionRemote(sessionName, remoteRef, opts.Keymap, opts.MouseEnabled, opts.StartupWarnings)
}

func detectTerminalSize() (width, height int, ok bool) {
	fds := []uintptr{os.Stdout.Fd(), os.Stdin.Fd()}
	for _, fd := range fds {
		ws, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
		if err != nil || ws == nil || ws.Col == 0 || ws.Row == 0 {
			continue
		}
		return int(ws.Col), int(ws.Row), true
	}
	return 0, 0, false
}

// runSessionRemote runs the UI with a remote session reference.
func runSessionRemote(sessionName string, remoteRef *shux.RemoteSessionRef, keymap shux.Keymap, mouseEnabled bool, startupWarnings []string) error {
	shux.Infof("ui: session=%s starting remote program", sessionName)
	model := shux.NewModelWithStartupWarnings(remoteRef, keymap, mouseEnabled, startupWarnings)
	opts := []tea.ProgramOption{}
	if os.Getenv("COLORTERM") == "truecolor" || os.Getenv("COLORTERM") == "24bit" {
		opts = append(opts, tea.WithColorProfile(colorprofile.TrueColor))
	}
	if width, height, ok := detectTerminalSize(); ok {
		shux.Infof("ui: bootstrap terminal size %dx%d", width, height)
		model.SetInitialSize(width, height)
		opts = append(opts, tea.WithWindowSize(width, height))
		if existing := <-remoteRef.Ask(shux.GetActiveWindow{}); existing == nil {
			shux.Infof("ui: bootstrap creating initial window %dx%d", height, width)
			remoteRef.Send(shux.CreateWindow{Rows: height, Cols: width})
		} else {
			shux.Infof("ui: bootstrap resizing existing session to %dx%d", height, width)
			remoteRef.Send(shux.ResizeMsg{Rows: height, Cols: width})
		}
	}
	p := tea.NewProgram(model, opts...)

	updates := make(chan any, 32)
	remoteRef.Send(shux.SubscribeUpdates{Subscriber: updates})
	defer remoteRef.Send(shux.UnsubscribeUpdates{Subscriber: updates})

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
	go func() {
		delays := []time.Duration{0, 50 * time.Millisecond, 150 * time.Millisecond, 400 * time.Millisecond}
		for _, delay := range delays {
			if delay > 0 {
				time.Sleep(delay)
			}
			p.Send(shux.UpdateMsg{})
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
