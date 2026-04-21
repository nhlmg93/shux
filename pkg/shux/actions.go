package shux

import (
	"fmt"
	"sort"
	"strings"
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

func ValidActions() []Action {
	actions := make([]Action, 0, len(validActions))
	for action := range validActions {
		actions = append(actions, action)
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i] < actions[j] })
	return actions
}

func parseAction(name string) (Action, error) {
	action := Action(strings.TrimSpace(name))
	if _, ok := validActions[action]; !ok {
		return "", fmt.Errorf("unknown action %q", name)
	}
	return action, nil
}
