package shux

import (
	"fmt"
	"strconv"
)

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
