package gomux

// Message types for actor communication

// Legacy pane messages (kept for compatibility, will be removed)
type CreatePane struct {
	Cmd  string
	Args []string
}
type KillPane struct{}
type PaneExited struct{ ID uint32 }
type SwitchToPane struct{ Index int }
type PaneOutput struct{ ID uint32; Data []byte }
type WriteToPane struct{ Data []byte }
type GetActivePane struct{}
type GetGrid struct{}

// Window messages
type CreateWindow struct{}
type WindowEmpty struct{ ID uint32 }
type SwitchWindow struct{ Delta int }
type GetActiveWindow struct{}

// TermActor messages (using Alacritty FFI)
type CreateTerm struct {
	Rows  int
	Cols  int
	Shell string
}
type KillTerm struct{}
type TermExited struct{ ID uint32 }
type SwitchToTerm struct{ Index int }
type WriteToTerm struct{ Data []byte }
type GetActiveTerm struct{}
type GetTermContent struct{}
type TermContent struct {
	Lines     []string
	CursorRow int
	CursorCol int
}

// GridUpdated is sent when a term's content changes
type GridUpdated struct {
	ID uint32
}

// ResizeGrid requests resize
type ResizeGrid struct {
	Width  int
	Height int
}
