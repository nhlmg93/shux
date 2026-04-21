//go:build ci
// +build ci

package shux

import (
	"testing"

	"github.com/mitchellh/go-libghostty"
)

func FuzzRenderRow(f *testing.F) {
	f.Add([]byte("hello"), uint8(5))
	f.Add([]byte{0x1b, '[', '3', '1', 'm'}, uint8(10))
	f.Add([]byte{}, uint8(0))

	f.Fuzz(func(t *testing.T, data []byte, width uint8) {
		cells := make([]PaneCell, 0, len(data))
		for i, b := range data {
			if i >= 64 {
				break
			}
			cells = append(cells, PaneCell{
				Text:       string([]byte{b}),
				Width:      int(b % 3),
				HasFgColor: b%2 == 0,
				FgColor:    libghostty.ColorRGB{R: b, G: uint8(i), B: uint8(len(data))},
				HasBgColor: b%5 == 0,
				BgColor:    libghostty.ColorRGB{R: uint8(len(data)), G: b, B: uint8(i)},
				Bold:       b&1 != 0,
				Italic:     b&2 != 0,
				Underline:  b&4 != 0,
				Blink:      b&8 != 0,
				Reverse:    b&16 != 0,
			})
		}

		_ = renderRow(cells, int(width))
		for i := 0; i+1 < len(cells); i++ {
			_ = styleTransition(styleFromCell(cells[i]), styleFromCell(cells[i+1]))
		}
	})
}
