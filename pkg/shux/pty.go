// Package shux provides terminal multiplexing primitives.
package shux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// Pty is the interface for pseudo-terminal operations.
// This allows mocking PTY for testing.
type Pty interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	Kill() error // Kill process first, then close PTY (interrupts blocking reads)
	Resize(rows, cols int) error
	Wait() error
	PID() int
}

// Compile-time check that *PTY implements Pty interface
var _ Pty = (*PTY)(nil)

// PTY represents a pseudo-terminal with a running process.
type PTY struct {
	TTY *os.File
	Cmd *exec.Cmd
	pid int
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
		if closeErr := ptmx.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to set blocking mode: %w (close PTY: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to set blocking mode: %w", err)
	}

	return &PTY{TTY: ptmx, Cmd: cmd, pid: cmd.Process.Pid}, nil
}

// Read reads data from the PTY.
func (p *PTY) Read(buf []byte) (int, error) {
	return p.TTY.Read(buf)
}

// Write writes data to the PTY.
func (p *PTY) Write(data []byte) (int, error) {
	return p.TTY.Write(data)
}

// Kill terminates the child process first, then closes the PTY.
// This order ensures blocking reads are interrupted by the process death
// (which closes the slave side of the PTY) rather than relying on
// Close() to wake up blocked readers.
func (p *PTY) Kill() error {
	// Kill process first - this closes the slave side and wakes up master read
	if p.Cmd != nil && p.Cmd.Process != nil {
		_ = p.Cmd.Process.Kill()
	}

	// Give process a moment to die and close slave side
	time.Sleep(10 * time.Millisecond)

	// Now close the master side
	if p.TTY != nil {
		return p.TTY.Close()
	}
	return nil
}

// Close closes the PTY and stops the child process.
func (p *PTY) Close() error {
	var ttyErr error
	if p.TTY != nil {
		ttyErr = p.TTY.Close()
	}
	if p.Cmd != nil && p.Cmd.Process != nil {
		if err := p.Cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			if ttyErr != nil {
				return fmt.Errorf("close pty: %w (kill process: %v)", ttyErr, err)
			}
			return err
		}
	}
	return ttyErr
}

// Resize updates the PTY size (rows x cols).
func (p *PTY) Resize(rows, cols int) error {
	return pty.Setsize(p.TTY, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// Wait waits for the process to exit.
func (p *PTY) Wait() error {
	return p.Cmd.Wait()
}

// PID returns the process ID of the running command.
func (p *PTY) PID() int {
	return p.pid
}
