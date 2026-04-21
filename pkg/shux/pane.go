package shux

// Pane is the legacy pane wrapper kept for compatibility while the runtime /
// controller split settles.
type Pane struct {
	runtime *PaneRuntime
}

// NewPane creates a legacy pane wrapper.
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
func StartPane(id uint32, rows, cols int, shell, cwd string, parent *WindowRef, logger ShuxLogger) *PaneRef {
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
		if parent != nil {
			parent.Send(PaneExited{ID: id})
		}
		ref := &PaneRef{loopRef: newLoopRef(1)}
		close(ref.done)
		return ref
	}

	return StartPaneController(id, runtime, parent, logger)
}

func GetPaneModeMsg(ref *PaneRef) *PaneMode {
	result, _ := askValue(ref, GetPaneMode{})
	if mode, ok := result.(*PaneMode); ok {
		return mode
	}
	return nil
}

func GetPaneContentMsg(ref *PaneRef) *PaneContent {
	result, _ := askValue(ref, GetPaneContent{})
	if content, ok := result.(*PaneContent); ok {
		return content
	}
	return nil
}

func GetPaneSnapshotDataMsg(ref *PaneRef) PaneSnapshotData {
	result, _ := askValue(ref, GetPaneSnapshotData{})
	if data, ok := result.(PaneSnapshotData); ok {
		return data
	}
	return PaneSnapshotData{}
}

func WriteToPaneMsg(ref *PaneRef, data []byte) {
	if ref != nil {
		ref.Send(WriteToPane{Data: data})
	}
}

func SendKeyInputMsg(ref *PaneRef, input KeyInput) {
	if ref != nil {
		ref.Send(input)
	}
}

func SendMouseInputMsg(ref *PaneRef, input MouseInput) {
	if ref != nil {
		ref.Send(input)
	}
}

func KillPaneMsg(ref *PaneRef) {
	if ref != nil {
		ref.Send(KillPane{})
	}
}

func BuildPaneContent(runtime *PaneRuntime) *PaneContent {
	if runtime == nil {
		return &PaneContent{
			Lines: make([]string, 0),
			Cells: make([][]PaneCell, 0),
		}
	}
	return runtime.BuildContent()
}

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

func GetPaneCursorPosition(ref *PaneRef) (row, col int, visible bool) {
	content := GetPaneContentMsg(ref)
	if content == nil {
		return 0, 0, false
	}
	return content.CursorRow, content.CursorCol, !content.CursorHidden
}

func IsPaneInAltScreen(ref *PaneRef) bool {
	mode := GetPaneModeMsg(ref)
	if mode == nil {
		return false
	}
	return mode.InAltScreen
}

func blankPaneCell() PaneCell {
	return PaneCell{Text: " ", Width: 1}
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
