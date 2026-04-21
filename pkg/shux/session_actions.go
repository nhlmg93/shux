package shux

import (
	"fmt"
)

// dispatchAction handles session-scoped and window-scoped actions.
func (s *Session) dispatchAction(msg ActionMsg) ActionResult {
	switch msg.Action {

	case ActionQuit:
		return ActionResult{Quit: true}
	case ActionNewWindow:

		rows, cols := s.lastRows, s.lastCols
		if rows <= 0 || cols <= 0 {
			rows, cols = 24, 80
		}
		s.createWindow(rows, cols)
	case ActionNextWindow:
		s.switchWindow(1)
	case ActionPrevWindow:
		s.switchWindow(-1)
	case ActionLastWindow:
		if s.lastActive != 0 {
			if _, ok := s.windows[s.lastActive]; ok {
				s.setActiveWindow(s.lastActive)
			}
		}
	case ActionDetach:
		if err := s.handleDetach(); err != nil {
			s.logger.Warnf("detach: session=%s id=%d failed err=%v", s.name, s.id, err)
			return ActionResult{Err: err}
		}
		return ActionResult{Quit: true}
	case ActionRenameSession:

		s.logger.Infof("session: rename requested (not yet implemented)")

	case ActionSplitHorizontal:
		s.forwardToActiveWindow(Split{Dir: SplitH})
	case ActionSplitVertical:
		s.forwardToActiveWindow(Split{Dir: SplitV})
	case ActionSelectPaneLeft:
		s.forwardToActiveWindow(NavigatePane{Dir: PaneNavLeft})
	case ActionSelectPaneDown:
		s.forwardToActiveWindow(NavigatePane{Dir: PaneNavDown})
	case ActionSelectPaneUp:
		s.forwardToActiveWindow(NavigatePane{Dir: PaneNavUp})
	case ActionSelectPaneRight:
		s.forwardToActiveWindow(NavigatePane{Dir: PaneNavRight})
	case ActionResizePaneLeft:
		s.forwardToActiveWindow(ResizePane{Dir: PaneNavLeft, Amount: msg.Amount})
	case ActionResizePaneDown:
		s.forwardToActiveWindow(ResizePane{Dir: PaneNavDown, Amount: msg.Amount})
	case ActionResizePaneUp:
		s.forwardToActiveWindow(ResizePane{Dir: PaneNavUp, Amount: msg.Amount})
	case ActionResizePaneRight:
		s.forwardToActiveWindow(ResizePane{Dir: PaneNavRight, Amount: msg.Amount})
	case ActionSelectWindow0, ActionSelectWindow1, ActionSelectWindow2, ActionSelectWindow3,
		ActionSelectWindow4, ActionSelectWindow5, ActionSelectWindow6, ActionSelectWindow7,
		ActionSelectWindow8, ActionSelectWindow9:
		idx := int(msg.Action[len("select_window_")] - '0')
		if idx >= 0 && idx < len(s.windowOrder) {
			s.setActiveWindow(s.windowOrder[idx])
		}
	case ActionKillWindow:
		s.killActiveWindow()
	case ActionKillSession:
		return s.killSession()

	case ActionKillPane, ActionZoomPane, ActionSwapPaneUp, ActionSwapPaneDown,
		ActionRenameWindow:
		s.forwardToActiveWindow(msg)

	case ActionListSessions:
		s.logger.Infof("session: list-sessions requested (not yet implemented)")
	case ActionAttachSession:
		if len(msg.Args) > 0 {
			s.logger.Infof("session: attach-session %q requested (not yet implemented)", msg.Args[0])
		} else {
			s.logger.Infof("session: attach-session requested without name (not yet implemented)")
		}

	case ActionCommandPrompt, ActionChooseTreeSessions, ActionChooseTreeWindows, ActionShowHelp:
		s.logger.Infof("session: action %q requested (not yet implemented)", msg.Action)

	default:
		s.logger.Warnf("session: unknown action %q", msg.Action)
	}
	return ActionResult{}
}

// handleExecuteCommand parses and executes a command string.
func (s *Session) handleExecuteCommand(msg ExecuteCommandMsg) {
	cmd, err := ParseCommand(msg.Command)
	if err != nil {
		s.logger.Warnf("command: parse error: %v", err)
		return
	}

	actionMsg, ok := cmd.ToActionMsg()
	if !ok {
		s.logger.Warnf("command: unknown command %q", cmd.Name)
		return
	}

	result := s.dispatchAction(actionMsg)
	if result.Err != nil {
		s.logger.Warnf("command: execution error: %v", result.Err)
		return
	}
	if result.Quit {
		s.logger.Infof("command: %q triggered quit", msg.Command)
	}
}

// executeCommandWithResult parses and executes a command, returning a CommandResult.
func (s *Session) executeCommandWithResult(msg ExecuteCommandMsg) CommandResult {
	cmd, err := ParseCommand(msg.Command)
	if err != nil {
		return CommandResult{Success: false, Error: err.Error()}
	}

	actionMsg, ok := cmd.ToActionMsg()
	if !ok {
		return CommandResult{Success: false, Error: fmt.Sprintf("unknown command: %s", cmd.Name)}
	}

	result := s.dispatchAction(actionMsg)
	if result.Err != nil {
		return CommandResult{Success: false, Error: result.Err.Error()}
	}
	return CommandResult{Success: true, Quit: result.Quit}
}
