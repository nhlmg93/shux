package ui

import (
	"strings"
)

type runeCanvas struct {
	cols int
	rows int
	buf  [][]rune
}

func newRuneCanvas(cols, rows int) *runeCanvas {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	buf := make([][]rune, rows)
	for r := range buf {
		buf[r] = make([]rune, cols)
		for c := range buf[r] {
			buf[r][c] = ' '
		}
	}
	return &runeCanvas{cols: cols, rows: rows, buf: buf}
}

func (c *runeCanvas) drawPane(p LayoutPane, active bool) {
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
	if y < 0 || y >= c.rows || x >= c.cols {
		return
	}
	for _, r := range s {
		if x < 0 {
			x++
			continue
		}
		if x >= c.cols {
			return
		}
		c.set(x, y, r)
		x++
	}
}

func (c *runeCanvas) set(x, y int, r rune) {
	if x < 0 || x >= c.cols || y < 0 || y >= c.rows {
		return
	}
	c.buf[y][x] = r
}

func (c *runeCanvas) String() string {
	lines := make([]string, c.rows)
	for i, row := range c.buf {
		lines[i] = string(row)
	}
	return strings.Join(lines, "\n")
}
