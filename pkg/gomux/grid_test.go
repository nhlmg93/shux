package gomux

import "testing"

func TestGridResize(t *testing.T) {
	g := NewGrid(10, 5)

	if g.Width != 10 || g.Height != 5 {
		t.Errorf("Expected 10x5, got %dx%d", g.Width, g.Height)
	}
	if len(g.Cells) != 5 {
		t.Errorf("Expected 5 rows, got %d", len(g.Cells))
	}
	if len(g.Cells[0]) != 10 {
		t.Errorf("Expected 10 cols, got %d", len(g.Cells[0]))
	}

	g.Resize(20, 10)

	if g.Width != 20 || g.Height != 10 {
		t.Errorf("Expected 20x10 after resize, got %dx%d", g.Width, g.Height)
	}
}

func TestGridWriteChar(t *testing.T) {
	g := NewGrid(5, 3)

	g.WriteChar('h')
	g.WriteChar('i')

	if g.Cells[0][0].Char != 'h' {
		t.Errorf("Expected 'h' at [0][0], got '%c'", g.Cells[0][0].Char)
	}
	if g.Cells[0][1].Char != 'i' {
		t.Errorf("Expected 'i' at [0][1], got '%c'", g.Cells[0][1].Char)
	}
	if g.CursorX != 2 {
		t.Errorf("Expected cursor at col 2, got %d", g.CursorX)
	}
}

func TestGridNewLine(t *testing.T) {
	g := NewGrid(5, 3)

	g.WriteChar('a')
	g.NewLine()
	g.WriteChar('b')

	if g.CursorY != 1 || g.CursorX != 1 {
		t.Errorf("Expected cursor at (1,1), got (%d,%d)", g.CursorY, g.CursorX)
	}

	row0 := g.GetRow(0)
	if row0 != "a    " {
		t.Errorf("Expected row0 'a    ', got '%s'", row0)
	}
}

func TestGridClear(t *testing.T) {
	g := NewGrid(5, 3)

	g.WriteChar('x')
	g.Clear()

	if g.Cells[0][0].Char != 0 {
		t.Errorf("Expected cell cleared, got '%c'", g.Cells[0][0].Char)
	}
	if g.CursorX != 0 || g.CursorY != 0 {
		t.Errorf("Expected cursor reset to (0,0), got (%d,%d)", g.CursorX, g.CursorY)
	}
}
