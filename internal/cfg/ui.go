package cfg

const DefaultStatuslineStyle = "reverse"

// UIConfig holds client-side rendering options for the Bubble Tea UI.
type UIConfig struct {
	Statusline         bool
	PaneBorders        bool
	PaneLabels         bool
	StatuslineStyle    string
	SearchMatchANSI    string
	SearchActiveANSI   string
	CopyModeStatusANSI string
}

func DefaultUIConfig() UIConfig {
	return UIConfig{
		Statusline:      true,
		PaneBorders:     true,
		PaneLabels:      true,
		StatuslineStyle: DefaultStatuslineStyle,
	}
}

func (c UIConfig) WithDefaults() UIConfig {
	if c.StatuslineStyle == "" {
		c.StatuslineStyle = DefaultStatuslineStyle
	}
	return c
}
