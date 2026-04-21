package shux

import (
	"fmt"
	"strconv"
	"strings"
)

// Command represents a parsed command with its arguments.
type Command struct {
	Name string
	Args []string
}

// ParseCommand parses a command string into a Command.
// Commands follow tmux-like syntax: "command-name arg1 arg2 ..."
func ParseCommand(input string) (Command, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Command{}, fmt.Errorf("empty command")
	}

	// Simple tokenization - split on whitespace, respecting basic quoting
	fields := tokenizeCommand(input)
	if len(fields) == 0 {
		return Command{}, fmt.Errorf("empty command")
	}

	return Command{
		Name: fields[0],
		Args: fields[1:],
	}, nil
}

// tokenizeCommand splits input into fields, handling basic quoting.
func tokenizeCommand(input string) []string {
	var fields []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range input {
		switch {
		case !inQuote && (r == '"' || r == '\''):
			inQuote = true
			quoteChar = r
		case inQuote && r == quoteChar:
			inQuote = false
			quoteChar = 0
		case !inQuote && (r == ' ' || r == '\t'):
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		fields = append(fields, current.String())
	}

	return fields
}

// ToActionMsg converts a Command to an ActionMsg that can be dispatched.
// Returns (msg, ok) where ok is false if command is unknown.
func (c Command) ToActionMsg() (ActionMsg, bool) {
	msg, err := c.toActionMsg()
	if err != nil {
		return ActionMsg{}, false
	}
	return msg, true
}

// ToActionMsg converts a Command to an ActionMsg with full error details.
// This is used internally by keymap parsing to maintain DRY command handling.
func (c Command) toActionMsg() (ActionMsg, error) {
	switch c.Name {
	case "new-window":
		if len(c.Args) > 0 {
			return ActionMsg{}, fmt.Errorf("new-window takes no arguments")
		}
		return ActionMsg{Action: ActionNewWindow}, nil

	case "next-window":
		if len(c.Args) > 0 {
			return ActionMsg{}, fmt.Errorf("next-window takes no arguments")
		}
		return ActionMsg{Action: ActionNextWindow}, nil

	case "previous-window", "prev-window":
		if len(c.Args) > 0 {
			return ActionMsg{}, fmt.Errorf("previous-window takes no arguments")
		}
		return ActionMsg{Action: ActionPrevWindow}, nil

	case "select-window":
		if len(c.Args) != 1 {
			return ActionMsg{}, fmt.Errorf("select-window requires exactly one argument: <index>")
		}
		idx, err := strconv.Atoi(c.Args[0])
		if err != nil || idx < 0 || idx > 9 {
			return ActionMsg{}, fmt.Errorf("select-window requires a window index 0-9")
		}
		action := Action("select_window_" + strconv.Itoa(idx))
		return ActionMsg{Action: action}, nil

	case "last-window":
		if len(c.Args) > 0 {
			return ActionMsg{}, fmt.Errorf("last-window takes no arguments")
		}
		return ActionMsg{Action: ActionLastWindow}, nil

	case "kill-window":
		if len(c.Args) > 0 {
			return ActionMsg{}, fmt.Errorf("kill-window takes no arguments")
		}
		return ActionMsg{Action: ActionKillWindow}, nil

	case "kill-session":
		if len(c.Args) > 0 {
			return ActionMsg{}, fmt.Errorf("kill-session takes no arguments")
		}
		return ActionMsg{Action: ActionKillSession}, nil

	case "rename-window":
		if len(c.Args) != 1 {
			return ActionMsg{}, fmt.Errorf("rename-window requires exactly one argument: <name>")
		}
		return ActionMsg{Action: ActionRenameWindow, Args: []string{c.Args[0]}}, nil

	case "split-window":
		if len(c.Args) != 1 {
			return ActionMsg{}, fmt.Errorf("split-window requires exactly one flag: -h or -v")
		}
		switch c.Args[0] {
		case "-h":
			return ActionMsg{Action: ActionSplitVertical}, nil
		case "-v":
			return ActionMsg{Action: ActionSplitHorizontal}, nil
		default:
			return ActionMsg{}, fmt.Errorf("split-window requires -h (horizontal) or -v (vertical)")
		}

	case "select-pane":
		if len(c.Args) != 1 {
			return ActionMsg{}, fmt.Errorf("select-pane requires exactly one flag: -L, -R, -U, or -D")
		}
		switch c.Args[0] {
		case "-L":
			return ActionMsg{Action: ActionSelectPaneLeft}, nil
		case "-R":
			return ActionMsg{Action: ActionSelectPaneRight}, nil
		case "-U":
			return ActionMsg{Action: ActionSelectPaneUp}, nil
		case "-D":
			return ActionMsg{Action: ActionSelectPaneDown}, nil
		default:
			return ActionMsg{}, fmt.Errorf("select-pane requires -L, -R, -U, or -D")
		}

	case "kill-pane":
		if len(c.Args) > 0 {
			return ActionMsg{}, fmt.Errorf("kill-pane takes no arguments")
		}
		return ActionMsg{Action: ActionKillPane}, nil

	case "resize-pane":
		if len(c.Args) < 1 || len(c.Args) > 2 {
			return ActionMsg{}, fmt.Errorf("resize-pane requires a direction flag and optional amount")
		}

		amount := 1
		if len(c.Args) == 2 {
			n, err := strconv.Atoi(c.Args[1])
			if err != nil || n <= 0 {
				return ActionMsg{}, fmt.Errorf("resize-pane amount must be a positive number")
			}
			amount = n
		}

		switch c.Args[0] {
		case "-L":
			return ActionMsg{Action: ActionResizePaneLeft, Amount: amount}, nil
		case "-R":
			return ActionMsg{Action: ActionResizePaneRight, Amount: amount}, nil
		case "-U":
			return ActionMsg{Action: ActionResizePaneUp, Amount: amount}, nil
		case "-D":
			return ActionMsg{Action: ActionResizePaneDown, Amount: amount}, nil
		case "-Z":
			if len(c.Args) != 1 {
				return ActionMsg{}, fmt.Errorf("resize-pane -Z takes no additional arguments")
			}
			return ActionMsg{Action: ActionZoomPane}, nil
		default:
			return ActionMsg{}, fmt.Errorf("resize-pane requires -L, -R, -U, -D, or -Z")
		}

	case "swap-pane":
		if len(c.Args) != 1 {
			return ActionMsg{}, fmt.Errorf("swap-pane requires exactly one flag: -U or -D")
		}
		switch c.Args[0] {
		case "-U":
			return ActionMsg{Action: ActionSwapPaneUp}, nil
		case "-D":
			return ActionMsg{Action: ActionSwapPaneDown}, nil
		default:
			return ActionMsg{}, fmt.Errorf("swap-pane requires -U or -D")
		}

	case "rename-session":
		if len(c.Args) != 1 {
			return ActionMsg{}, fmt.Errorf("rename-session requires exactly one argument: <name>")
		}
		return ActionMsg{Action: ActionRenameSession, Args: []string{c.Args[0]}}, nil

	case "list-sessions":
		if len(c.Args) > 0 {
			return ActionMsg{}, fmt.Errorf("list-sessions takes no arguments")
		}
		return ActionMsg{Action: ActionListSessions}, nil

	case "attach-session":
		if len(c.Args) != 1 {
			return ActionMsg{}, fmt.Errorf("attach-session requires exactly one argument: <name>")
		}
		return ActionMsg{Action: ActionAttachSession, Args: []string{c.Args[0]}}, nil

	case "choose-tree":
		if len(c.Args) != 1 {
			return ActionMsg{}, fmt.Errorf("choose-tree requires exactly one flag: -s or -w")
		}
		switch c.Args[0] {
		case "-s":
			return ActionMsg{Action: ActionChooseTreeSessions}, nil
		case "-w":
			return ActionMsg{Action: ActionChooseTreeWindows}, nil
		default:
			return ActionMsg{}, fmt.Errorf("choose-tree requires -s (sessions) or -w (windows)")
		}

	case "detach", "detach-client":
		if len(c.Args) > 0 {
			return ActionMsg{}, fmt.Errorf("detach takes no arguments")
		}
		return ActionMsg{Action: ActionDetach}, nil

	case "list-keys":
		if len(c.Args) > 0 {
			return ActionMsg{}, fmt.Errorf("list-keys takes no arguments")
		}
		return ActionMsg{Action: ActionShowHelp}, nil

	case "quit":
		if len(c.Args) > 0 {
			return ActionMsg{}, fmt.Errorf("quit takes no arguments")
		}
		return ActionMsg{Action: ActionQuit}, nil

	default:
		return ActionMsg{}, fmt.Errorf("unknown command: %s", c.Name)
	}
}

// ExecuteCommandMsg is sent to request command execution.
type ExecuteCommandMsg struct {
	Command string
}

// CommandResult is returned after attempting command execution.
type CommandResult struct {
	Success bool
	Error   string
	Quit    bool // true if command requested quit (detach/quit)
}

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
			// Show examples if any arg has one
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
