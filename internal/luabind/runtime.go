package luabind

// Runtime exposes the subset of Lua runtime behavior needed by shux.
type Runtime interface {
	CallKeymapRef(ref int)
	Close()
}
