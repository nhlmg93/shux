package cfg

import "strings"

const DefaultStatuslineStyle = "reverse"

// Tmux pane-border-lines values (see tmux options-table.c).
const (
	PaneBorderLinesSingle = "single"
	PaneBorderLinesDouble = "double"
	PaneBorderLinesHeavy  = "heavy"
	PaneBorderLinesSimple = "simple"
	PaneBorderLinesNumber = "number"
	PaneBorderLinesSpaces = "spaces"
	PaneBorderLinesNone   = "none"
)

// UIConfig holds client-side rendering options for the Bubble Tea UI.
type UIConfig struct {
	Statusline            bool
	PaneBorders           bool // legacy; use pane_border_lines when possible
	PaneBorderLines       string
	PaneOuterBorder       bool   // false = internal split dividers only (no window box)
	PaneBorderStyle       string // ANSI/SGR prefix for inactive pane border cells
	PaneActiveBorderStyle string // ANSI/SGR prefix for active pane border cells
	PaneLabels            bool
	StatuslineStyle       string
	SearchMatchANSI       string
	SearchActiveANSI      string
	CopyModeStatusANSI    string
}

func DefaultUIConfig() UIConfig {
	return UIConfig{
		Statusline:      true,
		PaneBorders:     true,
		PaneBorderLines: PaneBorderLinesSingle,
		PaneOuterBorder: true,
		PaneLabels:      true,
		StatuslineStyle: DefaultStatuslineStyle,
	}
}

func (c UIConfig) WithDefaults() UIConfig {
	if c.StatuslineStyle == "" {
		c.StatuslineStyle = DefaultStatuslineStyle
	}
	if c.PaneBorderLines == "" {
		if c.PaneBorders {
			c.PaneBorderLines = PaneBorderLinesSingle
		} else {
			c.PaneBorderLines = PaneBorderLinesNone
		}
	}
	return c
}

// NormalizePaneBorderLines maps config input to a tmux pane-border-lines value.
func NormalizePaneBorderLines(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case PaneBorderLinesSingle, PaneBorderLinesDouble, PaneBorderLinesHeavy,
		PaneBorderLinesSimple, PaneBorderLinesNumber, PaneBorderLinesSpaces, PaneBorderLinesNone:
		return strings.ToLower(strings.TrimSpace(s))
	case "off":
		return PaneBorderLinesNone
	case "full":
		return PaneBorderLinesSingle
	default:
		return ""
	}
}

// EffectivePaneBorderLines returns the resolved tmux-style border mode.
func (c UIConfig) EffectivePaneBorderLines() string {
	if mode := NormalizePaneBorderLines(c.PaneBorderLines); mode != "" {
		return mode
	}
	if !c.PaneBorders {
		return PaneBorderLinesNone
	}
	return PaneBorderLinesSingle
}

// DrawsWindowBorders reports whether the full pane-box border pass runs before content.
func (c UIConfig) DrawsWindowBorders() bool {
	switch c.EffectivePaneBorderLines() {
	case PaneBorderLinesNone:
		return false
	default:
		return !c.SplitLinesOnly()
	}
}

// SplitLinesOnly draws internal dividers between panes without a box around the window.
func (c UIConfig) SplitLinesOnly() bool {
	switch c.EffectivePaneBorderLines() {
	case PaneBorderLinesNone, PaneBorderLinesSpaces:
		return false
	default:
		return !c.PaneOuterBorder
	}
}

// DrawsPaneBorders reports whether pane content should be inset for a full border box.
func (c UIConfig) DrawsPaneBorders() bool {
	if c.SplitLinesOnly() {
		return false
	}
	switch c.EffectivePaneBorderLines() {
	case PaneBorderLinesNone, PaneBorderLinesSpaces:
		return false
	default:
		return true
	}
}
