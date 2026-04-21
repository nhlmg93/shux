package shux

import "testing"

func TestStitchDividerBordersConnectsIntersections(t *testing.T) {
	borders := newBorderGrid(3, 3)
	borders[0][1].v = true
	borders[1][1].v = true
	borders[2][1].v = true
	borders[1][0].h = true
	borders[1][2].h = true

	stitchDividerBorders(borders)

	if !borders[1][1].h || !borders[1][1].v {
		t.Fatalf("center border = %+v, want both horizontal and vertical", borders[1][1])
	}
	if glyph := dividerGlyph(borders, 1, 1); glyph != "┼" {
		t.Fatalf("dividerGlyph(center) = %q, want %q", glyph, "┼")
	}
}

func TestMarkActivePaneBordersMarksExistingBorders(t *testing.T) {
	borders := newBorderGrid(5, 5)
	for c := 1; c <= 2; c++ {
		borders[0][c].h = true
		borders[3][c].h = true
	}
	for r := 1; r <= 2; r++ {
		borders[r][0].v = true
		borders[r][3].v = true
	}

	markActivePaneBorders(borders, []paneLayout{{paneID: 7, row: 1, col: 1, rows: 2, cols: 2}}, 7)

	activeCells := [][2]int{{0, 1}, {0, 2}, {3, 1}, {3, 2}, {1, 0}, {2, 0}, {1, 3}, {2, 3}}
	for _, cell := range activeCells {
		if !borders[cell[0]][cell[1]].active {
			t.Fatalf("border at (%d,%d) was not marked active", cell[0], cell[1])
		}
	}
	if borders[0][0].active {
		t.Fatal("expected non-border cell to remain inactive")
	}
}
