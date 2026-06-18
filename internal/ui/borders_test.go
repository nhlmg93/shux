package ui

import (
	"context"
	"testing"
	"time"

	"shux/internal/cfg"
)

func TestBorderCellTypeAt_leftFullRightSplitHorizontal(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	panes := []LayoutPane{
		{PaneID: "p-tl", Col: 0, Row: 0, Cols: 40, Rows: 12},
		{PaneID: "p-bl", Col: 0, Row: 12, Cols: 40, Rows: 12},
		{PaneID: "p-r", Col: 40, Row: 0, Cols: 40, Rows: 24},
	}
	// Right pane's left edge meets the horizontal split between left panes.
	if got := borderCellTypeAt(panes, 40, 12); got == borderCellLeftRight {
		t.Fatalf("split junction must not be plain vertical (got LEFTRIGHT)")
	}
	// Row above the left-stack split on the right pane edge must not be │.
	if got := borderCellTypeAt(panes, 40, 11); got == borderCellLeftRight {
		t.Fatalf("row above split should not be plain vertical (got LEFTRIGHT)")
	}
}

func TestBorderCellTypeAt_topFullBottomSplitVertical(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	panes := []LayoutPane{
		{PaneID: "p-top", Col: 0, Row: 0, Cols: 20, Rows: 10},
		{PaneID: "p-bl", Col: 0, Row: 10, Cols: 10, Rows: 10},
		{PaneID: "p-br", Col: 10, Row: 10, Cols: 10, Rows: 10},
	}
	if got := borderCellTypeAt(panes, 9, 10); got == borderCellLeftRight {
		t.Fatalf("vertical split under horizontal split must not be plain vertical (got LEFTRIGHT)")
	}
}

func TestDrawWindowBorders_horizontalEdgesUseHorizontalChars(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	canvas := newRuneCanvas(20, 12, cfg.UIConfig{PaneBorderLines: cfg.PaneBorderLinesSingle})
	panes := []LayoutPane{
		{PaneID: "p-top", Col: 0, Row: 0, Cols: 20, Rows: 6},
		{PaneID: "p-bot", Col: 0, Row: 6, Cols: 20, Rows: 6},
	}
	canvas.drawWindowBorders(panes, "p-top")
	for x := 1; x < 19; x++ {
		if got := canvas.buf[0][x]; got != '─' && got != '┌' && got != '┐' {
			t.Fatalf("top outer edge col %d = %q, want horizontal/corner", x, got)
		}
		if got := canvas.buf[6][x]; got == '│' {
			t.Fatalf("horizontal split row col %d must not be vertical bar", x)
		}
	}
}

func TestDrawInternalSplitLines_noOuterBorderSingleHorizontalSplit(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	canvas := newRuneCanvas(20, 12, cfg.UIConfig{
		PaneBorderLines: cfg.PaneBorderLinesSingle,
		PaneOuterBorder: false,
	})
	panes := []LayoutPane{
		{PaneID: "p-top", Col: 0, Row: 0, Cols: 20, Rows: 6},
		{PaneID: "p-bot", Col: 0, Row: 6, Cols: 20, Rows: 6},
	}
	canvas.drawInternalSplitLines(panes, "p-top")
	for x := 0; x < 20; x++ {
		if got := canvas.buf[0][x]; got == '─' || got == '┌' || got == '┐' || got == '│' {
			t.Fatalf("outer top row col %d should be blank, got %q", x, got)
		}
	}
	for x := 0; x < 20; x++ {
		if got := canvas.buf[6][x]; got == '│' {
			t.Fatalf("horizontal split must not use vertical caps; col %d = %q", x, got)
		}
		if got := canvas.buf[6][x]; got != '─' {
			t.Fatalf("internal split col %d = %q, want horizontal line", x, got)
		}
	}
	for x := 1; x < 19; x++ {
		if got := canvas.buf[5][x]; got == '─' {
			t.Fatalf("expected single split row, got horizontal on row 5 col %d", x)
		}
	}
}

func TestDrawInternalSplitLines_noVerticalStickAboveHorizontal(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	canvas := newRuneCanvas(20, 24, cfg.UIConfig{
		PaneBorderLines: cfg.PaneBorderLinesSingle,
		PaneOuterBorder: false,
	})
	// Top + bottom split, vertical only in bottom half.
	panes := []LayoutPane{
		{PaneID: "p-top", Col: 0, Row: 0, Cols: 20, Rows: 10},
		{PaneID: "p-bl", Col: 0, Row: 10, Cols: 10, Rows: 14},
		{PaneID: "p-br", Col: 10, Row: 10, Cols: 10, Rows: 14},
	}
	canvas.drawInternalSplitLines(panes, "p-top")
	if got := canvas.buf[9][10]; got == '│' {
		t.Fatalf("vertical must not protrude above horizontal split; row 9 col 10 = %q", got)
	}
	if got := canvas.buf[10][10]; got != '┬' {
		t.Fatalf("split junction row 10 col 10 = %q, want top join (horizontal + vertical below)", got)
	}
	for _, x := range []int{0, 9} {
		if got := canvas.buf[10][x]; got != '─' {
			t.Fatalf("horizontal split endpoint row 10 col %d = %q, want horizontal", x, got)
		}
	}

	// Full-height left pane + right stack split horizontally.
	canvas = newRuneCanvas(20, 24, cfg.UIConfig{
		PaneBorderLines: cfg.PaneBorderLinesSingle,
		PaneOuterBorder: false,
	})
	panes = []LayoutPane{
		{PaneID: "p-left", Col: 0, Row: 0, Cols: 10, Rows: 24},
		{PaneID: "p-tr", Col: 10, Row: 0, Cols: 10, Rows: 12},
		{PaneID: "p-br", Col: 10, Row: 12, Cols: 10, Rows: 12},
	}
	canvas.drawInternalSplitLines(panes, "p-top")
	if got := canvas.buf[11][10]; got != '│' {
		t.Fatalf("vertical divider between left and top-right row 11 col 10 = %q, want vertical", got)
	}
	if got := canvas.buf[12][10]; got == '│' {
		t.Fatalf("horizontal split junction row 12 col 10 must not be plain vertical, got %q", got)
	}
	if got := canvas.buf[12][10]; got != '├' {
		t.Fatalf("horizontal split junction row 12 col 10 = %q, want left join", got)
	}
}

func TestDrawInternalSplitLines_fourPaneGridCenterCross(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	canvas := newRuneCanvas(20, 10, cfg.UIConfig{
		PaneBorderLines: cfg.PaneBorderLinesSingle,
		PaneOuterBorder: false,
	})
	panes := []LayoutPane{
		{PaneID: "p-tl", Col: 0, Row: 0, Cols: 10, Rows: 5},
		{PaneID: "p-tr", Col: 10, Row: 0, Cols: 10, Rows: 5},
		{PaneID: "p-bl", Col: 0, Row: 5, Cols: 10, Rows: 5},
		{PaneID: "p-br", Col: 10, Row: 5, Cols: 10, Rows: 5},
	}
	canvas.drawInternalSplitLines(panes, "p-tl")

	if got := canvas.buf[5][10]; got != '┼' {
		t.Fatalf("center junction row 5 col 10 = %q, want cross", got)
	}
	if got := canvas.buf[5][9]; got != '─' {
		t.Fatalf("horizontal left of center row 5 col 9 = %q, want horizontal", got)
	}
	if got := canvas.buf[4][10]; got != '│' {
		t.Fatalf("vertical above center row 4 col 10 = %q, want vertical", got)
	}
	if got := canvas.buf[5][10]; got == '│' {
		t.Fatalf("center must not be plain vertical")
	}
}


func TestDrawWindowBorders_activeBorderStyle(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	activeStyle := "\x1b[1;34m"
	inactiveStyle := "\x1b[90m"
	canvas := newRuneCanvas(20, 12, cfg.UIConfig{
		PaneBorderLines:       cfg.PaneBorderLinesSingle,
		PaneActiveBorderStyle: activeStyle,
		PaneBorderStyle:       inactiveStyle,
	})
	panes := []LayoutPane{
		{PaneID: "p-top", Col: 0, Row: 0, Cols: 20, Rows: 6},
		{PaneID: "p-bot", Col: 0, Row: 6, Cols: 20, Rows: 6},
	}
	canvas.drawWindowBorders(panes, "p-top")
	if got := canvas.ansi[0][0]; got != activeStyle {
		t.Fatalf("active pane top-left style = %q, want %q", got, activeStyle)
	}
	if got := canvas.ansi[11][0]; got != inactiveStyle {
		t.Fatalf("inactive pane bottom-left style = %q, want %q", got, inactiveStyle)
	}
	if got := canvas.ansi[5][10]; got != activeStyle {
		t.Fatalf("active pane bottom edge style = %q, want %q", got, activeStyle)
	}
	if got := canvas.ansi[6][10]; got != inactiveStyle {
		t.Fatalf("inactive pane top edge style = %q, want %q", got, inactiveStyle)
	}
}

func TestDrawInternalSplitLines_activeBorderStyle(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	activeStyle := "\x1b[1;34m"
	inactiveStyle := "\x1b[90m"
	canvas := newRuneCanvas(20, 12, cfg.UIConfig{
		PaneBorderLines:       cfg.PaneBorderLinesSingle,
		PaneOuterBorder:       false,
		PaneActiveBorderStyle: activeStyle,
		PaneBorderStyle:       inactiveStyle,
	})
	panes := []LayoutPane{
		{PaneID: "p-top", Col: 0, Row: 0, Cols: 20, Rows: 6},
		{PaneID: "p-bot", Col: 0, Row: 6, Cols: 20, Rows: 6},
	}
	canvas.drawInternalSplitLines(panes, "p-bot")
	if got := canvas.ansi[6][10]; got != activeStyle {
		t.Fatalf("active split line style = %q, want %q", got, activeStyle)
	}
	canvas.drawInternalSplitLines(panes, "p-top")
	if got := canvas.ansi[6][10]; got != inactiveStyle {
		t.Fatalf("inactive split line style = %q, want %q", got, inactiveStyle)
	}
}

func TestDrawWindowBorders_noVerticalOverdrawAtSplit(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	canvas := newRuneCanvas(80, 24, cfg.UIConfig{PaneBorderLines: cfg.PaneBorderLinesSingle})
	panes := []LayoutPane{
		{PaneID: "p-tl", Col: 0, Row: 0, Cols: 40, Rows: 12},
		{PaneID: "p-bl", Col: 0, Row: 12, Cols: 40, Rows: 12},
		{PaneID: "p-r", Col: 40, Row: 0, Cols: 40, Rows: 24},
	}
	canvas.drawWindowBorders(panes, "p-tl")
	for _, row := range []int{11, 12} {
		if got := canvas.buf[row][40]; got == '│' {
			t.Fatalf("vertical bar overdraw at horizontal split junction row %d", row)
		}
	}
}
