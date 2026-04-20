package shux

import (
	"fmt"
	"strings"

	"github.com/mitchellh/go-libghostty"
)

func renderRow(cells []PaneCell, width int) string {
	var b strings.Builder
	col := 0
	currentStyle := defaultCellStyle()

	for i := 0; i < len(cells) && col < width; i++ {
		cell := cells[i]
		if cell.Width == 0 {
			continue
		}

		nextStyle := styleFromCell(cell)
		b.WriteString(styleTransition(currentStyle, nextStyle))
		currentStyle = nextStyle

		text := cell.Text
		if text == "" {
			text = " "
		}
		b.WriteString(text)

		if cell.Width > 0 {
			col += cell.Width
		} else {
			col++
		}
	}

	b.WriteString(styleTransition(currentStyle, defaultCellStyle()))
	for ; col < width; col++ {
		b.WriteByte(' ')
	}
	return b.String()
}

type cellStyle struct {
	HasFgColor bool
	FgColor    libghostty.ColorRGB
	HasBgColor bool
	BgColor    libghostty.ColorRGB
	Bold       bool
	Italic     bool
	Underline  bool
	Blink      bool
	Reverse    bool
}

func defaultCellStyle() cellStyle {
	return cellStyle{}
}

func styleFromCell(cell PaneCell) cellStyle {
	return cellStyle{
		HasFgColor: cell.HasFgColor,
		FgColor:    cell.FgColor,
		HasBgColor: cell.HasBgColor,
		BgColor:    cell.BgColor,
		Bold:       cell.Bold,
		Italic:     cell.Italic,
		Underline:  cell.Underline,
		Blink:      cell.Blink,
		Reverse:    cell.Reverse,
	}
}

func styleTransition(from, to cellStyle) string {
	if from == to {
		return ""
	}
	if to == defaultCellStyle() {
		return "\x1b[0m"
	}

	codes := make([]string, 0, 8)

	appendAttrTransition := func(was, now bool, on, off string) {
		if was == now {
			return
		}
		if now {
			codes = append(codes, on)
		} else {
			codes = append(codes, off)
		}
	}

	appendAttrTransition(from.Bold, to.Bold, "1", "22")
	appendAttrTransition(from.Italic, to.Italic, "3", "23")
	appendAttrTransition(from.Underline, to.Underline, "4", "24")
	appendAttrTransition(from.Blink, to.Blink, "5", "25")
	appendAttrTransition(from.Reverse, to.Reverse, "7", "27")

	switch {
	case from.HasFgColor && !to.HasFgColor:
		codes = append(codes, "39")
	case to.HasFgColor && (!from.HasFgColor || from.FgColor != to.FgColor):
		codes = append(codes, fmt.Sprintf("38;2;%d;%d;%d", to.FgColor.R, to.FgColor.G, to.FgColor.B))
	}

	switch {
	case from.HasBgColor && !to.HasBgColor:
		codes = append(codes, "49")
	case to.HasBgColor && (!from.HasBgColor || from.BgColor != to.BgColor):
		codes = append(codes, fmt.Sprintf("48;2;%d;%d;%d", to.BgColor.R, to.BgColor.G, to.BgColor.B))
	}

	if len(codes) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(codes, ";") + "m"
}
