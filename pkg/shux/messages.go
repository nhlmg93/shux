package shux

// Message types for actor communication

// Session messages
type CreateWindow struct {
	Rows int
	Cols int
}
type WindowEmpty struct{ ID uint32 }
type SwitchWindow struct{ Delta int }
type GetActiveWindow struct{}
type SessionEmpty struct{ ID uint32 }

// Pane messages (terminal panes within windows)
type CreatePane struct {
	Rows  int
	Cols  int
	Shell string
}
type KillPane struct{}
type PaneExited struct{ ID uint32 }
type SwitchToPane struct{ Index int }
type WriteToPane struct{ Data []byte }
type GetActivePane struct{}
type GetPaneContent struct{}
type GetPaneMode struct{}

// PaneMode contains state information about a pane
type PaneMode struct {
	InAltScreen  bool
	CursorHidden bool
}

// PaneContentUpdated is sent when a pane's content changes
type PaneContentUpdated struct {
	ID uint32
}

// ResizeTerm is the specific resize message for Pane actors
// (uses rows/cols like ResizeMsg but kept for explicit pane handling)
type ResizeTerm struct {
	Rows int
	Cols int
}

// ResizeMsg is the common resize message for any Resizable actor
type ResizeMsg struct {
	Rows int
	Cols int
}
