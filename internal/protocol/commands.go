package protocol

type Command any

type CommandNoop struct{}

// CommandCreateSession is handled by the supervisor; it spawns a new session actor.
type CommandCreateSession struct{}

// CommandCreateWindow is handled by the session actor; it creates a new window.
// When sent via the supervisor, SessionID selects the session; zero value is invalid.
type CommandCreateWindow struct {
	SessionID SessionID
}

// CommandCreatePane is handled by the window actor; it creates a new pane.
// When sent via the supervisor, SessionID selects the session; the session then
// routes to WindowID. Zero SessionID/WindowID are invalid for routed sends.
type CommandCreatePane struct {
	SessionID SessionID
	WindowID  WindowID
}

// CommandPaneInit creates the pane's libghostty terminal at the given cell size.
// Cols and Rows must be non-zero; as uint16 they cannot be negative. The pane
// actor also panics on double init or if NewTerminal fails (see pane package).
type CommandPaneInit struct {
	Cols uint16
	Rows uint16
}
