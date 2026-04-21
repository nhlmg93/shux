package shux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/go-libghostty"
)

// PaneRuntime owns the irreplaceable live state of a pane:
// - PTY master FD
// - Child process PID/wait handling
// - Ghostty terminal state
// - VT/playback state
//
// This layer survives pane/window/session controller restarts.
// It is destroyed only by explicit kill or actual child exit.
type PaneRuntime struct {
	id           uint32
	term         *libghostty.Terminal
	renderState  *libghostty.RenderState
	rowIterator  *libghostty.RenderStateRowIterator
	rowCells     *libghostty.RenderStateRowCells
	keyEncoder   *libghostty.KeyEncoder
	mouseEncoder *libghostty.MouseEncoder
	pty          Pty

	shell       string
	cwd         string
	rows        int
	cols        int
	windowTitle string
	bellCount   uint64

	// Synchronization
	mu       sync.RWMutex
	closed   bool
	stopCh   chan struct{} // Signals readLoop to exit
	readDone chan struct{} // Signals readLoop has exited

	// Callbacks for events
	onTitleChanged func(title string)
	onBell         func()
	onOutput       func()
	onProcessExit  func(error)

	logger ShuxLogger
}

// PaneRuntimeConfig contains configuration for creating a new PaneRuntime.
type PaneRuntimeConfig struct {
	ID             uint32
	Rows           int
	Cols           int
	Shell          string
	CWD            string
	Logger         ShuxLogger
	OnTitleChanged func(title string)
	OnBell         func()
	OnOutput       func()
	OnProcessExit  func(err error)
}

// NewPaneRuntime creates a new pane runtime with PTY and child process.
// Returns an initialized runtime ready for use.
func NewPaneRuntime(cfg PaneRuntimeConfig) (*PaneRuntime, error) {
	originalRows, originalCols := cfg.Rows, cfg.Cols
	rows, cols, changed := sanitizeTermSize(cfg.Rows, cfg.Cols)
	if changed && cfg.Logger != nil {
		cfg.Logger.Warnf("pane_runtime: id=%d sanitize-size from=%dx%d to=%dx%d", cfg.ID, originalRows, originalCols, rows, cols)
	}

	pr := &PaneRuntime{
		id:             cfg.ID,
		rows:           rows,
		cols:           cols,
		shell:          cfg.Shell,
		cwd:            cfg.CWD,
		logger:         cfg.Logger,
		onTitleChanged: cfg.OnTitleChanged,
		onBell:         cfg.OnBell,
		onOutput:       cfg.OnOutput,
		onProcessExit:  cfg.OnProcessExit,
		stopCh:         make(chan struct{}),
		readDone:       make(chan struct{}),
	}

	if err := pr.init(); err != nil {
		return nil, err
	}

	return pr, nil
}

// init initializes the runtime: creates terminal, starts PTY/process.
func (pr *PaneRuntime) init() error {
	if pr.logger != nil {
		pr.logger.Infof("pane_runtime: id=%d init shell=%s cwd=%s rows=%d cols=%d", pr.id, pr.shell, pr.cwd, pr.rows, pr.cols)
	}

	if err := pr.createTerminal(); err != nil {
		return fmt.Errorf("create terminal: %w", err)
	}

	if err := pr.createPTY(); err != nil {
		pr.cleanupTerminal()
		return fmt.Errorf("create pty: %w", err)
	}

	go pr.readLoop()

	if pr.logger != nil {
		pr.logger.Infof("pane_runtime: id=%d started pid=%d shell=%s rows=%d cols=%d", pr.id, pr.PID(), pr.shell, pr.rows, pr.cols)
	}

	return nil
}

// createPTY spawns the shell process with PTY.
func (pr *PaneRuntime) createPTY() error {
	if pr.logger != nil {
		pr.logger.Infof("pane_runtime: id=%d spawn shell=%s cwd=%s", pr.id, pr.shell, pr.cwd)
	}

	cmd := pr.newCommand()
	pty, err := startPanePTY(cmd, pr.rows, pr.cols)
	if err != nil {
		return err
	}

	pr.pty = pty
	return nil
}

// newCommand creates the exec.Cmd for the shell.
func (pr *PaneRuntime) newCommand() *exec.Cmd {
	cmd := exec.Command(pr.shell)
	cmd.Env = ensureTermEnv(os.Environ())
	if pr.cwd != "" {
		cmd.Dir = ResolveCWD(pr.cwd)
	}
	return cmd
}

// readLoop reads PTY output and feeds it to the terminal.
// This runs in its own goroutine and coordinates with Kill() for clean shutdown.
func (pr *PaneRuntime) readLoop() {
	defer close(pr.readDone)

	buf := make([]byte, 4096)
	readErrCh := make(chan error, 1)
	waitDone := make(chan error, 1)

	pr.mu.RLock()
	pty := pr.pty
	pr.mu.RUnlock()

	if pty == nil {
		return
	}

	go func() {
		for {
			select {
			case <-pr.stopCh:
				readErrCh <- fmt.Errorf("stopped")
				return
			default:
			}

			n, err := pty.Read(buf)
			if n > 0 {
				pr.mu.RLock()
				term := pr.term
				onOutput := pr.onOutput
				pr.mu.RUnlock()
				if term != nil {
					term.VTWrite(buf[:n])
				}
				if onOutput != nil {
					onOutput()
				}
			}
			if err != nil {
				readErrCh <- err
				return
			}
		}
	}()

	go func() {
		waitDone <- pty.Wait()
	}()

	// Wait for either read error or process exit
	var err error
	select {
	case err = <-readErrCh:
	case err = <-waitDone:

		select {
		case <-pr.stopCh:

		default:
			close(pr.stopCh)
		}

		<-readErrCh
	}

	pr.mu.Lock()
	pr.closed = true
	livePTY := pr.pty
	pr.pty = nil
	pr.cleanupTerminal()
	pr.mu.Unlock()

	pid := 0
	if livePTY != nil {
		pid = livePTY.PID()
		_ = livePTY.Close()
	}

	if pr.logger != nil {
		pr.logger.Infof("pane_runtime: id=%d process-exited pid=%d err=%v", pr.id, pid, err)
	}

	if pr.onProcessExit != nil {
		pr.onProcessExit(err)
	}
}

// ID returns the pane ID.
func (pr *PaneRuntime) ID() uint32 {
	return pr.id
}

// PID returns the child process PID (0 if not running).
func (pr *PaneRuntime) PID() int {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	if pr.pty == nil {
		return 0
	}
	return pr.pty.PID()
}

// Write writes data to the PTY.
func (pr *PaneRuntime) Write(data []byte) (int, error) {
	pr.mu.RLock()
	pty := pr.pty
	pr.mu.RUnlock()

	if pty == nil {
		return 0, fmt.Errorf("pty not available")
	}
	return pty.Write(data)
}

// Resize resizes the terminal and PTY.
func (pr *PaneRuntime) Resize(rows, cols int) error {
	originalRows, originalCols := rows, cols
	rows, cols, changed := sanitizeTermSize(rows, cols)
	if changed && pr.logger != nil {
		pr.logger.Warnf("pane_runtime: id=%d sanitize-resize from=%dx%d to=%dx%d", pr.id, originalRows, originalCols, rows, cols)
	}

	pr.mu.Lock()
	defer pr.mu.Unlock()

	if pr.term != nil {
		_ = pr.term.Resize(uint16(cols), uint16(rows), 0, 0)
	}
	if pr.pty != nil {
		_ = pr.pty.Resize(rows, cols)
	}

	pr.rows = rows
	pr.cols = cols

	return nil
}

// GetSize returns the current terminal size.
func (pr *PaneRuntime) GetSize() (rows, cols int) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.rows, pr.cols
}

// IsAltScreen returns true if the terminal is in alt screen mode.
func (pr *PaneRuntime) IsAltScreen() bool {
	pr.mu.RLock()
	term := pr.term
	pr.mu.RUnlock()

	if term == nil {
		return false
	}
	alt, _ := term.ModeGet(libghostty.ModeAltScreen)
	return alt
}

// IsCursorVisible returns true if the cursor is visible.
func (pr *PaneRuntime) IsCursorVisible() bool {
	pr.mu.RLock()
	term := pr.term
	pr.mu.RUnlock()

	if term == nil {
		return false
	}
	visible, _ := term.ModeGet(libghostty.ModeCursorVisible)
	return visible
}

// GetTitle returns the current window title.
func (pr *PaneRuntime) GetTitle() string {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.windowTitle
}

// GetBellCount returns the bell count.
func (pr *PaneRuntime) GetBellCount() uint64 {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.bellCount
}

// GetCWD returns the current working directory of the child process.
func (pr *PaneRuntime) GetCWD() string {
	pr.mu.RLock()
	pty := pr.pty
	pr.mu.RUnlock()

	if pty == nil {
		return ""
	}

	pid := pty.PID()
	if pid == 0 {
		return ""
	}

	cwd, err := GetProcessCWD(pid)
	if err != nil {
		return ""
	}

	return ResolveCWD(cwd)
}

// IsClosed returns true if the runtime has closed (process exited).
func (pr *PaneRuntime) IsClosed() bool {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.closed
}

// Kill terminates the child process and closes the PTY.
// This is the explicit kill path - destroys the runtime.
// Kill terminates the child process and closes the PTY.
// This is the explicit kill path - destroys the runtime.
func (pr *PaneRuntime) Kill() error {
	pr.mu.Lock()
	if pr.closed {
		pr.mu.Unlock()
		return nil
	}
	pr.closed = true

	pty := pr.pty
	pid := 0
	if pty != nil {
		pid = pty.PID()
	}
	readDone := pr.readDone
	if pr.logger != nil {
		pr.logger.Infof("pane_runtime: id=%d kill pid=%d", pr.id, pid)
	}

	select {
	case <-pr.stopCh:
	default:
		close(pr.stopCh)
	}
	pr.mu.Unlock()

	if pty != nil {
		_ = pty.Kill()
	}

	select {
	case <-readDone:
	case <-time.After(250 * time.Millisecond):
		if pty != nil {
			_ = pty.Close()
		}
	}

	return nil
}

// Close closes the runtime resources without killing (if already exited).
func (pr *PaneRuntime) Close() {
	pr.mu.Lock()
	pty := pr.pty
	pid := 0
	if pty != nil {
		pid = pty.PID()
	}
	pr.closed = true
	select {
	case <-pr.stopCh:
	default:
		close(pr.stopCh)
	}
	readDone := pr.readDone
	pr.mu.Unlock()

	if pr.logger != nil {
		pr.logger.Infof("pane_runtime: id=%d close pid=%d", pr.id, pid)
	}

	if pty != nil {
		_ = pty.Close()
	}

	select {
	case <-readDone:
	case <-time.After(250 * time.Millisecond):
	}

	pr.mu.Lock()
	pr.pty = nil
	pr.cleanupTerminal()
	pr.mu.Unlock()
}

// GetSnapshotData returns snapshot data for the runtime.
func (pr *PaneRuntime) GetSnapshotData() PaneSnapshotData {
	rows, cols := pr.GetSize()

	return PaneSnapshotData{
		ID:          pr.id,
		Shell:       pr.shell,
		Rows:        rows,
		Cols:        cols,
		CWD:         pr.GetCWD(),
		WindowTitle: pr.GetTitle(),
	}
}

// startPanePTY is the function used to start a PTY for a pane.
// This can be overridden for testing.
var startPanePTY = func(cmd *exec.Cmd, rows, cols int) (Pty, error) {
	return StartWithSize(cmd, rows, cols)
}

// ensureTermEnv ensures the TERM environment variable is set.
func ensureTermEnv(env []string) []string {
	for _, entry := range env {
		if strings.HasPrefix(entry, "TERM=") {
			return env
		}
	}
	return append(env, "TERM=xterm-256color")
}
