package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"shux/internal/cfg"
	"shux/internal/protocol"
)

func (m Model) startCommandPrompt() Model {
	m.CommandOpen = true
	m.CommandInput = ""
	return m
}

func (m Model) handleCommandKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Key().String() {
	case "esc", "ctrl+c":
		m.CommandOpen = false
		m.CommandInput = ""
		return m, nil
	case "enter":
		line := strings.TrimSpace(m.CommandInput)
		m.CommandOpen = false
		m.CommandInput = ""
		return m.executeCommand(line)
	case "backspace":
		if m.CommandInput == "" {
			return m, nil
		}
		_, size := utf8.DecodeLastRuneInString(m.CommandInput)
		if size <= 0 {
			m.CommandInput = ""
		} else {
			m.CommandInput = m.CommandInput[:len(m.CommandInput)-size]
		}
		return m, nil
	}
	if msg.Key().Text == "" {
		return m, nil
	}
	m.CommandInput += msg.Key().Text
	return m, nil
}

func (m Model) commandPrompt() string {
	if !m.CommandOpen {
		return ""
	}
	return ":" + m.CommandInput
}

func (m Model) executeCommand(line string) (Model, tea.Cmd) {
	if line == "" {
		return m, nil
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return m, nil
	}
	switch fields[0] {
	case "detach", "detach-client":
		if m.OnExit != nil {
			m.OnExit(ExitDetach)
		}
		return m, tea.Quit
	case "quit", "exit":
		if m.OnExit != nil {
			m.OnExit(ExitQuit)
		}
		return m, tea.Quit
	case "kill-window", "close-window":
		return m.startWindowClose()
	case "kill-pane", "close-pane":
		return m.startPaneClose(m.ActivePaneID)
	case "new-window":
		return m.startWindowCreate()
	case "next-window", "nwin":
		m = m.switchWindowByOffset(1)
		return m, m.currentWindowResizeCmd()
	case "previous-window", "pwin":
		m = m.switchWindowByOffset(-1)
		return m, m.currentWindowResizeCmd()
	case "select-window", "switch-window":
		if n, ok := commandTargetIndex(fields[1:]); ok {
			m = m.switchWindowByNumber(n)
			return m, m.currentWindowResizeCmd()
		}
	case "rename-window":
		if name := commandRemainder(line, fields[0]); name != "" {
			return m, m.dispatch(protocol.CommandWindowRename{
				SessionID: m.SessionID,
				WindowID:  m.WindowID,
				Name:      name,
			})
		}
		return m.startWindowRename()
	case "rename-pane":
		if name := commandRemainder(line, fields[0]); name != "" {
			return m, m.dispatch(protocol.CommandPaneRename{
				SessionID: m.SessionID,
				WindowID:  m.WindowID,
				PaneID:    m.ActivePaneID,
				Name:      name,
			})
		}
		return m.startPaneRename()
	case "split-window", "split-pane":
		switch commandSplitFlag(fields[1:]) {
		case protocol.SplitVertical:
			return m.startPaneSplit(protocol.SplitVertical)
		case protocol.SplitHorizontal:
			return m.startPaneSplit(protocol.SplitHorizontal)
		}
	case "display-panes", "display-pane":
		return m.startPaneQuickSelect()
	case "choose-tree", "tree":
		return m.startTreeView(treeViewDefault)
	case "choose-session":
		return m.startTreeView(treeViewSessionsCollapsed)
	case "choose-window":
		return m.startTreeView(treeViewWindowsCollapsed)
	case "list-keymaps", "list-keys", "show-keys":
		return m, m.dispatchBuiltinListKeymaps()
	case "resize-pane":
		if edge, ok := commandResizeEdge(fields[1:]); ok {
			return m.startPaneResize(edge)
		}
	case "select-pane":
		if dir, ok := commandFocusDirection(fields[1:]); ok {
			return m.startPaneFocusDirection(dir)
		}
	case "next-pane", "select-pane-next":
		m.ActivePaneID = cycleActivePane(m.ActivePaneID, m.Layout.Panes)
		m.Layout.ActivePane = m.ActivePaneID
		return m, nil
	case "toggle-pane-zoom", "zoom-pane":
		return m.startPaneZoomToggle(m.ActivePaneID)
	case "toggle-sync-panes", "sync-panes":
		return m, m.dispatch(protocol.CommandWindowToggleSyncPanes{
			SessionID: m.SessionID,
			WindowID:  m.WindowID,
		})
	case "help":
		return m, m.dispatchBuiltinListKeymaps()
	}
	fmt.Fprintf(os.Stderr, "shux: unknown command: %q\n", fields[0])
	return m, nil
}

func commandRemainder(line, verb string) string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, verb) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(line, verb))
}

func commandTargetIndex(args []string) (int, bool) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-t", "-T":
			if i+1 >= len(args) {
				return 0, false
			}
			return parseWindowNumber(args[i+1])
		default:
			return parseWindowNumber(args[i])
		}
	}
	return 0, false
}

func parseWindowNumber(s string) (int, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

func commandSplitFlag(args []string) protocol.SplitDirection {
	for _, arg := range args {
		switch arg {
		case "-h", "-H", "horizontal", "lr", "left-right":
			return protocol.SplitVertical
		case "-v", "-V", "vertical", "tb", "top-bottom":
			return protocol.SplitHorizontal
		}
	}
	return 0
}

func commandResizeEdge(args []string) (protocol.PaneResizeEdge, bool) {
	if len(args) == 0 {
		return 0, false
	}
	switch args[0] {
	case "-L", "left":
		return protocol.PaneResizeEdgeLeft, true
	case "-R", "right":
		return protocol.PaneResizeEdgeRight, true
	case "-U", "up":
		return protocol.PaneResizeEdgeUp, true
	case "-D", "down":
		return protocol.PaneResizeEdgeDown, true
	default:
		return 0, false
	}
}

func commandFocusDirection(args []string) (protocol.PaneFocusDirection, bool) {
	if len(args) == 0 {
		return 0, false
	}
	switch args[0] {
	case "-L", "left":
		return protocol.PaneFocusLeft, true
	case "-R", "right":
		return protocol.PaneFocusRight, true
	case "-U", "up":
		return protocol.PaneFocusUp, true
	case "-D", "down":
		return protocol.PaneFocusDown, true
	default:
		return 0, false
	}
}

func (m Model) dispatchBuiltinListKeymaps() tea.Cmd {
	_, cmd := m.dispatchBuiltin(cfg.ActionListKeymaps)
	return cmd
}
