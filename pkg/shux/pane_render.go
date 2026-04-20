package shux

import (
	"strings"

	"github.com/mitchellh/go-libghostty"
)

func (p *Pane) buildContent() *PaneContent {
	content := &PaneContent{
		Lines:          make([]string, p.rows),
		Cells:          make([][]PaneCell, p.rows),
		CursorHidden:   true,
		Title:          p.windowTitle,
		BellCount:      p.bellCount,
		ScrollbackRows: 0,
	}
	if p.term == nil || p.renderState == nil {
		p.fillBlankContent(content)
		return content
	}

	if rows, err := p.term.ScrollbackRows(); err == nil {
		content.ScrollbackRows = rows
	}
	if altScreen, _ := p.term.ModeGet(libghostty.ModeAltScreen); altScreen {
		content.InAltScreen = true
	}

	if err := p.renderState.Update(p.term); err != nil {
		p.fillBlankContent(content)
		return content
	}

	if cursorVisible, err := p.renderState.CursorVisible(); err == nil {
		content.CursorHidden = !cursorVisible
	}
	if hasValue, _ := p.renderState.CursorViewportHasValue(); hasValue {
		if x, err := p.renderState.CursorViewportX(); err == nil {
			content.CursorCol = int(x)
		}
		if y, err := p.renderState.CursorViewportY(); err == nil {
			content.CursorRow = int(y)
		}
		if !content.CursorHidden {
			content.CursorHidden = false
		}
	}

	if err := p.renderState.RowIterator(p.rowIterator); err != nil {
		p.fillBlankContent(content)
		return content
	}

	rowIdx := 0
	for rowIdx < p.rows && p.rowIterator.Next() {
		cells := blankPaneRow(p.cols)
		if err := p.rowIterator.Cells(p.rowCells); err == nil {
			col := 0
			for col < p.cols && p.rowCells.Next() {
				cells[col] = p.currentRowCell()
				col++
			}
		}
		content.Cells[rowIdx] = cells
		content.Lines[rowIdx] = cellsToLine(cells)
		rowIdx++
	}

	for ; rowIdx < p.rows; rowIdx++ {
		cells := blankPaneRow(p.cols)
		content.Cells[rowIdx] = cells
		content.Lines[rowIdx] = cellsToLine(cells)
	}

	return content
}

func (p *Pane) currentRowCell() PaneCell {
	cell := blankPaneCell()

	raw, err := p.rowCells.Raw()
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

	if graphemes, err := p.rowCells.Graphemes(); err == nil && len(graphemes) > 0 {
		cell.Text = codepointsToString(graphemes)
	} else if cell.Width > 0 {
		cell.Text = " "
	}

	if style, err := p.rowCells.Style(); err == nil {
		cell.Bold = style.Bold()
		cell.Italic = style.Italic()
		cell.Underline = style.Underline() != libghostty.UnderlineNone
		cell.Blink = style.Blink()
		cell.Reverse = style.Inverse()
	}

	if fg, err := p.rowCells.FgColor(); err == nil && fg != nil {
		cell.HasFgColor = true
		cell.FgColor = *fg
	}
	if bg, err := p.rowCells.BgColor(); err == nil && bg != nil {
		cell.HasBgColor = true
		cell.BgColor = *bg
	}
	if hasHyperlink, err := raw.HasHyperlink(); err == nil {
		cell.HasHyperlink = hasHyperlink
	}

	return cell
}

func (p *Pane) fillBlankContent(content *PaneContent) {
	for row := 0; row < p.rows; row++ {
		cells := blankPaneRow(p.cols)
		content.Cells[row] = cells
		content.Lines[row] = cellsToLine(cells)
	}
}

func blankPaneCell() PaneCell {
	return PaneCell{Text: " ", Width: 1}
}

func blankPaneRow(cols int) []PaneCell {
	row := make([]PaneCell, cols)
	for i := range row {
		row[i] = blankPaneCell()
	}
	return row
}

func cellsToLine(cells []PaneCell) string {
	var b strings.Builder
	for _, cell := range cells {
		if cell.Width == 0 {
			continue
		}
		if cell.Text == "" {
			b.WriteByte(' ')
			continue
		}
		b.WriteString(cell.Text)
	}
	return b.String()
}

func codepointsToString(codepoints []uint32) string {
	var b strings.Builder
	for _, cp := range codepoints {
		b.WriteRune(rune(cp))
	}
	return b.String()
}
