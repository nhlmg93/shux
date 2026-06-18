package luabind

// StatuslineContext carries per-client UI state used by Lua statusline hooks.
type StatuslineContext struct {
	SessionID   string
	WindowID    string
	WindowIndex int
	WindowName  string
	ActivePane  string
	Hostname    string
	Title       string
	Status      string
}

// Runtime exposes the subset of Lua runtime behavior needed by shux.
type Runtime interface {
	CallKeymapRef(ref int)
	Statusline(ctx StatuslineContext) (left, right string)
	Close()
}
