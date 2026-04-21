package shux

import (
	"fmt"
	"strings"
)

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
