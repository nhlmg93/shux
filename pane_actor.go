package main

import (
	"os/exec"

	"github.com/nhlmg93/gotor/actor"
)

// PaneActor manages a single PTY and its process
type PaneActor struct {
	id     uint32
	pty    *PTY
	parent *actor.Ref
	self   *actor.Ref
}

// Receive handles messages for the pane
func (p *PaneActor) Receive(msg any) {
	switch m := msg.(type) {
	case WriteToPane:
		if p.pty != nil {
			p.pty.TTY.Write(m.Data)
		}
	case KillPane:
		if p.pty != nil {
			p.pty.Close()
		}
		p.notifyExited()
	}
}

func (p *PaneActor) GetRef() *actor.Ref {
	return p.self
}

func (p *PaneActor) notifyExited() {
	if p.parent != nil {
		p.parent.Send(PaneExited{ID: p.id})
	}
}

func SpawnPaneActor(id uint32, cmd *exec.Cmd, parent *actor.Ref) (*actor.Ref, error) {
	pty, err := Start(cmd)
	if err != nil {
		return nil, err
	}

	pane := &PaneActor{
		id:     id,
		pty:    pty,
		parent: parent,
	}

	ref := actor.Spawn(pane, 10)
	pane.self = ref
	go pane.readLoop()

	return ref, nil
}

func (p *PaneActor) readLoop() {
	buf := make([]byte, 1024)
	for {
		n, err := p.pty.TTY.Read(buf)
		if err != nil {
			p.notifyExited()
			return
		}
		if p.parent != nil {
			p.parent.Send(PaneOutput{ID: p.id, Data: append([]byte(nil), buf[:n]...)})
		}
	}
}