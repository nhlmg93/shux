package main

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// PTY represents a pseudo-terminal with a running process
type PTY struct {
	TTY *os.File
	Cmd *exec.Cmd
}

// Start creates a new PTY and starts the given command in it
func Start(cmd *exec.Cmd) (*PTY, error) {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	return &PTY{TTY: ptmx, Cmd: cmd}, nil
}

// Close closes the PTY and cleans up resources
func (p *PTY) Close() error {
	if p.TTY != nil {
		return p.TTY.Close()
	}
	return nil
}
