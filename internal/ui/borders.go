package ui

import (
	"shux/internal/cfg"
	"shux/internal/protocol"
)

// Tmux border cell types (tmux.h CELL_*).
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

// borderCellTypeAt classifies a border cell from neighboring border cells, matching
// tmux screen_redraw_type_of_cell (tmux.h CELL_*).
func borderCellTypeAt(panes []LayoutPane, x, y int) int {
	borders := 0
	if isBorderCellAt(panes, x-1, y) {
		borders |= 8
	}
	if isBorderCellAt(panes, x+1, y) {
		borders |= 4
	}
	if isBorderCellAt(panes, x, y-1) {
		borders |= 2
	}
	if isBorderCellAt(panes, x, y+1) {
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
		return borderCellTopBottom // tmux: vertical bar on lone left/right edge
	case 2, 1:
		return borderCellLeftRight // tmux: horizontal bar on lone top/bottom edge
	default:
		return borderCellOutside
	}
}

func (c *runeCanvas) borderStyleForPane(paneID, activePane protocol.PaneID) string {
	if paneID == activePane {
		return c.ui.PaneActiveBorderStyle
	}
	return c.ui.PaneBorderStyle
}

func (c *runeCanvas) borderStyleAt(panes []LayoutPane, activePane protocol.PaneID, x, y int) string {
	for _, p := range panes {
		if p.PaneID == activePane && isPaneBorderCell(p, x, y) {
			return c.ui.PaneActiveBorderStyle
		}
	}
	return c.ui.PaneBorderStyle
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
	numberMode := mode == cfg.PaneBorderLinesNumber
	seen := make(map[protocol.PaneID]int, len(panes))
	for i, p := range panes {
		seen[p.PaneID] = i
	}
	for _, p := range panes {
		for y := p.Row; y < p.Row+p.Rows; y++ {
			for x := p.Col; x < p.Col+p.Cols; x++ {
				if !isPaneBorderCell(p, x, y) {
					continue
				}
				cellType := borderCellTypeAt(panes, x, y)
				if cellType < 0 || cellType >= len(runset) {
					continue
				}
				ch := runset[cellType]
				if numberMode && cellType != borderCellOutside && x == p.Col && y == p.Row {
					if idx := seen[p.PaneID]; idx == 0 {
						ch = '0'
					} else {
						ch = '*'
					}
				}
				c.setStyled(x, y, ch, c.borderStyleForPane(p.PaneID, activePane))
			}
		}
	}
}

func paneSpanOverlap(aStart, aLen, bStart, bLen int) bool {
	return aStart < bStart+bLen && bStart < aStart+aLen
}

type splitLineV struct {
	x, rowStart, rowEnd int
}

type splitLineH struct {
	y, colStart, colEnd int
}

// drawInternalSplitLines draws one line on shared pane boundaries only (no outer box).
func (c *runeCanvas) drawInternalSplitLines(panes []LayoutPane, activePane protocol.PaneID) {
	if len(panes) < 2 {
		return
	}
	runset, ok := borderRuneSet(c.ui.EffectivePaneBorderLines())
	if !ok {
		return
	}
	vert := runset[borderCellTopBottom]
	horiz := runset[borderCellLeftRight]
	joint := runset[borderCellJoin]

	type vSplit = splitLineV
	type hSplit = splitLineH
	var vertSplits []vSplit
	var horizSplits []hSplit
	for i := 0; i < len(panes); i++ {
		a := panes[i]
		for j := i + 1; j < len(panes); j++ {
			b := panes[j]
			if a.Col+a.Cols == b.Col && paneSpanOverlap(a.Row, a.Rows, b.Row, b.Rows) {
				vertSplits = append(vertSplits, vSplit{
					x:        b.Col,
					rowStart: max(a.Row, b.Row),
					rowEnd:   min(a.Row+a.Rows, b.Row+b.Rows),
				})
			}
			if a.Row+a.Rows == b.Row && paneSpanOverlap(a.Col, a.Cols, b.Col, b.Cols) {
				horizSplits = append(horizSplits, hSplit{
					y:        b.Row,
					colStart: max(a.Col, b.Col),
					colEnd:   min(a.Col+a.Cols, b.Col+b.Cols),
				})
			}
		}
	}
	intersections := make(map[[2]int]struct{})
	for _, v := range vertSplits {
		for _, h := range horizSplits {
			if v.x >= h.colStart && v.x < h.colEnd && h.y >= v.rowStart && h.y < v.rowEnd {
				intersections[[2]int{v.x, h.y}] = struct{}{}
			}
		}
	}
	for _, h := range horizSplits {
		for col := h.colStart; col < h.colEnd; col++ {
			ch := horiz
			if _, ok := intersections[[2]int{col, h.y}]; ok {
				ch = joint
			}
			c.setStyled(col, h.y, ch, c.borderStyleAt(panes, activePane, col, h.y))
		}
	}
	for _, v := range vertSplits {
		for _, seg := range verticalSegments(v, horizSplits) {
			for row := seg[0]; row < seg[1]; row++ {
				if _, ok := intersections[[2]int{v.x, row}]; ok {
					continue
				}
				c.setStyled(v.x, row, vert, c.borderStyleAt(panes, activePane, v.x, row))
			}
		}
	}
}

// verticalSegments returns row ranges [start,end) where a vertical split should draw,
// stopping one row above each crossing horizontal split so the vertical does not
// protrude through the horizontal line.
func verticalSegments(v splitLineV, horizSplits []splitLineH) [][2]int {
	var crossings []int
	for _, h := range horizSplits {
		if h.y < v.rowStart || h.y > v.rowEnd {
			continue
		}
		if v.x < h.colStart || v.x >= h.colEnd {
			continue
		}
		crossings = append(crossings, h.y)
	}
	if len(crossings) == 0 {
		return [][2]int{{v.rowStart, v.rowEnd}}
	}
	// Sort ascending (insertion sort — at most a handful of splits).
	for i := 1; i < len(crossings); i++ {
		for j := i; j > 0 && crossings[j] < crossings[j-1]; j-- {
			crossings[j], crossings[j-1] = crossings[j-1], crossings[j]
		}
	}
	var segs [][2]int
	start := v.rowStart
	for _, hy := range crossings {
		if end := hy - 1; start < end {
			segs = append(segs, [2]int{start, end})
		}
		start = hy + 1
	}
	if start < v.rowEnd {
		segs = append(segs, [2]int{start, v.rowEnd})
	}
	return segs
}
