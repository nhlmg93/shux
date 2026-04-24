package protocol

type Command any

type CommandNoop struct{}

// CommandPaneInit creates the pane's libghostty terminal at the given cell size.
// Cols and Rows must be non-zero; as uint16 they cannot be negative. The pane
// actor also panics on double init or if NewTerminal fails (see pane package).
type CommandPaneInit struct {
	Cols uint16
	Rows uint16
}
