package gomux

// Resizable is the common interface for windows and terms (panes)
// that can be resized to specific dimensions
type Resizable interface {
	// Resize updates the dimensions to rows x cols
	Resize(rows, cols int)
}

// ResizeMsg is the common resize message for any Resizable actor
type ResizeMsg struct {
	Rows int
	Cols int
}
