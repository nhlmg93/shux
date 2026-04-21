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
	ActionQuit            Action = "quit"
	ActionNewWindow       Action = "new_window"
	ActionNextWindow      Action = "next_window"
	ActionPrevWindow      Action = "prev_window"
	ActionSplitHorizontal Action = "split_horizontal"
	ActionSplitVertical   Action = "split_vertical"
	ActionSelectPaneLeft  Action = "select_pane_left"
	ActionSelectPaneDown  Action = "select_pane_down"
	ActionSelectPaneUp    Action = "select_pane_up"
	ActionSelectPaneRight Action = "select_pane_right"
	ActionDetach          Action = "detach"
)

var validActions = map[Action]struct{}{
	ActionQuit:            {},
	ActionNewWindow:       {},
	ActionNextWindow:      {},
	ActionPrevWindow:      {},
	ActionSplitHorizontal: {},
	ActionSplitVertical:   {},
	ActionSelectPaneLeft:  {},
	ActionSelectPaneDown:  {},
	ActionSelectPaneUp:    {},
	ActionSelectPaneRight: {},
	ActionDetach:          {},
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
	bindings    map[string]Action
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

	bindings := map[string]Action{}
	for spec, action := range tmuxDefaultBindingSpecs() {
		key, _, err := parseKeySpec(spec)
		if err != nil {
			return Keymap{}, fmt.Errorf("default binding %q: %w", spec, err)
		}
		bindings[key] = action
	}

	for _, spec := range cfg.Unbind {
		key, _, err := parseKeySpec(spec)
		if err != nil {
			return Keymap{}, fmt.Errorf("unbind %q: %w", spec, err)
		}
		delete(bindings, key)
	}

	for spec, actionName := range cfg.Bind {
		action, err := resolveAction(actionName)
		if err != nil {
			return Keymap{}, fmt.Errorf("bind %q: %w", spec, err)
		}
		key, _, err := parseKeySpec(spec)
		if err != nil {
			return Keymap{}, fmt.Errorf("bind %q: %w", spec, err)
		}
		bindings[key] = action
	}

	if _, exists := bindings[prefix]; exists {
		return Keymap{}, fmt.Errorf("binding conflict: prefix %q is also bound as an action", prefixSpec)
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
	action, ok := k.bindings[key]
	return action, ok
}

func (k Keymap) PrefixInput() KeyInput {
	return k.prefixInput
}

func (k Keymap) Bindings() map[string]Action {
	result := make(map[string]Action, len(k.bindings))
	for key, action := range k.bindings {
		result[key] = action
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

func tmuxDefaultBindingSpecs() map[string]Action {
	return map[string]Action{
		"c":     ActionNewWindow,
		"n":     ActionNextWindow,
		"p":     ActionPrevWindow,
		`"`:     ActionSplitHorizontal,
		"%":     ActionSplitVertical,
		"Left":  ActionSelectPaneLeft,
		"Down":  ActionSelectPaneDown,
		"Up":    ActionSelectPaneUp,
		"Right": ActionSelectPaneRight,
		"d":     ActionDetach,
	}
}

func parseAction(name string) (Action, error) {
	action := Action(strings.TrimSpace(name))
	if _, ok := validActions[action]; !ok {
		return "", fmt.Errorf("unknown action %q", name)
	}
	return action, nil
}

func resolveAction(name string) (Action, error) {
	if action, err := parseAction(name); err == nil {
		return action, nil
	}
	return parseTmuxCommand(name)
}

func parseTmuxCommand(command string) (Action, error) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return "", fmt.Errorf("empty command")
	}

	switch fields[0] {
	case "new-window":
		if len(fields) != 1 {
			return "", fmt.Errorf("unsupported command %q", command)
		}
		return ActionNewWindow, nil
	case "next-window":
		if len(fields) != 1 {
			return "", fmt.Errorf("unsupported command %q", command)
		}
		return ActionNextWindow, nil
	case "previous-window", "prev-window":
		if len(fields) != 1 {
			return "", fmt.Errorf("unsupported command %q", command)
		}
		return ActionPrevWindow, nil
	case "detach", "detach-client":
		if len(fields) != 1 {
			return "", fmt.Errorf("unsupported command %q", command)
		}
		return ActionDetach, nil
	case "split-window":
		if len(fields) != 2 {
			return "", fmt.Errorf("unsupported command %q", command)
		}
		switch fields[1] {
		case "-h":
			return ActionSplitVertical, nil
		case "-v":
			return ActionSplitHorizontal, nil
		default:
			return "", fmt.Errorf("unsupported command %q", command)
		}
	case "select-pane":
		if len(fields) != 2 {
			return "", fmt.Errorf("unsupported command %q", command)
		}
		switch fields[1] {
		case "-L":
			return ActionSelectPaneLeft, nil
		case "-D":
			return ActionSelectPaneDown, nil
		case "-U":
			return ActionSelectPaneUp, nil
		case "-R":
			return ActionSelectPaneRight, nil
		default:
			return "", fmt.Errorf("unsupported command %q", command)
		}
	case "quit":
		if len(fields) != 1 {
			return "", fmt.Errorf("unsupported command %q", command)
		}
		return ActionQuit, nil
	default:
		return "", fmt.Errorf("unknown action or command %q", command)
	}
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
