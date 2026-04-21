package shux

import (
	"strings"

	"github.com/mitchellh/go-libghostty"
)

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

	if err := renderState.Update(term); err != nil {
		return &PaneContent{
			Lines:     make([]string, rows),
			Cells:     make([][]PaneCell, rows),
			CursorRow: 0,
			CursorCol: 0,
		}
	}

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

	scrollbackRows := uint(0)
	if scrollback, err := term.ScrollbackRows(); err == nil {
		scrollbackRows = scrollback
	}

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

		if err := rowIterator.Cells(rowCells); err == nil {
			colIdx := 0
			for colIdx < cols && rowCells.Next() {
				cell := pr.buildCellFromRowCells(rowCells)
				lineCells[colIdx] = cell
				colIdx++
			}
		}

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

	raw, err := rowCells.Raw()
	if err != nil {
		return cell
	}

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

	if graphemes, err := rowCells.Graphemes(); err == nil && len(graphemes) > 0 {
		cell.Text = codepointsToString(graphemes)
	} else if cell.Width > 0 {
		cell.Text = " "
	}

	if style, err := rowCells.Style(); err == nil {
		cell.Bold = style.Bold()
		cell.Italic = style.Italic()
		cell.Underline = style.Underline() != libghostty.UnderlineNone
		cell.Blink = style.Blink()
		cell.Reverse = style.Inverse()
	}

	if fg, err := rowCells.FgColor(); err == nil && fg != nil {
		cell.HasFgColor = true
		cell.FgColor = *fg
	}
	if bg, err := rowCells.BgColor(); err == nil && bg != nil {
		cell.HasBgColor = true
		cell.BgColor = *bg
	}

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
