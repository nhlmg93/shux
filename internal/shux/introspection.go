package shux

import (
	"strconv"
	"strings"

	"shux/internal/protocol"
)

func (a *Shux) ListWindows() []protocol.WindowInfo {
	if a.cache == nil {
		return nil
	}
	sessionID := a.DefaultSessionID
	windowIDs := a.cache.WindowIDs(sessionID)
	windows := make([]protocol.WindowInfo, 0, len(windowIDs))
	for i, windowID := range windowIDs {
		layout, ok := a.cache.LayoutSnapshot(sessionID, windowID)
		paneCount := 0
		if ok {
			paneCount = len(layout.Panes)
		}
		windows = append(windows, protocol.WindowInfo{
			Index:     i + 1,
			SessionID: sessionID,
			WindowID:  windowID,
			PaneCount: paneCount,
		})
	}
	return windows
}

func (a *Shux) ListPanes() []protocol.PaneInfo {
	if a.cache == nil {
		return nil
	}
	sessionID := a.DefaultSessionID
	windowIDs := a.cache.WindowIDs(sessionID)
	panes := make([]protocol.PaneInfo, 0, len(windowIDs))
	for i, windowID := range windowIDs {
		layout, ok := a.cache.LayoutSnapshot(sessionID, windowID)
		if !ok {
			continue
		}
		for _, pane := range layout.Panes {
			panes = append(panes, protocol.PaneInfo{
				Index:       len(panes) + 1,
				SessionID:   sessionID,
				WindowID:    windowID,
				WindowIndex: i + 1,
				PaneID:      pane.PaneID,
				Col:         pane.Col,
				Row:         pane.Row,
				Cols:        pane.Cols,
				Rows:        pane.Rows,
			})
		}
	}
	return panes
}

func (a *Shux) DisplayMessageContext() protocol.DisplayMessageContext {
	ctx := protocol.DisplayMessageContext{
		SessionID:   a.DefaultSessionID,
		WindowID:    a.DefaultWindowID,
		WindowIndex: 1,
		PaneID:      a.DefaultPaneID,
		PaneIndex:   1,
	}
	for _, window := range a.ListWindows() {
		if window.WindowID == ctx.WindowID {
			ctx.WindowIndex = window.Index
			break
		}
	}
	panes := a.ListPanes()
	for _, pane := range panes {
		if pane.PaneID == ctx.PaneID {
			ctx.PaneIndex = pane.Index
			return ctx
		}
	}
	if len(panes) > 0 {
		ctx.WindowID = panes[0].WindowID
		ctx.WindowIndex = panes[0].WindowIndex
		ctx.PaneID = panes[0].PaneID
		ctx.PaneIndex = panes[0].Index
	}
	return ctx
}

func FormatDisplayMessage(format string, ctx protocol.DisplayMessageContext) string {
	replacements := []string{
		"#{session_id}", string(ctx.SessionID),
		"#{window_id}", string(ctx.WindowID),
		"#{window_index}", strconv.Itoa(ctx.WindowIndex),
		"#{pane_id}", string(ctx.PaneID),
		"#{pane_index}", strconv.Itoa(ctx.PaneIndex),
	}
	return strings.NewReplacer(replacements...).Replace(format)
}
