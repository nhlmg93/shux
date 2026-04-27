package ui

import (
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"shux/internal/protocol"
)

var debugViewPath = os.Getenv("SHUX_DEBUG_VIEW")

const cursorANSI = "\x1b[7m" // reverse video: portable across terminal palettes.

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
	c.ansi[cy][cx] = c.ansi[cy][cx] + cursorANSI
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
	return string(p.PaneID) + " " + strconv.Itoa(p.Cols) + "x" + strconv.Itoa(p.Rows) + " @" + strconv.Itoa(p.Col) + "," + strconv.Itoa(p.Row)
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
	for drawn, cell := range line.Cells {
		if drawn >= maxWidth {
			return
		}
		text := cell.Text
		if text == "" {
			text = " "
		}
		r, _ := utf8.DecodeRuneInString(text)
		c.setStyled(x+drawn, y, r, cellANSI(cell))
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
	out := strings.Join(lines, "\n")
	if debugViewPath != "" {
		c.dumpDebug(out)
	}
	return out
}

func (c *runeCanvas) dumpDebug(rendered string) {
	f, err := os.OpenFile(debugViewPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString("=== row 0 cells ===\n")
	if c.rows > 0 {
		for x, r := range c.buf[0] {
			ansi := c.ansi[0][x]
			if ansi == "" && r == ' ' {
				continue
			}
			f.WriteString("  col ")
			f.WriteString(strconv.Itoa(x))
			f.WriteString(" rune=")
			f.WriteString(strconv.QuoteRune(r))
			f.WriteString(" ansi=")
			f.WriteString(strconv.Quote(ansi))
			f.WriteString("\n")
		}
	}
	f.WriteString("=== rendered ===\n")
	f.WriteString(strconv.Quote(rendered))
	f.WriteString("\n")
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
	base := "38"
	if !fg {
		base = "48"
	}
	switch color.Kind {
	case "palette":
		return base + ";5;" + strconv.Itoa(int(color.Index))
	case "rgb":
		return base + ";2;" + strconv.Itoa(int(color.R)) + ";" + strconv.Itoa(int(color.G)) + ";" + strconv.Itoa(int(color.B))
	default:
		return ""
	}
}
