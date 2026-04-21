package shux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

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
	stopCh   chan struct{}  // Signals readLoop to exit
	readDone chan struct{}  // Signals readLoop has exited

	// Callbacks for events
	onTitleChanged func(title string)
	onBell         func()
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

	// Start read loop
	go pr.readLoop()

	if pr.logger != nil {
		pr.logger.Infof("pane_runtime: id=%d started pid=%d shell=%s rows=%d cols=%d", pr.id, pr.PID(), pr.shell, pr.rows, pr.cols)
	}

	return nil
}

// createTerminal creates the Ghostty terminal and supporting objects.
func (pr *PaneRuntime) createTerminal() error {
	var err error

	pr.term, err = libghostty.NewTerminal(
		libghostty.WithSize(uint16(pr.cols), uint16(pr.rows)),
		libghostty.WithMaxScrollback(10000),
		libghostty.WithTitleChanged(func(t *libghostty.Terminal) {
			if title, err := t.Title(); err == nil {
				pr.mu.Lock()
				pr.windowTitle = title
				pr.mu.Unlock()
				if pr.onTitleChanged != nil {
					pr.onTitleChanged(title)
				}
			}
		}),
		libghostty.WithBell(func(t *libghostty.Terminal) {
			pr.mu.Lock()
			pr.bellCount++
			pr.mu.Unlock()
			if pr.onBell != nil {
				pr.onBell()
			}
		}),
		libghostty.WithWritePty(func(t *libghostty.Terminal, data []byte) {
			pr.mu.RLock()
			pty := pr.pty
			pr.mu.RUnlock()
			if pty != nil {
				_, _ = pty.Write(data)
			}
		}),
	)
	if err != nil {
		return err
	}

	pr.renderState, err = libghostty.NewRenderState()
	if err != nil {
		pr.term.Close()
		pr.term = nil
		return err
	}

	pr.rowIterator, err = libghostty.NewRenderStateRowIterator()
	if err != nil {
		pr.renderState.Close()
		pr.renderState = nil
		pr.term.Close()
		pr.term = nil
		return err
	}

	pr.rowCells, err = libghostty.NewRenderStateRowCells()
	if err != nil {
		pr.rowIterator.Close()
		pr.rowIterator = nil
		pr.renderState.Close()
		pr.renderState = nil
		pr.term.Close()
		pr.term = nil
		return err
	}

	pr.keyEncoder, err = libghostty.NewKeyEncoder()
	if err != nil {
		pr.rowCells.Close()
		pr.rowCells = nil
		pr.rowIterator.Close()
		pr.rowIterator = nil
		pr.renderState.Close()
		pr.renderState = nil
		pr.term.Close()
		pr.term = nil
		return err
	}

	pr.mouseEncoder, err = libghostty.NewMouseEncoder()
	if err != nil {
		pr.keyEncoder.Close()
		pr.keyEncoder = nil
		pr.rowCells.Close()
		pr.rowCells = nil
		pr.rowIterator.Close()
		pr.rowIterator = nil
		pr.renderState.Close()
		pr.renderState = nil
		pr.term.Close()
		pr.term = nil
		return err
	}
	pr.mouseEncoder.SetOptTrackLastCell(true)

	return nil
}

// cleanupTerminal closes terminal-related resources.
func (pr *PaneRuntime) cleanupTerminal() {
	if pr.mouseEncoder != nil {
		pr.mouseEncoder.Close()
		pr.mouseEncoder = nil
	}
	if pr.keyEncoder != nil {
		pr.keyEncoder.Close()
		pr.keyEncoder = nil
	}
	if pr.rowCells != nil {
		pr.rowCells.Close()
		pr.rowCells = nil
	}
	if pr.rowIterator != nil {
		pr.rowIterator.Close()
		pr.rowIterator = nil
	}
	if pr.renderState != nil {
		pr.renderState.Close()
		pr.renderState = nil
	}
	if pr.term != nil {
		pr.term.Close()
		pr.term = nil
	}
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

	// Get PTY reference - we hold it for the entire read loop duration
	// to avoid races with Kill() closing the PTY
	pr.mu.RLock()
	pty := pr.pty
	pr.mu.RUnlock()

	if pty == nil {
		return
	}

	// Read loop - runs until error or stop signal
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
				pr.mu.RUnlock()
				if term != nil {
					term.VTWrite(buf[:n])
				}
			}
			if err != nil {
				readErrCh <- err
				return
			}
		}
	}()

	// Wait loop - waits for process exit
	go func() {
		waitDone <- pty.Wait()
	}()

	// Wait for either read error or process exit
	var err error
	select {
	case err = <-readErrCh:
	case err = <-waitDone:
		// Process exited - signal read loop to stop (if not already closed)
		select {
		case <-pr.stopCh:
			// Already closed
		default:
			close(pr.stopCh)
		}
		// Wait for read loop to finish
		<-readErrCh
	}

	pr.mu.Lock()
	pr.closed = true
	pr.mu.Unlock()

	if pr.logger != nil {
		pr.logger.Infof("pane_runtime: id=%d process-exited pid=%d err=%v", pr.id, pr.PID(), err)
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

	// Get PID directly without calling PID() to avoid lock reentrancy issues
	pid := 0
	if pr.pty != nil {
		pid = pr.pty.PID()
	}
	if pr.logger != nil {
		pr.logger.Infof("pane_runtime: id=%d kill pid=%d", pr.id, pid)
	}

	// Signal read loop to stop
	select {
	case <-pr.stopCh:
		// Already closed
	default:
		close(pr.stopCh)
	}

	// Kill PTY (kills process first, then closes PTY - interrupts blocking reads)
	if pr.pty != nil {
		_ = pr.pty.Kill()
	}

	pr.mu.Unlock()

	// Note: We don't wait for read loop to exit here because blocking
	// PTY reads may not be interruptible on all systems. The read loop
	// will exit asynchronously when the PTY close is detected or process dies.
	// The cleanup in Close() will handle any remaining resources.

	return nil
}

// Close closes the runtime resources without killing (if already exited).
func (pr *PaneRuntime) Close() {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if pr.logger != nil {
		pr.logger.Infof("pane_runtime: id=%d close pid=%d", pr.id, pr.PID())
	}

	if pr.pty != nil {
		_ = pr.pty.Close()
	}

	pr.cleanupTerminal()
}

// RenderState returns the render state for building content.
func (pr *PaneRuntime) RenderState() *libghostty.RenderState {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.renderState
}

// Term returns the underlying terminal (for advanced operations).
func (pr *PaneRuntime) Term() *libghostty.Terminal {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.term
}

// KeyEncoder returns the key encoder.
func (pr *PaneRuntime) KeyEncoder() *libghostty.KeyEncoder {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.keyEncoder
}

// MouseEncoder returns the mouse encoder.
func (pr *PaneRuntime) MouseEncoder() *libghostty.MouseEncoder {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.mouseEncoder
}

// BuildContent builds the current pane content from the render state.
func (pr *PaneRuntime) BuildContent() *PaneContent {
	pr.mu.RLock()
	term := pr.term
	renderState := pr.renderState
	rowIterator := pr.rowIterator
	rowCells := pr.rowCells
	rows := pr.rows
	cols := pr.cols
	pr.mu.RUnlock()

	if term == nil || renderState == nil || rowIterator == nil || rowCells == nil {
		return &PaneContent{
			Lines:     make([]string, 0),
			Cells:     make([][]PaneCell, 0),
			CursorRow: 0,
			CursorCol: 0,
		}
	}

	// Update render state from terminal
	if err := renderState.Update(term); err != nil {
		return &PaneContent{
			Lines:     make([]string, rows),
			Cells:     make([][]PaneCell, rows),
			CursorRow: 0,
			CursorCol: 0,
		}
	}

	// Get cursor position
	cursorRow, cursorCol := 0, 0
	cursorVisible := true
	if hasCursor, _ := renderState.CursorViewportHasValue(); hasCursor {
		if x, err := renderState.CursorViewportX(); err == nil {
			cursorCol = int(x)
		}
		if y, err := renderState.CursorViewportY(); err == nil {
			cursorRow = int(y)
		}
	}
	if visible, err := renderState.CursorVisible(); err == nil {
		cursorVisible = visible
	}

	// Get scrollback rows
	scrollbackRows := uint(0)
	if scrollback, err := term.ScrollbackRows(); err == nil {
		scrollbackRows = scrollback
	}

	// Populate row iterator
	if err := renderState.RowIterator(rowIterator); err != nil {
		return &PaneContent{
			Lines:     make([]string, rows),
			Cells:     make([][]PaneCell, rows),
			CursorRow: cursorRow,
			CursorCol: cursorCol,
		}
	}

	lines := make([]string, rows)
	cells := make([][]PaneCell, rows)

	rowIdx := 0
	for rowIdx < rows && rowIterator.Next() {
		lineCells := make([]PaneCell, cols)

		// Get cells for this row
		if err := rowIterator.Cells(rowCells); err == nil {
			colIdx := 0
			for colIdx < cols && rowCells.Next() {
				cell := pr.buildCellFromRowCells(rowCells)
				lineCells[colIdx] = cell
				colIdx++
			}
		}

		// Fill remaining cells with blanks
		for c := 0; c < cols; c++ {
			if lineCells[c].Text == "" {
				lineCells[c] = PaneCell{Text: " ", Width: 1}
			}
		}

		// Build line string
		var line strings.Builder
		for _, cell := range lineCells {
			if cell.Width == 0 {
				continue
			}
			if cell.Text == "" {
				line.WriteByte(' ')
			} else {
				line.WriteString(cell.Text)
			}
		}

		lines[rowIdx] = line.String()
		cells[rowIdx] = lineCells
		rowIdx++
	}

	// Fill remaining rows with blanks
	for ; rowIdx < rows; rowIdx++ {
		lineCells := make([]PaneCell, cols)
		for c := 0; c < cols; c++ {
			lineCells[c] = PaneCell{Text: " ", Width: 1}
		}
		lines[rowIdx] = strings.Repeat(" ", cols)
		cells[rowIdx] = lineCells
	}

	return &PaneContent{
		Lines:          lines,
		Cells:          cells,
		CursorRow:      cursorRow,
		CursorCol:      cursorCol,
		InAltScreen:    pr.IsAltScreen(),
		CursorHidden:   !cursorVisible,
		Title:          pr.GetTitle(),
		BellCount:      pr.GetBellCount(),
		ScrollbackRows: scrollbackRows,
	}
}

// buildCellFromRowCells builds a PaneCell from row cell data.
func (pr *PaneRuntime) buildCellFromRowCells(rowCells *libghostty.RenderStateRowCells) PaneCell {
	cell := PaneCell{Text: " ", Width: 1}

	// Get raw cell info
	raw, err := rowCells.Raw()
	if err != nil {
		return cell
	}

	// Check width
	if wide, err := raw.Wide(); err == nil {
		switch wide {
		case libghostty.CellWideWide:
			cell.Width = 2
		case libghostty.CellWideSpacerTail, libghostty.CellWideSpacerHead:
			cell.Width = 0
			cell.Text = ""
		default:
			cell.Width = 1
		}
	}

	// Get graphemes (text content)
	if graphemes, err := rowCells.Graphemes(); err == nil && len(graphemes) > 0 {
		cell.Text = codepointsToString(graphemes)
	} else if cell.Width > 0 {
		cell.Text = " "
	}

	// Get style
	if style, err := rowCells.Style(); err == nil {
		cell.Bold = style.Bold()
		cell.Italic = style.Italic()
		cell.Underline = style.Underline() != libghostty.UnderlineNone
		cell.Blink = style.Blink()
		cell.Reverse = style.Inverse()
	}

	// Get colors
	if fg, err := rowCells.FgColor(); err == nil && fg != nil {
		cell.HasFgColor = true
		cell.FgColor = *fg
	}
	if bg, err := rowCells.BgColor(); err == nil && bg != nil {
		cell.HasBgColor = true
		cell.BgColor = *bg
	}

	// Check hyperlink
	if hasHyperlink, err := raw.HasHyperlink(); err == nil {
		cell.HasHyperlink = hasHyperlink
	}

	return cell
}

// codepointsToString converts a slice of codepoints to a string.
func codepointsToString(codepoints []uint32) string {
	var b strings.Builder
	for _, cp := range codepoints {
		b.WriteRune(rune(cp))
	}
	return b.String()
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
