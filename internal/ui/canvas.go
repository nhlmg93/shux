package ui

import (
	"strconv"
	"strings"

	"shux/internal/protocol"
)

const cursorANSI = "\x1b[97;44m" // bright white text on blue: visible on dark and light themes.

type runeCanvas struct {
	cols int
	rows int
	buf  [][]rune
	ansi [][]string
}

func newRuneCanvas(cols, rows int) *runeCanvas {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	buf := make([][]rune, rows)
	ansi := make([][]string, rows)
	for r := range buf {
		buf[r] = make([]rune, cols)
		ansi[r] = make([]string, cols)
		for c := range buf[r] {
			buf[r][c] = ' '
		}
	}
	return &runeCanvas{cols: cols, rows: rows, buf: buf, ansi: ansi}
}

func (c *runeCanvas) drawPane(p LayoutPane, active bool) {
	c.drawPaneWithScreen(p, active, nil)
}

func (c *runeCanvas) drawPaneWithScreen(p LayoutPane, active bool, lines []string) {
	c.drawPaneWithCells(p, active, textLinesToCells(lines))
}

func (c *runeCanvas) drawPaneWithCells(p LayoutPane, active bool, lines []protocol.EventPaneScreenLine) {
	c.drawPaneWithScreenEvent(p, active, protocol.EventPaneScreenChanged{Lines: lines})
}

func (c *runeCanvas) drawPaneWithScreenEvent(p LayoutPane, active bool, screen protocol.EventPaneScreenChanged) {
	lines := screen.Lines
	x, y := p.Col, p.Row
	w, h := p.Cols, p.Rows
	if w <= 0 || h <= 0 || x >= c.cols || y >= c.rows {
		return
	}
	if x < 0 {
		w += x
		x = 0
	}
	if y < 0 {
		h += y
		y = 0
	}
	if x+w > c.cols {
		w = c.cols - x
	}
	if y+h > c.rows {
		h = c.rows - y
	}
	if w <= 0 || h <= 0 {
		return
	}

	horiz, vert := '─', '│'
	tl, tr, bl, br := '┌', '┐', '└', '┘'
	if active {
		horiz, vert = '═', '║'
		tl, tr, bl, br = '╔', '╗', '╚', '╝'
	}

	if h == 1 {
		for col := x; col < x+w; col++ {
			c.set(col, y, horiz)
		}
		c.drawText(x, y, labelForPane(p))
		return
	}
	if w == 1 {
		for row := y; row < y+h; row++ {
			c.set(x, row, vert)
		}
		return
	}

	c.set(x, y, tl)
	c.set(x+w-1, y, tr)
	c.set(x, y+h-1, bl)
	c.set(x+w-1, y+h-1, br)
	for col := x + 1; col < x+w-1; col++ {
		c.set(col, y, horiz)
		c.set(col, y+h-1, horiz)
	}
	for row := y + 1; row < y+h-1; row++ {
		c.set(x, row, vert)
		c.set(x+w-1, row, vert)
	}
	c.drawText(x+1, y, labelForPane(p))
	c.drawPaneContent(x+1, y+1, w-2, h-2, lines)
	if active {
		c.drawCursor(x+1, y+1, w-2, h-2, screen.Cursor)
	}
}

func (c *runeCanvas) drawCursor(x, y, w, h int, cursor protocol.EventPaneScreenCursor) {
	if !cursor.Visible {
		return
	}
	if cursor.Col < 0 || cursor.Row < 0 || cursor.Col >= w || cursor.Row >= h {
		return
	}
	cx, cy := x+cursor.Col, y+cursor.Row
	if cx < 0 || cx >= c.cols || cy < 0 || cy >= c.rows {
		return
	}
	if c.buf[cy][cx] == ' ' {
		c.buf[cy][cx] = '█'
	}
	c.ansi[cy][cx] = cursorANSI
}

func (c *runeCanvas) drawPaneContent(x, y, w, h int, lines []protocol.EventPaneScreenLine) {
	if w <= 0 || h <= 0 {
		return
	}
	for row := 0; row < h && row < len(lines); row++ {
		c.drawCellsClipped(x, y+row, lines[row], w)
	}
}

func labelForPane(p LayoutPane) string {
	return string(p.PaneID) + " " + itoa(p.Cols) + "x" + itoa(p.Rows) + " @" + itoa(p.Col) + "," + itoa(p.Row)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func (c *runeCanvas) drawText(x, y int, s string) {
	c.drawTextClipped(x, y, s, c.cols)
}

func (c *runeCanvas) drawTextClipped(x, y int, s string, maxWidth int) {
	if y < 0 || y >= c.rows || x >= c.cols || maxWidth <= 0 {
		return
	}
	drawn := 0
	for _, r := range s {
		if drawn >= maxWidth {
			return
		}
		if x < 0 {
			x++
			drawn++
			continue
		}
		if x >= c.cols {
			return
		}
		c.set(x, y, r)
		x++
		drawn++
	}
}

func (c *runeCanvas) drawCellsClipped(x, y int, line protocol.EventPaneScreenLine, maxWidth int) {
	if len(line.Cells) == 0 {
		c.drawTextClipped(x, y, line.Text, maxWidth)
		return
	}
	drawn := 0
	for _, cell := range line.Cells {
		if drawn >= maxWidth {
			return
		}
		text := cell.Text
		if text == "" {
			text = " "
		}
		for _, r := range text {
			if drawn >= maxWidth {
				return
			}
			c.setStyled(x+drawn, y, r, cellANSI(cell))
			drawn++
			break
		}
	}
}

func (c *runeCanvas) set(x, y int, r rune) {
	c.setStyled(x, y, r, "")
}

func (c *runeCanvas) setStyled(x, y int, r rune, ansi string) {
	if x < 0 || x >= c.cols || y < 0 || y >= c.rows {
		return
	}
	c.buf[y][x] = r
	c.ansi[y][x] = ansi
}

func (c *runeCanvas) String() string {
	lines := make([]string, c.rows)
	for y, row := range c.buf {
		var b strings.Builder
		current := ""
		styled := false
		for x, r := range row {
			style := c.ansi[y][x]
			if style != current {
				if styled {
					b.WriteString("\x1b[0m")
					styled = false
				}
				if style != "" {
					b.WriteString(style)
					styled = true
				}
				current = style
			}
			b.WriteRune(r)
		}
		if styled {
			b.WriteString("\x1b[0m")
		}
		lines[y] = b.String()
	}
	return strings.Join(lines, "\n")
}

func textLinesToCells(lines []string) []protocol.EventPaneScreenLine {
	if len(lines) == 0 {
		return nil
	}
	out := make([]protocol.EventPaneScreenLine, 0, len(lines))
	for _, line := range lines {
		out = append(out, protocol.EventPaneScreenLine{Text: line})
	}
	return out
}

func cellANSI(cell protocol.EventPaneScreenCell) string {
	codes := make([]string, 0, 8)
	if cell.Bold {
		codes = append(codes, "1")
	}
	if cell.Italic {
		codes = append(codes, "3")
	}
	if cell.Faint {
		codes = append(codes, "2")
	}
	if cell.Blink {
		codes = append(codes, "5")
	}
	if cell.Inverse {
		codes = append(codes, "7")
	}
	if cell.Invisible {
		codes = append(codes, "8")
	}
	if cell.Underline {
		codes = append(codes, "4")
	}
	if cell.Strikethrough {
		codes = append(codes, "9")
	}
	if fg := colorANSI(cell.Foreground, true); fg != "" {
		codes = append(codes, fg)
	}
	if bg := colorANSI(cell.Background, false); bg != "" {
		codes = append(codes, bg)
	}
	if len(codes) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(codes, ";") + "m"
}

func colorANSI(color protocol.EventPaneScreenColor, fg bool) string {
	switch color.Kind {
	case "palette":
		base := "38"
		if !fg {
			base = "48"
		}
		return base + ";5;" + strconv.Itoa(int(color.Index))
	case "rgb":
		base := "38"
		if !fg {
			base = "48"
		}
		return base + ";2;" + strconv.Itoa(int(color.R)) + ";" + strconv.Itoa(int(color.G)) + ";" + strconv.Itoa(int(color.B))
	default:
		return ""
	}
}
