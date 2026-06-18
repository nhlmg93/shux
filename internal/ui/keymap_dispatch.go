package ui

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"shux/internal/cfg"
	"shux/internal/luabind"
	"shux/internal/protocol"
)

func (m Model) handlePrefixKey(key string) (Model, tea.Cmd) {
	m.Prefix = false
	b, ok := m.Keymaps.Lookup("prefix", key)
	if !ok {
		return m, nil
	}
	if b.LuaCallback != 0 {
		var rt luabind.Runtime = m.Lua
		if rt != nil {
			rt.CallKeymapRef(b.LuaCallback)
		}
		return m, nil
	}
	return m.dispatchBuiltin(b.Builtin)
}

func (m Model) dispatchBuiltin(action cfg.BuiltinKeyAction) (Model, tea.Cmd) {
	switch action {
	case cfg.ActionDetach:
		if m.OnExit != nil {
			m.OnExit(ExitDetach)
		}
		return m, tea.Quit
	case cfg.ActionQuit:
		if m.OnExit != nil {
			m.OnExit(ExitQuit)
		}
		return m, tea.Quit
	case cfg.ActionSplitLR:
		return m.startPaneSplit(protocol.SplitVertical)
	case cfg.ActionSplitTB:
		return m.startPaneSplit(protocol.SplitHorizontal)
	case cfg.ActionResizePaneLeft:
		return m.startPaneResize(protocol.PaneResizeEdgeLeft)
	case cfg.ActionResizePaneDown:
		return m.startPaneResize(protocol.PaneResizeEdgeDown)
	case cfg.ActionResizePaneUp:
		return m.startPaneResize(protocol.PaneResizeEdgeUp)
	case cfg.ActionResizePaneRight:
		return m.startPaneResize(protocol.PaneResizeEdgeRight)
	case cfg.ActionNextPane:
		m.ActivePaneID = cycleActivePane(m.ActivePaneID, m.Layout.Panes)
		m.Layout.ActivePane = m.ActivePaneID
		return m, nil
	case cfg.ActionFocusPaneLeft:
		return m.startPaneFocusDirection(protocol.PaneFocusLeft)
	case cfg.ActionFocusPaneRight:
		return m.startPaneFocusDirection(protocol.PaneFocusRight)
	case cfg.ActionFocusPaneUp:
		return m.startPaneFocusDirection(protocol.PaneFocusUp)
	case cfg.ActionFocusPaneDown:
		return m.startPaneFocusDirection(protocol.PaneFocusDown)
	case cfg.ActionDisplayPanes:
		return m.startPaneQuickSelect()
	case cfg.ActionClosePane:
		return m.startPaneClose(m.ActivePaneID)
	case cfg.ActionTogglePaneZoom:
		return m.startPaneZoomToggle(m.ActivePaneID)
	case cfg.ActionNewWindow:
		return m.startWindowCreate()
	case cfg.ActionNextWindow:
		m = m.switchWindowByOffset(1)
		return m, m.currentWindowResizeCmd()
	case cfg.ActionPreviousWindow:
		m = m.switchWindowByOffset(-1)
		return m, m.currentWindowResizeCmd()
	case cfg.ActionSelectWindow1:
		m = m.switchWindowByNumber(1)
		return m, m.currentWindowResizeCmd()
	case cfg.ActionSelectWindow2:
		m = m.switchWindowByNumber(2)
		return m, m.currentWindowResizeCmd()
	case cfg.ActionSelectWindow3:
		m = m.switchWindowByNumber(3)
		return m, m.currentWindowResizeCmd()
	case cfg.ActionSelectWindow4:
		m = m.switchWindowByNumber(4)
		return m, m.currentWindowResizeCmd()
	case cfg.ActionSelectWindow5:
		m = m.switchWindowByNumber(5)
		return m, m.currentWindowResizeCmd()
	case cfg.ActionSelectWindow6:
		m = m.switchWindowByNumber(6)
		return m, m.currentWindowResizeCmd()
	case cfg.ActionSelectWindow7:
		m = m.switchWindowByNumber(7)
		return m, m.currentWindowResizeCmd()
	case cfg.ActionSelectWindow8:
		m = m.switchWindowByNumber(8)
		return m, m.currentWindowResizeCmd()
	case cfg.ActionSelectWindow9:
		m = m.switchWindowByNumber(9)
		return m, m.currentWindowResizeCmd()
	case cfg.ActionSelectWindow10:
		m = m.switchWindowByNumber(10)
		return m, m.currentWindowResizeCmd()
	case cfg.ActionToggleSyncPanes:
		return m, m.dispatch(protocol.CommandWindowToggleSyncPanes{
			SessionID: m.SessionID,
			WindowID:  m.WindowID,
		})
	case cfg.ActionListKeymaps:
		for _, b := range m.Keymaps.List("prefix") {
			action := string(b.Builtin)
			if action == "" {
				action = "lua"
			}
			fmt.Fprintf(os.Stderr, "  %s%s  %s\n", m.MapLeader, b.Key, action)
			if b.Desc != "" {
				fmt.Fprintf(os.Stderr, "    %s\n", b.Desc)
			}
		}
		return m, nil
	case cfg.ActionCopyModeToggle:
		if m.CopyMode {
			return m.exitCopyMode(), nil
		}
		return m.enterCopyMode(), nil
	case cfg.ActionPasteRegister:
		if m.CopyRegister == "" {
			return m, nil
		}
		return m, m.dispatch(protocol.CommandPanePaste{
			SessionID: m.SessionID,
			WindowID:  m.WindowID,
			PaneID:    m.ActivePaneID,
			Data:      []byte(m.CopyRegister),
		})
	case cfg.ActionRenameWindow:
		return m.startWindowRename()
	case cfg.ActionRenamePane:
		return m.startPaneRename()
	default:
		return m, nil
	}
}
