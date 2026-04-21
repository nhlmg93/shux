package shux

import (
	"strings"
)

// CommandRegistry provides a registry of available commands for help/documentation.
type CommandRegistry struct {
	commands map[string]CommandInfo
}

// CommandInfo describes a registered command.
type CommandInfo struct {
	Name        string
	Args        []ArgInfo
	Description string
}

// ArgInfo describes a command argument.
type ArgInfo struct {
	Name     string
	Required bool
	Example  string // Optional usage example (e.g., "0-9" for select-window)
}

// NewCommandRegistry creates a registry with all v0.1.0 commands.
func NewCommandRegistry() *CommandRegistry {
	r := &CommandRegistry{commands: make(map[string]CommandInfo)}
	r.registerV1Commands()
	return r
}

// registerV1Commands registers all v0.1.0 commands.
func (r *CommandRegistry) registerV1Commands() {
	commands := []CommandInfo{
		{Name: "new-window", Description: "Create a new window"},
		{Name: "next-window", Description: "Select the next window"},
		{Name: "previous-window", Description: "Select the previous window"},
		{Name: "select-window", Args: []ArgInfo{{Name: "index", Required: true, Example: "0-9"}}, Description: "Select window by index"},
		{Name: "last-window", Description: "Switch to the previously selected window"},
		{Name: "kill-window", Description: "Kill the current window"},
		{Name: "kill-session", Description: "Kill the entire session (all windows)"},
		{Name: "rename-window", Args: []ArgInfo{{Name: "name", Required: true, Example: "\"my window\""}}, Description: "Rename the current window"},
		{Name: "split-window", Args: []ArgInfo{{Name: "-h|-v", Required: true, Example: "-h or -v"}}, Description: "Split window horizontally (-v) or vertically (-h)"},
		{Name: "select-pane", Args: []ArgInfo{{Name: "-L|-R|-U|-D", Required: true, Example: "-L"}}, Description: "Select pane in direction"},
		{Name: "kill-pane", Description: "Kill the active pane"},
		{Name: "resize-pane", Args: []ArgInfo{{Name: "-L|-R|-U|-D|-Z", Required: true, Example: "-L 10"}, {Name: "amount", Required: false}}, Description: "Resize pane left/right/up/down or zoom (-Z)"},
		{Name: "swap-pane", Args: []ArgInfo{{Name: "-U|-D", Required: true, Example: "-U or -D"}}, Description: "Swap pane with previous (-U) or next (-D)"},
		{Name: "rename-session", Args: []ArgInfo{{Name: "name", Required: true, Example: "\"my session\""}}, Description: "Rename the current session"},
		{Name: "list-sessions", Description: "List all saved sessions"},
		{Name: "attach-session", Args: []ArgInfo{{Name: "name", Required: true, Example: "\"session name\""}}, Description: "Attach to a saved session"},
		{Name: "choose-tree", Args: []ArgInfo{{Name: "-s|-w", Required: true, Example: "-s or -w"}}, Description: "Open tree view for sessions (-s) or windows (-w)"},
		{Name: "detach", Description: "Detach from session and save state"},
		{Name: "list-keys", Description: "Show all key bindings"},
		{Name: "quit", Description: "Quit shux"},
	}

	for _, cmd := range commands {
		r.commands[cmd.Name] = cmd
	}
}

// Get returns command info by name.
func (r *CommandRegistry) Get(name string) (CommandInfo, bool) {
	info, ok := r.commands[name]
	return info, ok
}

// List returns all registered commands sorted by name.
func (r *CommandRegistry) List() []CommandInfo {
	result := make([]CommandInfo, 0, len(r.commands))
	for _, info := range r.commands {
		result = append(result, info)
	}
	return result
}

// FormatHelp returns formatted help text for all commands.
func (r *CommandRegistry) FormatHelp() string {
	var b strings.Builder
	b.WriteString("Available Commands:\n\n")

	maxLen := 0
	for name := range r.commands {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}

	for name, info := range r.commands {
		b.WriteString("  ")
		b.WriteString(name)
		for i := len(name); i < maxLen+2; i++ {
			b.WriteString(" ")
		}
		b.WriteString(info.Description)
		if len(info.Args) > 0 {
			b.WriteString(" (args: ")
			for i, arg := range info.Args {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(arg.Name)
				if !arg.Required {
					b.WriteString("?")
				}
			}
			b.WriteString(")")

			hasExamples := false
			for _, arg := range info.Args {
				if arg.Example != "" {
					hasExamples = true
					break
				}
			}
			if hasExamples {
				b.WriteString("  e.g., ")
				b.WriteString(name)
				for _, arg := range info.Args {
					if arg.Example != "" {
						b.WriteString(" ")
						b.WriteString(arg.Example)
					} else if arg.Required {
						b.WriteString(" <")
						b.WriteString(arg.Name)
						b.WriteString(">")
					}
				}
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// v1CommandNames is the static list of all v0.1.0 command names.
// This is used for Lua completion and avoids unnecessary allocations.
var v1CommandNames = []string{
	"new-window",
	"next-window",
	"previous-window",
	"select-window",
	"last-window",
	"kill-window",
	"kill-session",
	"rename-window",
	"split-window",
	"select-pane",
	"kill-pane",
	"resize-pane",
	"swap-pane",
	"rename-session",
	"list-sessions",
	"attach-session",
	"choose-tree",
	"detach",
	"list-keys",
	"quit",
}

// ValidCommands returns a list of valid command names for completion.
// Returns a copy of the static list to prevent external modification.
func ValidCommands() []string {
	result := make([]string, len(v1CommandNames))
	copy(result, v1CommandNames)
	return result
}
