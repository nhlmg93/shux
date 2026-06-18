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
	ActionFocusPaneLeft   BuiltinKeyAction = "focus_pane_left"
	ActionFocusPaneRight  BuiltinKeyAction = "focus_pane_right"
	ActionFocusPaneUp     BuiltinKeyAction = "focus_pane_up"
	ActionFocusPaneDown   BuiltinKeyAction = "focus_pane_down"
	ActionDisplayPanes    BuiltinKeyAction = "display_panes"
	ActionClosePane       BuiltinKeyAction = "close_pane"
	ActionTogglePaneZoom  BuiltinKeyAction = "toggle_pane_zoom"
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
	ActionRenameWindow    BuiltinKeyAction = "rename_window"
	ActionRenamePane      BuiltinKeyAction = "rename_pane"
	ActionCopyModeToggle  BuiltinKeyAction = "copy_mode_toggle"
	ActionPasteRegister   BuiltinKeyAction = "paste_register"

	ActionCopyLeft          BuiltinKeyAction = "copy_left"
	ActionCopyDown          BuiltinKeyAction = "copy_down"
	ActionCopyUp            BuiltinKeyAction = "copy_up"
	ActionCopyRight         BuiltinKeyAction = "copy_right"
	ActionCopyWordForward   BuiltinKeyAction = "copy_word_forward"
	ActionCopyWordBackward  BuiltinKeyAction = "copy_word_backward"
	ActionCopyTop           BuiltinKeyAction = "copy_top"
	ActionCopyBottom        BuiltinKeyAction = "copy_bottom"
	ActionCopyPageUp        BuiltinKeyAction = "copy_page_up"
	ActionCopyPageDown      BuiltinKeyAction = "copy_page_down"
	ActionCopySelectStart   BuiltinKeyAction = "copy_select_start"
	ActionCopyYankSelection BuiltinKeyAction = "copy_yank_selection"
	ActionCopyCancel        BuiltinKeyAction = "copy_cancel"
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
	setPrefix := func(key string, action BuiltinKeyAction, desc string) {
		k.Set("prefix", key, KeymapBinding{Builtin: action, Desc: desc})
	}
	setPrefix("d", ActionDetach, "Detach client")
	setPrefix("q", ActionDisplayPanes, "Display pane numbers for quick select")
	setPrefix("!", ActionQuit, "Quit shux when last client")
	setPrefix("%", ActionSplitLR, "Split pane left/right")
	setPrefix("\"", ActionSplitTB, "Split pane top/bottom")
	setPrefix("H", ActionResizePaneLeft, "Resize pane left")
	setPrefix("J", ActionResizePaneDown, "Resize pane down")
	setPrefix("K", ActionResizePaneUp, "Resize pane up")
	setPrefix("L", ActionResizePaneRight, "Resize pane right")
	setPrefix("o", ActionNextPane, "Next pane")
	setPrefix("h", ActionFocusPaneLeft, "Focus pane left")
	setPrefix("j", ActionFocusPaneDown, "Focus pane down")
	setPrefix("k", ActionFocusPaneUp, "Focus pane up")
	setPrefix("l", ActionFocusPaneRight, "Focus pane right")
	setPrefix("left", ActionFocusPaneLeft, "Focus pane left")
	setPrefix("down", ActionFocusPaneDown, "Focus pane down")
	setPrefix("up", ActionFocusPaneUp, "Focus pane up")
	setPrefix("right", ActionFocusPaneRight, "Focus pane right")
	setPrefix("x", ActionClosePane, "Close pane")
	setPrefix("z", ActionTogglePaneZoom, "Toggle pane zoom")
	setPrefix("c", ActionNewWindow, "New window")
	setPrefix("n", ActionNextWindow, "Next window")
	setPrefix("p", ActionPreviousWindow, "Previous window")
	setPrefix("1", ActionSelectWindow1, "Select window 1")
	setPrefix("2", ActionSelectWindow2, "Select window 2")
	setPrefix("3", ActionSelectWindow3, "Select window 3")
	setPrefix("4", ActionSelectWindow4, "Select window 4")
	setPrefix("5", ActionSelectWindow5, "Select window 5")
	setPrefix("6", ActionSelectWindow6, "Select window 6")
	setPrefix("7", ActionSelectWindow7, "Select window 7")
	setPrefix("8", ActionSelectWindow8, "Select window 8")
	setPrefix("9", ActionSelectWindow9, "Select window 9")
	setPrefix("0", ActionSelectWindow10, "Select window 10")
	setPrefix(",", ActionRenameWindow, "Rename active window")
	setPrefix(".", ActionRenamePane, "Rename active pane")
	setPrefix("?", ActionListKeymaps, "List key bindings")
	setPrefix("[", ActionCopyModeToggle, "Enter/exit copy mode")
	setPrefix("]", ActionPasteRegister, "Paste copy register")

	setCopy := func(key string, action BuiltinKeyAction, desc string) {
		k.Set("copy_mode", key, KeymapBinding{Builtin: action, Desc: desc})
	}
	setCopy("h", ActionCopyLeft, "Move left")
	setCopy("j", ActionCopyDown, "Move down")
	setCopy("k", ActionCopyUp, "Move up")
	setCopy("l", ActionCopyRight, "Move right")
	setCopy("w", ActionCopyWordForward, "Next word")
	setCopy("b", ActionCopyWordBackward, "Previous word")
	setCopy("g", ActionCopyTop, "Top of scrollback")
	setCopy("shift+g", ActionCopyBottom, "Bottom of scrollback")
	setCopy("pageup", ActionCopyPageUp, "Scroll page up")
	setCopy("pagedown", ActionCopyPageDown, "Scroll page down")
	setCopy("space", ActionCopySelectStart, "Start selection")
	setCopy("v", ActionCopySelectStart, "Start selection")
	setCopy("y", ActionCopyYankSelection, "Yank selection")
	setCopy("enter", ActionCopyYankSelection, "Yank selection and exit")
	setCopy("escape", ActionCopyCancel, "Exit copy mode")
	setCopy("q", ActionCopyCancel, "Exit copy mode")
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
