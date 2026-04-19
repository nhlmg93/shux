package main

import (
	"bufio"
	"os"

	"gomux/pkg/gomux"
	"github.com/nhlmg93/gotor/actor"
	"golang.org/x/term"
)

func main() {
	// Put terminal in raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Create supervisor (just handles top-level shutdown)
	supervisor := NewSupervisorActor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Shutdown()

	// Create session
	sessionRef := gomux.SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(gomux.CreateWindow{})

	// Run input loop in goroutine
	go runActorInputLoop(sessionRef, oldState)

	// Wait for supervisor to signal quit (when last session closes)
	supervisor.WaitForQuit()

	// Restore terminal before exit
	term.Restore(int(os.Stdin.Fd()), oldState)
}

func runActorInputLoop(sessionRef *actor.Ref, oldState *term.State) {
	reader := bufio.NewReader(os.Stdin)
	prefixMode := false

	for {
		ch, _ := reader.ReadByte()
		if prefixMode {
			prefixMode = false
			switch ch {
			case '1', '2':
				sessionRef.Send(gomux.SwitchToPane{Index: int(ch - '1')})
			case 'c':
				// Need to get active window first
				reply := sessionRef.Ask(gomux.GetActiveWindow{})
				winRef := <-reply
				if winRef != nil {
					winRef.(*actor.Ref).Send(gomux.CreatePane{Cmd: "/bin/sh", Args: []string{}})
				}
			case 'x':
				// Get active pane and kill it
				reply := sessionRef.Ask(gomux.GetActivePane{})
				paneRef := <-reply
				if paneRef != nil {
					paneRef.(*actor.Ref).Send(gomux.KillPane{})
				}
			case 'n':
				sessionRef.Send(gomux.SwitchWindow{Delta: 1})
			case 'p':
				sessionRef.Send(gomux.SwitchWindow{Delta: -1})
			case 'w':
				sessionRef.Send(gomux.CreateWindow{})
			case 'q':
				term.Restore(int(os.Stdin.Fd()), oldState)
				return
		default:
			// Get active pane and write to it
			reply := sessionRef.Ask(gomux.GetActivePane{})
			paneRef := <-reply
			if paneRef != nil {
				paneRef.(*actor.Ref).Send(gomux.WriteToPane{Data: []byte{1, ch}})
			}
		}
	} else if ch == 1 { // Ctrl+A
		prefixMode = true
	} else {
		// Get active pane and write to it
		reply := sessionRef.Ask(gomux.GetActivePane{})
		paneRef := <-reply
		if paneRef != nil {
			paneRef.(*actor.Ref).Send(gomux.WriteToPane{Data: []byte{ch}})
		}
	}
}
}

// SupervisorActor handles top-level coordination
type SupervisorActor struct {
	quitChan chan struct{}
}

func NewSupervisorActor() *SupervisorActor {
	return &SupervisorActor{
		quitChan: make(chan struct{}),
	}
}

func (s *SupervisorActor) Receive(msg any) {
	switch msg.(type) {
	case gomux.SessionEmpty:
		// Last session closed, signal quit
		close(s.quitChan)
	}
}

func (s *SupervisorActor) WaitForQuit() {
	<-s.quitChan
}
