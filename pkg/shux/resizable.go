package shux

// Resizable is the common interface for windows and terms (panes)
// that can be resized to specific dimensions
type Resizable interface {
	// Resize updates the dimensions to rows x cols
	Resize(rows, cols int)
}


