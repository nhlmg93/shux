package gomux

// Message types for actor communication

type CreatePane struct {
	Cmd  string
	Args []string
}

type KillPane struct{}

type PaneExited struct {
	ID uint32
}

type SwitchToPane struct {
	Index int
}

type PaneOutput struct {
	ID   uint32
	Data []byte
}

type WriteToPane struct {
	Data []byte
}

type CreateWindow struct{}

type WindowEmpty struct {
	ID uint32
}

type SwitchWindow struct {
	Delta int
}

type GetActivePane struct{}

type GetActiveWindow struct{}

// GridUpdated is sent when a pane's grid content changes
type GridUpdated struct {
	ID uint32
}

// GetGrid requests the pane's grid content
type GetGrid struct{}

// ResizeGrid requests the pane's grid be resized
type ResizeGrid struct {
	Width  int
	Height int
}
