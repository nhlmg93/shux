package ui

import (
	"shux/internal/cfg"
	"shux/internal/protocol"
)

// tmux.h CELL_* — index into border rune tables (CELL_BORDERS, SIMPLE_BORDERS, …).
const (
	borderCellInside = iota
	borderCellTopBottom
	borderCellLeftRight
	borderCellTopLeft
	borderCellTopRight
	borderCellBottomLeft
	borderCellBottomRight
	borderCellTopJoin
	borderCellBottomJoin
	borderCellLeftJoin
	borderCellRightJoin
	borderCellJoin
	borderCellOutside
)

// CELL_BORDERS " xqlkmjwvtun~" — tmux single style.
var singleBorderRunes = [13]rune{
	' ', '│', '─', '┌', '┐', '└', '┘', '┬', '┴', '├', '┤', '┼', ' ',
}

// SIMPLE_BORDERS " |-+++++++++."
var simpleBorderRunes = [13]rune{
	' ', '|', '-', '+', '+', '+', '+', '+', '+', '+', '+', '+', '.',
}

// tty_acs_double_borders_list.
var doubleBorderRunes = [13]rune{
	' ', '║', '═', '╔', '╗', '╚', '╝', '╦', '╩', '╠', '╣', '╬', '·',
}

// tty_acs_heavy_borders_list.
var heavyBorderRunes = [13]rune{
	' ', '┃', '━', '┏', '┓', '┗', '┛', '┳', '┻', '┣', '┫', '╋', '·',
}

func borderRuneSet(mode string) ([13]rune, bool) {
	switch cfg.NormalizePaneBorderLines(mode) {
	case cfg.PaneBorderLinesSimple:
		return simpleBorderRunes, true
	case cfg.PaneBorderLinesDouble:
		return doubleBorderRunes, true
	case cfg.PaneBorderLinesHeavy:
		return heavyBorderRunes, true
	case cfg.PaneBorderLinesSingle, cfg.PaneBorderLinesNumber:
		return singleBorderRunes, true
	default:
		return [13]rune{}, false
	}
}

func isPaneBorderCell(p LayoutPane, x, y int) bool {
	if x < p.Col || x >= p.Col+p.Cols || y < p.Row || y >= p.Row+p.Rows {
		return false
	}
	return x == p.Col || x == p.Col+p.Cols-1 || y == p.Row || y == p.Row+p.Rows-1
}

func isBorderCellAt(panes []LayoutPane, x, y int) bool {
	if x < 0 || y < 0 {
		return false
	}
	for _, p := range panes {
		if isPaneBorderCell(p, x, y) {
			return true
		}
	}
	return false
}

// borderCellTypeFromNeighbors implements tmux screen_redraw_type_of_cell.
func borderCellTypeFromNeighbors(x, y int, isBorder func(int, int) bool) int {
	borders := 0
	if isBorder(x-1, y) {
		borders |= 8
	}
	if isBorder(x+1, y) {
		borders |= 4
	}
	if isBorder(x, y-1) {
		borders |= 2
	}
	if isBorder(x, y+1) {
		borders |= 1
	}
	switch borders {
	case 15:
		return borderCellJoin
	case 14:
		return borderCellBottomJoin
	case 13:
		return borderCellTopJoin
	case 12:
		return borderCellLeftRight
	case 11:
		return borderCellRightJoin
	case 10:
		return borderCellBottomRight
	case 9:
		return borderCellTopRight
	case 7:
		return borderCellLeftJoin
	case 6:
		return borderCellBottomLeft
	case 5:
		return borderCellTopLeft
	case 3:
		return borderCellTopBottom
	case 8, 4:
		return borderCellTopBottom
	case 2, 1:
		return borderCellLeftRight
	default:
		return borderCellOutside
	}
}

// borderCellTypeAt classifies a full pane-box border cell (tests).
func borderCellTypeAt(panes []LayoutPane, x, y int) int {
	isBorder := func(bx, by int) bool { return isBorderCellAt(panes, bx, by) }
	return borderCellTypeFromNeighbors(x, y, isBorder)
}

type borderCellSet map[[2]int]struct{}

func (s borderCellSet) contains(x, y int) bool {
	_, ok := s[[2]int{x, y}]
	return ok
}

func addBorderCell(cells borderCellSet, x, y, cols, rows int) {
	if x < 0 || y < 0 || x >= cols || y >= rows {
		return
	}
	cells[[2]int{x, y}] = struct{}{}
}

// internalSplitBorderCells returns shared pane boundary cells (no outer window box).
func internalSplitBorderCells(panes []LayoutPane, cols, rows int) borderCellSet {
	cells := make(borderCellSet)
	for i := 0; i < len(panes); i++ {
		a := panes[i]
		for j := i + 1; j < len(panes); j++ {
			b := panes[j]
			if a.Col+a.Cols == b.Col {
				x := b.Col
				for y := max(a.Row, b.Row); y < min(a.Row+a.Rows, b.Row+b.Rows); y++ {
					addBorderCell(cells, x, y, cols, rows)
				}
			} else if b.Col+b.Cols == a.Col {
				x := a.Col
				for y := max(a.Row, b.Row); y < min(a.Row+a.Rows, b.Row+b.Rows); y++ {
					addBorderCell(cells, x, y, cols, rows)
				}
			}
			if a.Row+a.Rows == b.Row {
				y := b.Row
				for x := max(a.Col, b.Col); x < min(a.Col+a.Cols, b.Col+b.Cols); x++ {
					addBorderCell(cells, x, y, cols, rows)
				}
			} else if b.Row+b.Rows == a.Row {
				y := a.Row
				for x := max(a.Col, b.Col); x < min(a.Col+a.Cols, b.Col+b.Cols); x++ {
					addBorderCell(cells, x, y, cols, rows)
				}
			}
		}
	}
	return cells
}

// internalSplitAxes reports whether (x,y) lies on a shared horizontal and/or vertical split.
func internalSplitAxes(panes []LayoutPane, x, y int) (horizontal, vertical bool) {
	for i := 0; i < len(panes); i++ {
		a := panes[i]
		for j := i + 1; j < len(panes); j++ {
			b := panes[j]
			if a.Col+a.Cols == b.Col && x == b.Col &&
				y >= max(a.Row, b.Row) && y < min(a.Row+a.Rows, b.Row+b.Rows) {
				vertical = true
			} else if b.Col+b.Cols == a.Col && x == a.Col &&
				y >= max(a.Row, b.Row) && y < min(a.Row+a.Rows, b.Row+b.Rows) {
				vertical = true
			}
			if a.Row+a.Rows == b.Row && y == b.Row &&
				x >= max(a.Col, b.Col) && x < min(a.Col+a.Cols, b.Col+b.Cols) {
				horizontal = true
			} else if b.Row+b.Rows == a.Row && y == a.Row &&
				x >= max(a.Col, b.Col) && x < min(a.Col+a.Cols, b.Col+b.Cols) {
				horizontal = true
			}
		}
	}
	return horizontal, vertical
}

// internalSplitCellType classifies split-only dividers. Line endpoints stay straight
// (─ on horizontal splits, │ on vertical) instead of perpendicular caps.
func internalSplitCellType(panes []LayoutPane, x, y int, isBorder func(int, int) bool) int {
	cellType := borderCellTypeFromNeighbors(x, y, isBorder)
	horiz, vert := internalSplitAxes(panes, x, y)
	if horiz && !vert && cellType == borderCellTopBottom {
		return borderCellLeftRight
	}
	if vert && !horiz && cellType == borderCellLeftRight {
		return borderCellTopBottom
	}
	return cellType
}

func paneIndexMap(panes []LayoutPane) map[protocol.PaneID]int {
	seen := make(map[protocol.PaneID]int, len(panes))
	for i, p := range panes {
		seen[p.PaneID] = i
	}
	return seen
}

func (c *runeCanvas) borderStyleForCell(panes []LayoutPane, activePane protocol.PaneID, x, y int) string {
	for _, p := range panes {
		if p.PaneID == activePane && isPaneBorderCell(p, x, y) {
			return c.ui.PaneActiveBorderStyle
		}
	}
	return c.ui.PaneBorderStyle
}

func borderRune(mode string, runset [13]rune, cellType int, paneIdx int, atPaneOrigin bool) (rune, bool) {
	if cellType == borderCellOutside {
		return 0, false
	}
	ch := runset[cellType]
	if mode == cfg.PaneBorderLinesNumber && atPaneOrigin {
		if paneIdx == 0 {
			ch = '0'
		} else {
			ch = '*'
		}
	}
	return ch, true
}

func owningPaneAt(panes []LayoutPane, paneIdx map[protocol.PaneID]int, x, y int) (protocol.PaneID, int, bool) {
	for _, p := range panes {
		if !isPaneBorderCell(p, x, y) {
			continue
		}
		atOrigin := x == p.Col && y == p.Row
		return p.PaneID, paneIdx[p.PaneID], atOrigin
	}
	return "", 0, false
}

// drawBorderCell classifies and paints one border cell (tmux screen_redraw_border_set).
func (c *runeCanvas) drawBorderCell(
	panes []LayoutPane,
	activePane protocol.PaneID,
	mode string,
	runset [13]rune,
	x, y int,
	isBorder func(int, int) bool,
	paneIdx map[protocol.PaneID]int,
) {
	if !isBorder(x, y) {
		return
	}
	cellType := borderCellTypeFromNeighbors(x, y, isBorder)
	_, idx, atOrigin := owningPaneAt(panes, paneIdx, x, y)
	ch, ok := borderRune(mode, runset, cellType, idx, atOrigin)
	if !ok {
		return
	}
	c.setStyled(x, y, ch, c.borderStyleForCell(panes, activePane, x, y))
}

func (c *runeCanvas) drawInternalSplitCell(
	panes []LayoutPane,
	activePane protocol.PaneID,
	mode string,
	runset [13]rune,
	x, y int,
	isBorder func(int, int) bool,
	paneIdx map[protocol.PaneID]int,
) {
	if !isBorder(x, y) {
		return
	}
	cellType := internalSplitCellType(panes, x, y, isBorder)
	_, idx, atOrigin := owningPaneAt(panes, paneIdx, x, y)
	ch, ok := borderRune(mode, runset, cellType, idx, atOrigin)
	if !ok {
		return
	}
	c.setStyled(x, y, ch, c.borderStyleForCell(panes, activePane, x, y))
}

func (c *runeCanvas) drawWindowBorders(panes []LayoutPane, activePane protocol.PaneID) {
	mode := c.ui.EffectivePaneBorderLines()
	if mode == cfg.PaneBorderLinesSpaces {
		for _, p := range panes {
			for y := p.Row; y < p.Row+p.Rows; y++ {
				for x := p.Col; x < p.Col+p.Cols; x++ {
					if isPaneBorderCell(p, x, y) {
						c.set(x, y, ' ')
					}
				}
			}
		}
		return
	}
	runset, ok := borderRuneSet(mode)
	if !ok {
		return
	}
	paneIdx := paneIndexMap(panes)
	isBorder := func(bx, by int) bool { return isBorderCellAt(panes, bx, by) }
	for _, p := range panes {
		for y := p.Row; y < p.Row+p.Rows; y++ {
			for x := p.Col; x < p.Col+p.Cols; x++ {
				if !isPaneBorderCell(p, x, y) {
					continue
				}
				c.drawBorderCell(panes, activePane, mode, runset, x, y, isBorder, paneIdx)
			}
		}
	}
}

func (c *runeCanvas) drawInternalSplitLines(panes []LayoutPane, activePane protocol.PaneID) {
	if len(panes) < 2 {
		return
	}
	mode := c.ui.EffectivePaneBorderLines()
	runset, ok := borderRuneSet(mode)
	if !ok {
		return
	}
	cells := internalSplitBorderCells(panes, c.cols, c.rows)
	if len(cells) == 0 {
		return
	}
	paneIdx := paneIndexMap(panes)
	isBorder := cells.contains
	for cell := range cells {
		c.drawInternalSplitCell(panes, activePane, mode, runset, cell[0], cell[1], isBorder, paneIdx)
	}
}
