package shux

import (
	"context"

	"shux/internal/protocol"
)

const autocmdBridgeClientID = protocol.ClientID("autocmd-bridge")

type autocmdBridge struct {
	app *Shux
}

func newAutocmdBridge(app *Shux) *autocmdBridge {
	return &autocmdBridge{app: app}
}

func (b *autocmdBridge) DeliverEvent(ctx context.Context, e protocol.Event) error {
	if b == nil || b.app == nil || b.app.Autocmds == nil {
		return nil
	}
	switch e := e.(type) {
	case protocol.EventPaneCreated:
		b.app.Autocmds.Emit(ctx, EventPaneCreated, map[string]any{
			"session_id": string(e.SessionID),
			"window_id":  string(e.WindowID),
			"pane_id":    string(e.PaneID),
		})
	case protocol.EventPaneClosed:
		b.app.Autocmds.Emit(ctx, EventPaneClosed, map[string]any{
			"window_id": string(e.WindowID),
			"pane_id":   string(e.PaneID),
		})
	case protocol.EventPaneRenamed:
		b.app.Autocmds.Emit(ctx, EventPaneRenamed, map[string]any{
			"session_id": string(e.SessionID),
			"window_id":  string(e.WindowID),
			"pane_id":    string(e.PaneID),
			"name":       e.Name,
		})
	case protocol.EventWindowCreated:
		b.app.Autocmds.Emit(ctx, EventWindowCreated, map[string]any{
			"session_id": string(e.SessionID),
			"window_id":  string(e.WindowID),
		})
	case protocol.EventWindowRenamed:
		b.app.Autocmds.Emit(ctx, EventWindowRenamed, map[string]any{
			"session_id": string(e.SessionID),
			"window_id":  string(e.WindowID),
			"name":       e.Name,
		})
	case protocol.EventWindowClosed:
		b.app.Autocmds.Emit(ctx, EventWindowClosed, map[string]any{
			"session_id": string(e.SessionID),
			"window_id":  string(e.WindowID),
		})
	case protocol.EventWindowLayoutChanged:
		b.app.Autocmds.Emit(ctx, EventWindowLayoutChanged, map[string]any{
			"session_id": string(e.SessionID),
			"window_id":  string(e.WindowID),
			"revision":   e.Revision,
			"panes":      len(e.Panes),
		})
	}
	return nil
}
