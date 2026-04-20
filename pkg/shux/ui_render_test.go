package shux

import (
	"strings"
	"testing"

	"github.com/mitchellh/go-libghostty"
)

func TestRenderRowTrueColorStyles(t *testing.T) {
	row := []PaneCell{
		{
			Text:       "R",
			Width:      1,
			HasFgColor: true,
			FgColor:    libghostty.ColorRGB{R: 255, G: 0, B: 0},
			HasBgColor: true,
			BgColor:    libghostty.ColorRGB{R: 1, G: 2, B: 3},
			Bold:       true,
		},
	}

	rendered := renderRow(row, 1)
	if !strings.Contains(rendered, "\x1b[1;38;2;255;0;0;48;2;1;2;3mR") {
		t.Fatalf("expected truecolor SGR in rendered row, got %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[0m") {
		t.Fatalf("expected final reset in rendered row, got %q", rendered)
	}
}

func TestStyleTransitionResetsClearedColors(t *testing.T) {
	from := cellStyle{
		HasFgColor: true,
		FgColor:    libghostty.ColorRGB{R: 10, G: 20, B: 30},
		HasBgColor: true,
		BgColor:    libghostty.ColorRGB{R: 40, G: 50, B: 60},
	}
	transition := styleTransition(from, defaultCellStyle())
	if transition != "\x1b[0m" {
		t.Fatalf("expected full reset to default style, got %q", transition)
	}

	transition = styleTransition(from, cellStyle{HasFgColor: true, FgColor: libghostty.ColorRGB{R: 10, G: 20, B: 30}})
	if !strings.Contains(transition, "49") {
		t.Fatalf("expected background reset in transition, got %q", transition)
	}
}
