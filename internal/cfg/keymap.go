package cfg

import "strings"

// BuiltinKeyAction names dispatchable multiplexer actions.
type BuiltinKeyAction string

const (
	ActionDetach          BuiltinKeyAction = "detach"
	ActionQuit            BuiltinKeyAction = "quit"
	ActionSplitLR         BuiltinKeyAction = "split_lr"
	ActionSplitTB         BuiltinKeyAction = "split_tb"
	ActionResizePaneLeft  BuiltinKeyAction = "resize_pane_left"
	ActionResizePaneDown  BuiltinKeyAction = "resize_pane_down"
	ActionResizePaneUp    BuiltinKeyAction = "resize_pane_up"
	ActionResizePaneRight BuiltinKeyAction = "resize_pane_right"
	ActionNextPane        BuiltinKeyAction = "next_pane"
	ActionClosePane       BuiltinKeyAction = "close_pane"
	ActionNewWindow       BuiltinKeyAction = "new_window"
	ActionNextWindow      BuiltinKeyAction = "next_window"
	ActionPreviousWindow  BuiltinKeyAction = "previous_window"
	ActionSelectWindow1   BuiltinKeyAction = "select_window_1"
	ActionSelectWindow2   BuiltinKeyAction = "select_window_2"
	ActionSelectWindow3   BuiltinKeyAction = "select_window_3"
	ActionSelectWindow4   BuiltinKeyAction = "select_window_4"
	ActionSelectWindow5   BuiltinKeyAction = "select_window_5"
	ActionSelectWindow6   BuiltinKeyAction = "select_window_6"
	ActionSelectWindow7   BuiltinKeyAction = "select_window_7"
	ActionSelectWindow8   BuiltinKeyAction = "select_window_8"
	ActionSelectWindow9   BuiltinKeyAction = "select_window_9"
	ActionSelectWindow10  BuiltinKeyAction = "select_window_10"
	ActionListKeymaps     BuiltinKeyAction = "list_keymaps"
)

// KeymapBinding is one prefix-mode binding after config load.
type KeymapBinding struct {
	Mode        string
	Key         string
	Builtin     BuiltinKeyAction
	LuaCallback int // gopher-lua registry ref; 0 = use Builtin
	Desc        string
}

// Keymaps holds frozen key bindings keyed by mode then normalized key.
type Keymaps struct {
	entries map[string]map[string]KeymapBinding
}

func NewKeymaps() *Keymaps {
	return &Keymaps{entries: make(map[string]map[string]KeymapBinding)}
}

func (k *Keymaps) Set(mode, key string, b KeymapBinding) {
	key = normalizeKeyKey(key)
	if k.entries[mode] == nil {
		k.entries[mode] = make(map[string]KeymapBinding)
	}
	b.Mode = mode
	b.Key = key
	k.entries[mode][key] = b
}

func (k *Keymaps) Lookup(mode, key string) (KeymapBinding, bool) {
	key = normalizeKeyKey(key)
	m, ok := k.entries[mode]
	if !ok {
		return KeymapBinding{}, false
	}
	b, ok := m[key]
	return b, ok
}

func (k *Keymaps) List(mode string) []KeymapBinding {
	m, ok := k.entries[mode]
	if !ok {
		return nil
	}
	out := make([]KeymapBinding, 0, len(m))
	for _, b := range m {
		out = append(out, b)
	}
	return out
}

func (k *Keymaps) Clone() *Keymaps {
	out := NewKeymaps()
	for mode, m := range k.entries {
		for key, b := range m {
			out.Set(mode, key, b)
		}
	}
	return out
}

func normalizeKeyKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.ToLower(key)
	return key
}

// DefaultKeymaps returns tmux-style prefix bindings matching README defaults.
func DefaultKeymaps() *Keymaps {
	k := NewKeymaps()
	set := func(key string, action BuiltinKeyAction, desc string) {
		k.Set("prefix", key, KeymapBinding{Builtin: action, Desc: desc})
	}
	set("d", ActionDetach, "Detach client")
	set("q", ActionQuit, "Quit shux when last client")
	set("%", ActionSplitLR, "Split pane left/right")
	set("\"", ActionSplitTB, "Split pane top/bottom")
	set("left", ActionResizePaneLeft, "Resize pane left")
	set("down", ActionResizePaneDown, "Resize pane down")
	set("up", ActionResizePaneUp, "Resize pane up")
	set("right", ActionResizePaneRight, "Resize pane right")
	set("h", ActionResizePaneLeft, "Resize pane left")
	set("j", ActionResizePaneDown, "Resize pane down")
	set("k", ActionResizePaneUp, "Resize pane up")
	set("l", ActionResizePaneRight, "Resize pane right")
	set("o", ActionNextPane, "Next pane")
	set("x", ActionClosePane, "Close pane")
	set("c", ActionNewWindow, "New window")
	set("n", ActionNextWindow, "Next window")
	set("p", ActionPreviousWindow, "Previous window")
	set("1", ActionSelectWindow1, "Select window 1")
	set("2", ActionSelectWindow2, "Select window 2")
	set("3", ActionSelectWindow3, "Select window 3")
	set("4", ActionSelectWindow4, "Select window 4")
	set("5", ActionSelectWindow5, "Select window 5")
	set("6", ActionSelectWindow6, "Select window 6")
	set("7", ActionSelectWindow7, "Select window 7")
	set("8", ActionSelectWindow8, "Select window 8")
	set("9", ActionSelectWindow9, "Select window 9")
	set("0", ActionSelectWindow10, "Select window 10")
	set("?", ActionListKeymaps, "List key bindings")
	return k
}

// ExpandLeaderKey returns the key suffix after <leader> in a lhs string.
func ExpandLeaderKey(lhs string) string {
	lhs = strings.TrimSpace(lhs)
	if strings.HasPrefix(lhs, "<leader>") {
		return strings.TrimPrefix(lhs, "<leader>")
	}
	if strings.HasPrefix(lhs, "<Leader>") {
		return strings.TrimPrefix(lhs, "<Leader>")
	}
	return lhs
}
