package main

import (
	"os/exec"
	"sync/atomic"
)

var paneIDCounter uint32

// Pane represents a single terminal pane with a PTY
type Pane struct {
	ID  uint32
	PTY *PTY
}

// NewPane creates a new pane running the given command
func NewPane(cmd *exec.Cmd) (*Pane, error) {
	pty, err := Start(cmd)
	if err != nil {
		return nil, err
	}

	id := atomic.AddUint32(&paneIDCounter, 1)
	return &Pane{ID: id, PTY: pty}, nil
}

// Close closes the pane and its PTY
func (p *Pane) Close() error {
	if p.PTY != nil {
		return p.PTY.Close()
	}
	return nil
}

// Exited returns true if the pane's process has exited
func (p *Pane) Exited() bool {
	if p.PTY == nil || p.PTY.Cmd == nil {
		return true
	}
	return p.PTY.Cmd.ProcessState != nil
}
