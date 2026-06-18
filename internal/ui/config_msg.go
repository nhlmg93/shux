package ui

import "shux/internal/cfg"

// ConfigUpdatedMsg pushes daemon UI config to an attached client model.
type ConfigUpdatedMsg struct {
	UI cfg.UIConfig
}
