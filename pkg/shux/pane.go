package shux

import (
	"os/exec"

	"github.com/mitchellh/go-libghostty"
)

// This file provides backward compatibility during the migration to
// the new runtime/controller architecture.
//
// New code should use:
// - PaneRuntime for irreplaceable live state (PTY, process)
// - PaneController for restartable coordination
//
// The old Pane type is now a wrapper that combines both.

// Pane is the legacy pane type that combines runtime and controller.
// Deprecated: Use PaneRuntime and PaneController directly for new code.
type Pane struct {
	controller *PaneController
	runtime    *PaneRuntime
	ref        *PaneRef
}

// NewPane creates a new pane with both runtime and controller.
// This maintains backward compatibility with existing code.
func NewPane(id uint32, rows, cols int, shell, cwd string, logger ShuxLogger) *Pane {
	return &Pane{
		runtime: &PaneRuntime{
			id:     id,
			rows:   rows,
			cols:   cols,
			shell:  shell,
			cwd:    cwd,
			logger: logger,
		},
	}
}

// StartPane creates and starts a new pane with the given parameters.
// This is the main entry point for creating panes.
func StartPane(id uint32, rows, cols int, shell, cwd string, parent *WindowRef, logger ShuxLogger) *PaneRef {
	// Create the runtime first (this starts the PTY and process)
	cfg := PaneRuntimeConfig{
		ID:     id,
		Rows:   rows,
		Cols:   cols,
		Shell:  shell,
		CWD:    cwd,
		Logger: logger,
	}

	runtime, err := NewPaneRuntime(cfg)
	if err != nil {
		if logger != nil {
			logger.Errorf("pane: id=%d failed to create runtime: %v", id, err)
		}
		// Send exit notification so window can clean up
		if parent != nil {
			parent.Send(PaneExited{ID: id})
		}
		// Return a stub ref that's already done
		ref := &PaneRef{loopRef: newLoopRef(1)}
		close(ref.done)
		return ref
	}

	// Start the controller around the runtime
	return StartPaneController(id, runtime, parent, logger)
}

// PaneRef methods promoted for compatibility

// GetPaneModeMsg requests the pane mode from the controller.
// Renamed to avoid conflict with the type in messages.go.
func GetPaneModeMsg(ref *PaneRef) *PaneMode {
	result, _ := askValue(ref, GetPaneMode{})
	if mode, ok := result.(*PaneMode); ok {
		return mode
	}
	return nil
}

// GetPaneContentMsg requests the pane content from the controller.
// Renamed to avoid conflict with the type in messages.go.
func GetPaneContentMsg(ref *PaneRef) *PaneContent {
	result, _ := askValue(ref, GetPaneContent{})
	if content, ok := result.(*PaneContent); ok {
		return content
	}
	return nil
}

// GetPaneSnapshotDataMsg requests snapshot data from the controller.
// Renamed to avoid conflict with the type in messages.go.
func GetPaneSnapshotDataMsg(ref *PaneRef) PaneSnapshotData {
	result, _ := askValue(ref, GetPaneSnapshotData{})
	if data, ok := result.(PaneSnapshotData); ok {
		return data
	}
	return PaneSnapshotData{}
}

// WriteToPaneMsg writes data to the pane's PTY.
// Renamed to avoid conflict with the type in messages.go.
func WriteToPaneMsg(ref *PaneRef, data []byte) {
	if ref != nil {
		ref.Send(WriteToPane{Data: data})
	}
}

// SendKeyInputMsg sends keyboard input to the pane.
func SendKeyInputMsg(ref *PaneRef, input KeyInput) {
	if ref != nil {
		ref.Send(input)
	}
}

// SendMouseInputMsg sends mouse input to the pane.
func SendMouseInputMsg(ref *PaneRef, input MouseInput) {
	if ref != nil {
		ref.Send(input)
	}
}

// KillPaneMsg kills the pane and its runtime.
// Renamed to avoid conflict with the type in messages.go.
func KillPaneMsg(ref *PaneRef) {
	if ref != nil {
		ref.Send(KillPane{})
	}
}

// Helper functions for pane integration

// BuildPaneContent builds content from a runtime directly.
// Used by window rendering when it has direct runtime access.
func BuildPaneContent(runtime *PaneRuntime) *PaneContent {
	if runtime == nil {
		return &PaneContent{
			Lines: make([]string, 0),
			Cells: make([][]PaneCell, 0),
		}
	}
	return runtime.BuildContent()
}

// RenderPaneToCells renders a pane's content into a cell grid.
// This is used by the window renderer.
func RenderPaneToCells(ref *PaneRef, rows, cols int) [][]PaneCell {
	content := GetPaneContentMsg(ref)
	if content == nil {
		return make([][]PaneCell, 0)
	}

	screen := make([][]PaneCell, rows)
	for r := 0; r < rows; r++ {
		screen[r] = make([]PaneCell, cols)
		for c := 0; c < cols; c++ {
			screen[r][c] = blankPaneCell()
		}
	}

	for r := 0; r < rows && r < len(content.Cells); r++ {
		for c := 0; c < cols && c < len(content.Cells[r]); c++ {
			screen[r][c] = content.Cells[r][c]
		}
	}

	return screen
}

// GetPaneCursorPosition returns the cursor position for a pane.
func GetPaneCursorPosition(ref *PaneRef) (row, col int, visible bool) {
	content := GetPaneContentMsg(ref)
	if content == nil {
		return 0, 0, false
	}
	return content.CursorRow, content.CursorCol, !content.CursorHidden
}

// IsPaneInAltScreen checks if a pane is in alt screen mode.
func IsPaneInAltScreen(ref *PaneRef) bool {
	mode := GetPaneModeMsg(ref)
	if mode == nil {
		return false
	}
	return mode.InAltScreen
}

// blankPaneCell creates a blank cell.
func blankPaneCell() PaneCell {
	return PaneCell{Text: " ", Width: 1}
}

// Legacy paneRuntime type - now wraps PaneRuntime for compatibility
// This allows gradual migration of code that references paneRuntime

// paneRuntimeCompat wraps the new PaneRuntime for internal compatibility.
type paneRuntimeCompat struct {
	*PaneRuntime
}

// Close closes the runtime (legacy method).
func (r *paneRuntimeCompat) Close() {
	if r.PaneRuntime != nil {
		r.PaneRuntime.Close()
	}
}

// install installs the runtime into a legacy Pane struct.
func (r *paneRuntimeCompat) install(p *Pane) {
	// This is a no-op now - the runtime is accessed directly
}

// newRuntime creates a runtime for a pane (legacy compatibility).
func (p *Pane) newRuntime() (*paneRuntimeCompat, *exec.Cmd, error) {
	cfg := PaneRuntimeConfig{
		ID:     p.runtime.id,
		Rows:   p.runtime.rows,
		Cols:   p.runtime.cols,
		Shell:  p.runtime.shell,
		CWD:    p.runtime.cwd,
		Logger: p.runtime.logger,
	}

	runtime, err := NewPaneRuntime(cfg)
	if err != nil {
		return nil, nil, err
	}

	p.runtime = runtime
	return &paneRuntimeCompat{PaneRuntime: runtime}, nil, nil
}

// term accessors for legacy compatibility

func (p *Pane) term() *libghostty.Terminal {
	if p.runtime != nil {
		return p.runtime.Term()
	}
	return nil
}

func (p *Pane) renderState() *libghostty.RenderState {
	if p.runtime != nil {
		return p.runtime.RenderState()
	}
	return nil
}

func (p *Pane) keyEncoder() *libghostty.KeyEncoder {
	if p.runtime != nil {
		return p.runtime.KeyEncoder()
	}
	return nil
}

func (p *Pane) mouseEncoder() *libghostty.MouseEncoder {
	if p.runtime != nil {
		return p.runtime.MouseEncoder()
	}
	return nil
}

func (p *Pane) pty() Pty {
	if p.runtime != nil {
		p.runtime.mu.RLock()
		defer p.runtime.mu.RUnlock()
		return p.runtime.pty
	}
	return nil
}

func (p *Pane) pid() int {
	if p.runtime != nil {
		return p.runtime.PID()
	}
	return 0
}

func (p *Pane) getCWD() string {
	if p.runtime != nil {
		return p.runtime.GetCWD()
	}
	return ""
}

func (p *Pane) IsAltScreen() bool {
	if p.runtime != nil {
		return p.runtime.IsAltScreen()
	}
	return false
}

func (p *Pane) IsCursorVisible() bool {
	if p.runtime != nil {
		return p.runtime.IsCursorVisible()
	}
	return false
}
