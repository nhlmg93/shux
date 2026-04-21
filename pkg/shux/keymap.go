package shux

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Action names supported by the configurable top-level keymap.
type Action string

const (
	ActionQuit               Action = "quit"
	ActionNewWindow          Action = "new_window"
	ActionNextWindow         Action = "next_window"
	ActionPrevWindow         Action = "prev_window"
	ActionSplitHorizontal    Action = "split_horizontal"
	ActionSplitVertical      Action = "split_vertical"
	ActionSelectPaneLeft     Action = "select_pane_left"
	ActionSelectPaneDown     Action = "select_pane_down"
	ActionSelectPaneUp       Action = "select_pane_up"
	ActionSelectPaneRight    Action = "select_pane_right"
	ActionResizePaneLeft     Action = "resize_pane_left"
	ActionResizePaneDown     Action = "resize_pane_down"
	ActionResizePaneUp       Action = "resize_pane_up"
	ActionResizePaneRight    Action = "resize_pane_right"
	ActionDetach             Action = "detach"
	ActionSendPrefix         Action = "send_prefix"
	ActionSelectWindow0      Action = "select_window_0"
	ActionSelectWindow1      Action = "select_window_1"
	ActionSelectWindow2      Action = "select_window_2"
	ActionSelectWindow3      Action = "select_window_3"
	ActionSelectWindow4      Action = "select_window_4"
	ActionSelectWindow5      Action = "select_window_5"
	ActionSelectWindow6      Action = "select_window_6"
	ActionSelectWindow7      Action = "select_window_7"
	ActionSelectWindow8      Action = "select_window_8"
	ActionSelectWindow9      Action = "select_window_9"
	ActionKillPane           Action = "kill_pane"
	ActionKillWindow         Action = "kill_window"
	ActionRenameWindow       Action = "rename_window"
	ActionRenameSession      Action = "rename_session"
	ActionLastWindow         Action = "last_window"
	ActionZoomPane           Action = "zoom_pane"
	ActionSwapPaneUp         Action = "swap_pane_up"
	ActionSwapPaneDown       Action = "swap_pane_down"
	ActionCommandPrompt      Action = "command_prompt"
	ActionChooseTreeSessions Action = "choose_tree_sessions"
	ActionChooseTreeWindows  Action = "choose_tree_windows"
	ActionShowHelp           Action = "show_help"
	ActionListSessions       Action = "list_sessions"
	ActionAttachSession      Action = "attach_session"
	ActionKillSession        Action = "kill_session"
)

var validActions = map[Action]struct{}{
	ActionQuit:               {},
	ActionNewWindow:          {},
	ActionNextWindow:         {},
	ActionPrevWindow:         {},
	ActionSplitHorizontal:    {},
	ActionSplitVertical:      {},
	ActionSelectPaneLeft:     {},
	ActionSelectPaneDown:     {},
	ActionSelectPaneUp:       {},
	ActionSelectPaneRight:    {},
	ActionResizePaneLeft:     {},
	ActionResizePaneDown:     {},
	ActionResizePaneUp:       {},
	ActionResizePaneRight:    {},
	ActionDetach:             {},
	ActionSendPrefix:         {},
	ActionSelectWindow0:      {},
	ActionSelectWindow1:      {},
	ActionSelectWindow2:      {},
	ActionSelectWindow3:      {},
	ActionSelectWindow4:      {},
	ActionSelectWindow5:      {},
	ActionSelectWindow6:      {},
	ActionSelectWindow7:      {},
	ActionSelectWindow8:      {},
	ActionSelectWindow9:      {},
	ActionKillPane:           {},
	ActionKillWindow:         {},
	ActionRenameWindow:       {},
	ActionRenameSession:      {},
	ActionLastWindow:         {},
	ActionZoomPane:           {},
	ActionSwapPaneUp:         {},
	ActionSwapPaneDown:       {},
	ActionCommandPrompt:      {},
	ActionChooseTreeSessions: {},
	ActionChooseTreeWindows:  {},
	ActionShowHelp:           {},
	ActionListSessions:       {},
	ActionAttachSession:      {},
	ActionKillSession:        {},
}

// Binding describes a resolved key binding.
type Binding struct {
	Action Action
	Amount int
}

func (b Binding) normalized() Binding {
	if b.Amount <= 0 {
		switch b.Action {
		case ActionResizePaneLeft, ActionResizePaneDown, ActionResizePaneUp, ActionResizePaneRight:
			b.Amount = 1
		}
	}
	return b
}

// KeymapConfig describes user overrides applied on top of tmux-style defaults.
type KeymapConfig struct {
	Prefix string
	Bind   map[string]string
	Unbind []string
}

func (c *KeymapConfig) SetBinding(spec, action string) {
	if c == nil {
		return
	}
	if c.Bind == nil {
		c.Bind = make(map[string]string)
	}
	c.Bind[spec] = action
}

func (c *KeymapConfig) AddUnbind(spec string) {
	if c == nil {
		return
	}
	c.Unbind = append(c.Unbind, spec)
}

// Keymap is the resolved UI keymap used by the model.
type Keymap struct {
	prefix      string
	prefixInput KeyInput
	bindings    map[string]Binding
}

func DefaultKeymap() Keymap {
	keymap, err := NewKeymap(KeymapConfig{})
	if err != nil {
		panic(err)
	}
	return keymap
}

func NewKeymap(cfg KeymapConfig) (Keymap, error) {
	prefixSpec := strings.TrimSpace(cfg.Prefix)
	if prefixSpec == "" {
		prefixSpec = "C-b"
	}

	prefix, prefixInput, err := parseKeySpec(prefixSpec)
	if err != nil {
		return Keymap{}, fmt.Errorf("prefix: %w", err)
	}

	bindings := map[string]Binding{}
	for spec, binding := range tmuxDefaultBindingSpecs() {
		key, _, err := parseKeySpec(spec)
		if err != nil {
			return Keymap{}, fmt.Errorf("default binding %q: %w", spec, err)
		}
		bindings[key] = binding.normalized()
	}

	for _, spec := range cfg.Unbind {
		key, _, err := parseKeySpec(spec)
		if err != nil {
			return Keymap{}, fmt.Errorf("unbind %q: %w", spec, err)
		}
		delete(bindings, key)
	}

	for spec, actionName := range cfg.Bind {
		binding, err := resolveBinding(actionName)
		if err != nil {
			return Keymap{}, fmt.Errorf("bind %q: %w", spec, err)
		}
		key, _, err := parseKeySpec(spec)
		if err != nil {
			return Keymap{}, fmt.Errorf("bind %q: %w", spec, err)
		}
		bindings[key] = binding.normalized()
	}

	if binding, exists := bindings[prefix]; exists {
		// Allow binding prefix key to send-prefix action (tmux behavior: press prefix twice)
		if binding.Action != ActionSendPrefix {
			return Keymap{}, fmt.Errorf("binding conflict: prefix %q is also bound as an action", prefixSpec)
		}
	}

	return Keymap{
		prefix:      prefix,
		prefixInput: prefixInput,
		bindings:    bindings,
	}, nil
}

func (k Keymap) Prefix() string {
	return k.prefix
}

func (k Keymap) ActionFor(key string) (Action, bool) {
	binding, ok := k.bindings[key]
	if !ok {
		return "", false
	}
	return binding.Action, true
}

func (k Keymap) BindingFor(key string) (Binding, bool) {
	binding, ok := k.bindings[key]
	return binding, ok
}

func (k Keymap) PrefixInput() KeyInput {
	return k.prefixInput
}

func (k Keymap) Bindings() map[string]Action {
	result := make(map[string]Action, len(k.bindings))
	for key, binding := range k.bindings {
		result[key] = binding.Action
	}
	return result
}

func ValidActions() []Action {
	actions := make([]Action, 0, len(validActions))
	for action := range validActions {
		actions = append(actions, action)
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i] < actions[j] })
	return actions
}

func tmuxDefaultBindingSpecs() map[string]Binding {
	return map[string]Binding{
		"C-b":     {Action: ActionSendPrefix},
		"c":       {Action: ActionNewWindow},
		"n":       {Action: ActionNextWindow},
		"p":       {Action: ActionPrevWindow},
		`"`:       {Action: ActionSplitHorizontal},
		"%":       {Action: ActionSplitVertical},
		"Left":    {Action: ActionSelectPaneLeft},
		"Down":    {Action: ActionSelectPaneDown},
		"Up":      {Action: ActionSelectPaneUp},
		"Right":   {Action: ActionSelectPaneRight},
		"C-Left":  {Action: ActionResizePaneLeft, Amount: 1},
		"C-Down":  {Action: ActionResizePaneDown, Amount: 1},
		"C-Up":    {Action: ActionResizePaneUp, Amount: 1},
		"C-Right": {Action: ActionResizePaneRight, Amount: 1},
		"M-Left":  {Action: ActionResizePaneLeft, Amount: 5},
		"M-Down":  {Action: ActionResizePaneDown, Amount: 5},
		"M-Up":    {Action: ActionResizePaneUp, Amount: 5},
		"M-Right": {Action: ActionResizePaneRight, Amount: 5},
		"d":       {Action: ActionDetach},
	}
}

func parseAction(name string) (Action, error) {
	action := Action(strings.TrimSpace(name))
	if _, ok := validActions[action]; !ok {
		return "", fmt.Errorf("unknown action %q", name)
	}
	return action, nil
}

func resolveBinding(name string) (Binding, error) {
	if action, err := parseAction(name); err == nil {
		return Binding{Action: action}.normalized(), nil
	}
	return parseTmuxCommand(name)
}

// parseTmuxCommand parses a tmux-style command string into a Binding.
// It reuses the Command parsing infrastructure from command.go to ensure
// consistency and avoid duplication. The command string is first parsed
// using ParseCommand, then converted to an ActionMsg via ToActionMsg.
// For commands that support numeric amounts (like resize-pane), the Amount
// field is extracted from the ActionMsg and mapped to the Binding.
func parseTmuxCommand(command string) (Binding, error) {
	cmd, err := ParseCommand(command)
	if err != nil {
		return Binding{}, err
	}

	msg, err := cmd.toActionMsg()
	if err != nil {
		return Binding{}, err
	}

	return Binding{Action: msg.Action, Amount: msg.Amount}.normalized(), nil
}

func parseKeySpec(spec string) (string, KeyInput, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", KeyInput{}, fmt.Errorf("empty key spec")
	}

	if spec == "-" {
		return "-", KeyInput{Code: '-'}, nil
	}

	tokens := strings.FieldsFunc(spec, func(r rune) bool {
		return r == '-' || r == '+'
	})
	if len(tokens) == 0 {
		return "", KeyInput{}, fmt.Errorf("invalid key spec %q", spec)
	}

	var mods KeyMods
	for _, token := range tokens[:len(tokens)-1] {
		switch strings.ToLower(strings.TrimSpace(token)) {
		case "c", "ctrl", "control":
			mods |= KeyModCtrl
		case "m", "alt", "meta":
			mods |= KeyModAlt
		case "s", "shift":
			mods |= KeyModShift
		case "super":
			mods |= KeyModSuper
		default:
			return "", KeyInput{}, fmt.Errorf("unknown modifier %q", token)
		}
	}

	keyToken := strings.TrimSpace(tokens[len(tokens)-1])
	if keyToken == "" {
		return "", KeyInput{}, fmt.Errorf("missing key name")
	}

	keyName, input, err := parseKeyToken(keyToken, mods)
	if err != nil {
		return "", KeyInput{}, err
	}

	prefixes := make([]string, 0, 5)
	if mods&KeyModCtrl != 0 {
		prefixes = append(prefixes, "ctrl")
	}
	if mods&KeyModAlt != 0 {
		prefixes = append(prefixes, "alt")
	}
	if mods&KeyModShift != 0 && len([]rune(keyName)) != 1 {
		prefixes = append(prefixes, "shift")
	}
	if mods&KeyModMeta != 0 {
		prefixes = append(prefixes, "meta")
	}
	if mods&KeyModSuper != 0 {
		prefixes = append(prefixes, "super")
	}
	if len(prefixes) == 0 {
		return keyName, input, nil
	}
	return strings.Join(append(prefixes, keyName), "+"), input, nil
}

func parseKeyToken(token string, mods KeyMods) (string, KeyInput, error) {
	lower := strings.ToLower(token)
	switch lower {
	case "up":
		return "up", KeyInput{Code: KeyCodeUp, Mods: mods}, nil
	case "down":
		return "down", KeyInput{Code: KeyCodeDown, Mods: mods}, nil
	case "left":
		return "left", KeyInput{Code: KeyCodeLeft, Mods: mods}, nil
	case "right":
		return "right", KeyInput{Code: KeyCodeRight, Mods: mods}, nil
	case "home":
		return "home", KeyInput{Code: KeyCodeHome, Mods: mods}, nil
	case "end":
		return "end", KeyInput{Code: KeyCodeEnd, Mods: mods}, nil
	case "pgup", "pageup":
		return "pgup", KeyInput{Code: KeyCodePageUp, Mods: mods}, nil
	case "pgdown", "pagedown":
		return "pgdown", KeyInput{Code: KeyCodePageDown, Mods: mods}, nil
	case "insert", "ins":
		return "insert", KeyInput{Code: KeyCodeInsert, Mods: mods}, nil
	case "delete", "del":
		return "delete", KeyInput{Code: KeyCodeDelete, Mods: mods}, nil
	case "enter", "return":
		return "enter", KeyInput{Code: KeyCodeEnter, Mods: mods}, nil
	case "backspace", "bs":
		return "backspace", KeyInput{Code: KeyCodeBackspace, Mods: mods}, nil
	case "tab":
		return "tab", KeyInput{Code: KeyCodeTab, Mods: mods}, nil
	case "esc", "escape":
		return "esc", KeyInput{Code: KeyCodeEscape, Mods: mods}, nil
	case "space":
		if mods == 0 {
			return " ", KeyInput{Code: ' '}, nil
		}
		return "space", KeyInput{Code: ' ', Mods: mods}, nil
	}

	if len(lower) >= 2 && lower[0] == 'f' {
		switch lower {
		case "f1":
			return "f1", KeyInput{Code: KeyCodeF1, Mods: mods}, nil
		case "f2":
			return "f2", KeyInput{Code: KeyCodeF2, Mods: mods}, nil
		case "f3":
			return "f3", KeyInput{Code: KeyCodeF3, Mods: mods}, nil
		case "f4":
			return "f4", KeyInput{Code: KeyCodeF4, Mods: mods}, nil
		case "f5":
			return "f5", KeyInput{Code: KeyCodeF5, Mods: mods}, nil
		case "f6":
			return "f6", KeyInput{Code: KeyCodeF6, Mods: mods}, nil
		case "f7":
			return "f7", KeyInput{Code: KeyCodeF7, Mods: mods}, nil
		case "f8":
			return "f8", KeyInput{Code: KeyCodeF8, Mods: mods}, nil
		case "f9":
			return "f9", KeyInput{Code: KeyCodeF9, Mods: mods}, nil
		case "f10":
			return "f10", KeyInput{Code: KeyCodeF10, Mods: mods}, nil
		case "f11":
			return "f11", KeyInput{Code: KeyCodeF11, Mods: mods}, nil
		case "f12":
			return "f12", KeyInput{Code: KeyCodeF12, Mods: mods}, nil
		}
	}

	runes := []rune(token)
	if len(runes) != 1 {
		return "", KeyInput{}, fmt.Errorf("unsupported key %q", token)
	}

	r := runes[0]
	if unicode.IsLetter(r) {
		if unicode.IsUpper(r) {
			mods |= KeyModShift
		}
		r = unicode.ToLower(r)
	}

	input := KeyInput{Code: r, Mods: mods}
	return string(runes), input, nil
}
