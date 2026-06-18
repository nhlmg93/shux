package persist

const (
	MinLayoutCols     = 10
	MinLayoutRows     = 5
	DefaultLayoutCols = 80
	DefaultLayoutRows = 24
)

// LayoutValidForCheckpoint reports whether a layout is large enough to persist.
// Tiny layouts come from non-interactive attaches or control-mode probes and
// must not overwrite a good resurrection checkpoint.
func LayoutValidForCheckpoint(layout LayoutSnapshot) bool {
	return layout.Cols >= MinLayoutCols && layout.Rows >= MinLayoutRows
}

// NormalizeLayoutForRestore returns a usable layout snapshot. Invalid
// checkpoints are upgraded to a single full-window pane at default size.
func NormalizeLayoutForRestore(layout LayoutSnapshot) LayoutSnapshot {
	if LayoutValidForCheckpoint(layout) {
		return layout
	}
	paneID := "p-1"
	if len(layout.Panes) > 0 && layout.Panes[0].PaneID != "" {
		paneID = layout.Panes[0].PaneID
	}
	return LayoutSnapshot{
		WindowID: layout.WindowID,
		Cols:     DefaultLayoutCols,
		Rows:     DefaultLayoutRows,
		Panes: []LayoutPaneSnapshot{{
			PaneID: paneID,
			Col:    0,
			Row:    0,
			Cols:   DefaultLayoutCols,
			Rows:   DefaultLayoutRows,
		}},
	}
}
