package shux

import "github.com/mitchellh/go-libghostty"

type borderState struct {
	h      bool
	v      bool
	active bool
}

type renderedActivePane struct {
	title    string
	cursorOn bool
	cursorR  int
	cursorC  int
}

func newRenderedActivePane() renderedActivePane {
	return renderedActivePane{cursorR: -1, cursorC: -1}
}

func (p renderedActivePane) toWindowView(content string) WindowView {
	return WindowView{
		Content:   content,
		CursorRow: p.cursorR,
		CursorCol: p.cursorC,
		CursorOn:  p.cursorOn,
		Title:     p.title,
	}
}

func blankCell() PaneCell {
	return PaneCell{Text: " ", Width: 1}
}

func lineCell(ch string) PaneCell {
	return PaneCell{Text: ch, Width: 1}
}

func activeLineCell(ch string) PaneCell {
	return PaneCell{
		Text:       ch,
		Width:      1,
		HasFgColor: true,
		FgColor:    libghostty.ColorRGB{R: 100, G: 180, B: 255},
		Bold:       true,
	}
}

func dividerGlyph(borders [][]borderState, row, col int) string {
	state := borders[row][col]
	if !state.h && !state.v {
		return " "
	}

	height := len(borders)
	width := 0
	if height > 0 {
		width = len(borders[0])
	}

	left := state.h && col > 0 && borders[row][col-1].h
	right := state.h && col+1 < width && borders[row][col+1].h
	up := state.v && row > 0 && borders[row-1][col].v
	down := state.v && row+1 < height && borders[row+1][col].v

	switch {
	case left && right && up && down:
		return "┼"
	case left && up && down:
		return "┤"
	case right && up && down:
		return "├"
	case left && right && down:
		return "┬"
	case left && right && up:
		return "┴"
	case state.h:
		return "─"
	case state.v:
		return "│"
	default:
		return " "
	}
}

func newBlankScreen(rows, cols int) [][]PaneCell {
	screen := make([][]PaneCell, rows)
	for r := 0; r < rows; r++ {
		screen[r] = make([]PaneCell, cols)
		for c := 0; c < cols; c++ {
			screen[r][c] = blankCell()
		}
	}
	return screen
}

func renderPanesToScreen(screen [][]PaneCell, layout []paneLayout, panes map[uint32]*PaneRef, activePaneID uint32) renderedActivePane {
	activePane := newRenderedActivePane()
	if len(screen) == 0 {
		return activePane
	}
	rows := len(screen)
	cols := len(screen[0])

	for _, pl := range layout {
		paneRef, ok := panes[pl.paneID]
		if !ok {
			continue
		}
		result, _ := askValue(paneRef, GetPaneContent{})
		content, _ := result.(*PaneContent)
		if content == nil {
			continue
		}

		for r := 0; r < pl.rows && pl.row+r < rows && r < len(content.Cells); r++ {
			row := content.Cells[r]
			for c := 0; c < pl.cols && pl.col+c < cols && c < len(row); c++ {
				screen[pl.row+r][pl.col+c] = row[c]
			}
		}

		if pl.paneID != activePaneID {
			continue
		}
		activePane.title = content.Title
		activePane.cursorOn = !content.CursorHidden
		if content.CursorRow >= 0 && content.CursorRow < pl.rows {
			activePane.cursorR = pl.row + content.CursorRow
		}
		if content.CursorCol >= 0 && content.CursorCol < pl.cols {
			activePane.cursorC = pl.col + content.CursorCol
		}
	}

	return activePane
}

func newBorderGrid(rows, cols int) [][]borderState {
	borders := make([][]borderState, rows)
	for r := 0; r < rows; r++ {
		borders[r] = make([]borderState, cols)
	}
	return borders
}

func collectDividerSegmentsList(root *splitNode, rows, cols int) []dividerSegment {
	segments := make([]dividerSegment, 0)
	collectDividerSegments(root, 0, 0, rows, cols, &segments)
	return segments
}

func applyDividerSegment(borders [][]borderState, seg dividerSegment) {
	if len(borders) == 0 {
		return
	}
	rows := len(borders)
	cols := len(borders[0])

	if seg.horizontal {
		if seg.row < 0 || seg.row >= rows {
			return
		}
		for c := 0; c < seg.length && seg.col+c < cols; c++ {
			if seg.col+c >= 0 {
				borders[seg.row][seg.col+c].h = true
			}
		}
		return
	}

	if seg.col < 0 || seg.col >= cols {
		return
	}
	for r := 0; r < seg.length && seg.row+r < rows; r++ {
		if seg.row+r >= 0 {
			borders[seg.row+r][seg.col].v = true
		}
	}
}

func buildDividerBorders(root *splitNode, rows, cols int) [][]borderState {
	borders := newBorderGrid(rows, cols)
	for _, seg := range collectDividerSegmentsList(root, rows, cols) {
		applyDividerSegment(borders, seg)
	}
	stitchDividerBorders(borders)
	return borders
}

func stitchDividerBorders(borders [][]borderState) {
	rows := len(borders)
	if rows == 0 {
		return
	}
	cols := len(borders[0])

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if borders[r][c].v {
				if c > 0 && borders[r][c-1].h {
					borders[r][c].h = true
				}
				if c+1 < cols && borders[r][c+1].h {
					borders[r][c].h = true
				}
			}
			if borders[r][c].h {
				if r > 0 && borders[r-1][c].v {
					borders[r][c].v = true
				}
				if r+1 < rows && borders[r+1][c].v {
					borders[r][c].v = true
				}
			}
		}
	}
}

func markActiveHorizontalBorder(borders [][]borderState, row, startCol, length int) {
	if row < 0 || row >= len(borders) {
		return
	}
	for offset := 0; offset < length; offset++ {
		col := startCol + offset
		if col < 0 || col >= len(borders[row]) || !borders[row][col].h {
			continue
		}
		borders[row][col].active = true
	}
}

func markActiveVerticalBorder(borders [][]borderState, startRow, col, length int) {
	if len(borders) == 0 {
		return
	}
	for offset := 0; offset < length; offset++ {
		row := startRow + offset
		if row < 0 || row >= len(borders) {
			continue
		}
		if col < 0 || col >= len(borders[row]) || !borders[row][col].v {
			continue
		}
		borders[row][col].active = true
	}
}

func markActivePaneBorders(borders [][]borderState, layout []paneLayout, activePaneID uint32) {
	activeLayout, ok := findLayout(layout, activePaneID)
	if !ok {
		return
	}
	markActiveHorizontalBorder(borders, activeLayout.row-1, activeLayout.col, activeLayout.cols)
	markActiveHorizontalBorder(borders, activeLayout.row+activeLayout.rows, activeLayout.col, activeLayout.cols)
	markActiveVerticalBorder(borders, activeLayout.row, activeLayout.col-1, activeLayout.rows)
	markActiveVerticalBorder(borders, activeLayout.row, activeLayout.col+activeLayout.cols, activeLayout.rows)
}

func drawDividerBorders(screen [][]PaneCell, borders [][]borderState) {
	for r := range borders {
		for c := range borders[r] {
			state := borders[r][c]
			if !state.h && !state.v {
				continue
			}
			glyph := dividerGlyph(borders, r, c)
			if state.active {
				screen[r][c] = activeLineCell(glyph)
			} else {
				screen[r][c] = lineCell(glyph)
			}
		}
	}
}

func renderScreenLines(screen [][]PaneCell, cols int) []string {
	lines := make([]string, len(screen))
	for r := range screen {
		lines[r] = renderRow(screen[r], cols)
	}
	return lines
}

func (w *Window) buildWindowView() WindowView {
	if len(w.layout) == 0 || w.rows <= 0 || w.cols <= 0 {
		return WindowView{}
	}

	screen := newBlankScreen(w.rows, w.cols)
	activePane := renderPanesToScreen(screen, w.layout, w.panes, w.active)
	borders := buildDividerBorders(w.root, w.rows, w.cols)
	markActivePaneBorders(borders, w.layout, w.active)
	drawDividerBorders(screen, borders)

	return activePane.toWindowView(joinLines(renderScreenLines(screen, w.cols)))
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for i := 1; i < len(lines); i++ {
		out += "\n" + lines[i]
	}
	return out
}
