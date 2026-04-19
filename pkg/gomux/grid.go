package gomux

// Cell represents a single character cell in the terminal grid
type Cell struct {
	Char rune
}

// Grid is a 2D buffer of cells representing a terminal screen
type Grid struct {
	Cells         [][]Cell
	Width, Height int
	CursorX       int
	CursorY       int
}

// NewGrid creates a new grid with the given dimensions
func NewGrid(width, height int) *Grid {
	g := &Grid{Width: width, Height: height}
	g.Resize(width, height)
	return g
}

// Resize changes the grid dimensions
func (g *Grid) Resize(width, height int) {
	g.Width = width
	g.Height = height
	g.Cells = make([][]Cell, height)
	for i := range g.Cells {
		g.Cells[i] = make([]Cell, width)
	}
}

// WriteChar writes a character at the current cursor position
func (g *Grid) WriteChar(ch rune) {
	if g.CursorY >= 0 && g.CursorY < g.Height && g.CursorX >= 0 && g.CursorX < g.Width {
		g.Cells[g.CursorY][g.CursorX].Char = ch
		g.CursorX++
	}
}

// NewLine moves cursor to beginning of next line
func (g *Grid) NewLine() {
	g.CursorX = 0
	g.CursorY++
	if g.CursorY >= g.Height {
		g.CursorY = g.Height - 1
	}
}

// GetRow returns the characters in a row as a string
func (g *Grid) GetRow(y int) string {
	if y < 0 || y >= g.Height {
		return ""
	}
	chars := make([]rune, g.Width)
	for i, cell := range g.Cells[y] {
		if cell.Char == 0 {
			chars[i] = ' '
		} else {
			chars[i] = cell.Char
		}
	}
	return string(chars)
}

// Clear empties the entire grid
func (g *Grid) Clear() {
	for i := range g.Cells {
		for j := range g.Cells[i] {
			g.Cells[i][j].Char = 0
		}
	}
	g.CursorX = 0
	g.CursorY = 0
}
