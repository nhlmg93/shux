// Package shux provides terminal multiplexing primitives.
package shux

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// PTY represents a pseudo-terminal with a running process.
type PTY struct {
	TTY *os.File
	Cmd *exec.Cmd
}

// Start creates a new PTY and starts the given command in it.
func Start(cmd *exec.Cmd) (*PTY, error) {
	return StartWithSize(cmd, 0, 0)
}

// StartWithSize creates a new PTY, starts the given command in it, and applies
// the provided terminal size before the child process begins executing.
func StartWithSize(cmd *exec.Cmd, rows, cols int) (*PTY, error) {
	var ws *pty.Winsize
	if rows > 0 && cols > 0 {
		ws = &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
	}

	ptmx, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return nil, err
	}

	// Ensure PTY is in blocking mode - required for readLoop.
	// Some environments may have non-blocking by default.
	if err := syscall.SetNonblock(int(ptmx.Fd()), false); err != nil {
		ptmx.Close()
		return nil, fmt.Errorf("failed to set blocking mode: %w", err)
	}

	return &PTY{TTY: ptmx, Cmd: cmd}, nil
}

// Close closes the PTY and cleans up resources.
func (p *PTY) Close() error {
	if p.TTY != nil {
		return p.TTY.Close()
	}
	return nil
}

// Resize updates the PTY size (rows x cols).
func (p *PTY) Resize(rows, cols int) error {
	Infof("pty: resizing to %dx%d", rows, cols)
	return pty.Setsize(p.TTY, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// Wait waits for the process to exit.
func (p *PTY) Wait() error {
	return p.Cmd.Wait()
}
