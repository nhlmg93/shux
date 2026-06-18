package integration_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"shux/internal/hub"
	"shux/internal/protocol"
	"shux/internal/supervisor"
	"shux/test/testutil"
)

type commandSender interface {
	Send(context.Context, protocol.Command) error
}

// ID suffixes match supervisor/session/window counters: first create in each actor is "1".
const (
	initSessionID   = protocol.SessionID("s-1")
	initSessionName = "session-1"
	initWindowID    = protocol.WindowID("w-1")
	initPaneID      = protocol.PaneID("p-1")
	initPane2ID     = protocol.PaneID("p-2")
	initPane3ID     = protocol.PaneID("p-3")
	initPane4ID     = protocol.PaneID("p-4")
	testClientID    = protocol.ClientID("test-client")
	testClient2ID   = protocol.ClientID("test-client-2")
)

func TestStart_acceptsCommandNoop(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ref := supervisor.Start(ctx)
	testutil.MustSend(t, ctx, ref, protocol.CommandNoop{})

	drainCancel(cancel)
}

// Exercises supervisor -> session -> window -> pane with IDs routed on commands.
// Requires libghostty (CGO) because pane is linked; see Makefile.
func TestCreate_session_window_pane(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ref := supervisor.Start(ctx)
	testutil.MustSend(t, ctx, ref, protocol.CommandCreateSession{})
	testutil.MustSend(t, ctx, ref, protocol.CommandCreateWindow{SessionID: initSessionID})
	testutil.MustSend(t, ctx, ref, protocol.CommandCreatePane{SessionID: initSessionID, WindowID: initWindowID})

	drainCancel(cancel)
}

func TestHub_fansOutLifecycleEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 4)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: "test-client", Sink: events}); err != nil {
		t.Fatal(err)
	}

	ref := supervisor.StartWithHub(ctx, &eref)
	testutil.MustSend(t, ctx, ref, protocol.CommandCreateSession{})
	testutil.MustSend(t, ctx, ref, protocol.CommandCreateWindow{SessionID: initSessionID})
	testutil.MustSend(t, ctx, ref, protocol.CommandCreatePane{SessionID: initSessionID, WindowID: initWindowID})

	assertEvent(t, events, protocol.EventSessionCreated{SessionID: initSessionID, Name: initSessionName})
	assertEvent(t, events, protocol.EventWindowCreated{SessionID: initSessionID, WindowID: initWindowID})
	assertEvent(t, events, protocol.EventSessionWindowsChanged{SessionID: initSessionID, Revision: 1, Windows: []protocol.WindowID{initWindowID}})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPaneID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  1,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 80, Rows: 24},
		},
	})

	drainCancel(cancel)
}

func TestHub_pane_split_and_window_resize(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 16)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: "test-split", Sink: events}); err != nil {
		t.Fatal(err)
	}

	ref := supervisor.StartWithHub(ctx, &eref)
	bootstrapWindow(t, ctx, ref, events)

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	})

	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  2,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    1,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane2ID,
		Revision:     2,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandWindowResize{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Cols:      100,
		Rows:      30,
	})

	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  3,
		Cols:      100,
		Rows:      30,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 50, Rows: 30},
			{PaneID: initPane2ID, Col: 50, Row: 0, Cols: 50, Rows: 30},
		},
	})

	drainCancel(cancel)
}

func TestHub_pane_zoom_toggle_and_switch_target(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 24)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: "test-pane-zoom", Sink: events}); err != nil {
		t.Fatal(err)
	}

	ref := supervisor.StartWithHub(ctx, &eref)
	bootstrapWindow(t, ctx, ref, events)
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  2,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    1,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane2ID,
		Revision:     2,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneZoomToggle{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		Revision:     3,
		Cols:         80,
		Rows:         24,
		Panes:        []protocol.EventLayoutPane{{PaneID: initPaneID, Col: 0, Row: 0, Cols: 80, Rows: 24}},
		ZoomedPaneID: initPaneID,
		SavedPanes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneZoomToggle{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPane2ID,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		Revision:     4,
		Cols:         80,
		Rows:         24,
		Panes:        []protocol.EventLayoutPane{{PaneID: initPane2ID, Col: 0, Row: 0, Cols: 80, Rows: 24}},
		ZoomedPaneID: initPane2ID,
		SavedPanes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneZoomToggle{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPane2ID,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  5,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
}

func TestHub_serializes_repeated_targeted_splits(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 32)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: "test-concurrent-split", Sink: events}); err != nil {
		t.Fatal(err)
	}
	ref := supervisor.StartWithHub(ctx, &eref)
	bootstrapWindow(t, ctx, ref, events)

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  2,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    1,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane2ID,
		Revision:     2,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClient2ID, RequestID: 2},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitHorizontal,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane3ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  3,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane3ID, Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClient2ID,
		RequestID:    2,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane3ID,
		Revision:     3,
	})
}

func TestHub_rejects_missing_target_split_without_crashing(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ref, events := startWindowWithEvents(t, ctx, "test-reject-split")
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 7},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: protocol.PaneID("missing"),
		Direction:    protocol.SplitVertical,
	})
	assertEvent(t, events, protocol.EventCommandRejected{
		ClientID:  testClientID,
		RequestID: 7,
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Command:   "pane-split",
		Reason:    "target pane missing",
	})
}

func TestHub_pane_resize_delta_updates_layout(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ref, events := startWindowWithEvents(t, ctx, "test-pane-resize")
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  2,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    1,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane2ID,
		Revision:     2,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneResizeDelta{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 2},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Edge:         protocol.PaneResizeEdgeRight,
		Delta:        5,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  3,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 45, Rows: 24},
			{PaneID: initPane2ID, Col: 45, Row: 0, Cols: 35, Rows: 24},
		},
	})
}

func TestHub_pane_focus_grid_navigation_and_target(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ref, events := startWindowWithEvents(t, ctx, "test-pane-focus-grid")

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  2,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    1,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane2ID,
		Revision:     2,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 2},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitHorizontal,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane3ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  3,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane3ID, Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    2,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane3ID,
		Revision:     3,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 3},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPane2ID,
		Direction:    protocol.SplitHorizontal,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane4ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  4,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane3ID, Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane4ID, Col: 40, Row: 12, Cols: 40, Rows: 12},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    3,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPane2ID,
		NewPaneID:    initPane4ID,
		Revision:     4,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneFocus{
		Meta:          protocol.CommandMeta{ClientID: testClientID, RequestID: 10},
		SessionID:     initSessionID,
		WindowID:      initWindowID,
		CurrentPaneID: initPaneID,
		Direction:     protocol.PaneFocusRight,
	})
	assertEvent(t, events, protocol.EventPaneFocusResolved{
		ClientID:  testClientID,
		RequestID: 10,
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPane2ID,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneFocus{
		Meta:          protocol.CommandMeta{ClientID: testClientID, RequestID: 11},
		SessionID:     initSessionID,
		WindowID:      initWindowID,
		CurrentPaneID: initPaneID,
		Direction:     protocol.PaneFocusDown,
	})
	assertEvent(t, events, protocol.EventPaneFocusResolved{
		ClientID:  testClientID,
		RequestID: 11,
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPane3ID,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneFocus{
		Meta:          protocol.CommandMeta{ClientID: testClientID, RequestID: 12},
		SessionID:     initSessionID,
		WindowID:      initWindowID,
		CurrentPaneID: initPane4ID,
		Direction:     protocol.PaneFocusLeft,
	})
	assertEvent(t, events, protocol.EventPaneFocusResolved{
		ClientID:  testClientID,
		RequestID: 12,
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPane3ID,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneFocus{
		Meta:          protocol.CommandMeta{ClientID: testClientID, RequestID: 13},
		SessionID:     initSessionID,
		WindowID:      initWindowID,
		CurrentPaneID: initPane4ID,
		Direction:     protocol.PaneFocusUp,
	})
	assertEvent(t, events, protocol.EventPaneFocusResolved{
		ClientID:  testClientID,
		RequestID: 13,
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPane2ID,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneFocus{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 14},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPane4ID,
	})
	assertEvent(t, events, protocol.EventPaneFocusResolved{
		ClientID:  testClientID,
		RequestID: 14,
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPane4ID,
	})
}

func TestHub_pane_resize_delta_rejected_when_min_size_hit(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ref, events := startWindowWithEvents(t, ctx, "test-pane-resize-reject")
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  2,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    1,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane2ID,
		Revision:     2,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneResizeDelta{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 2},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Edge:         protocol.PaneResizeEdgeRight,
		Delta:        80,
	})
	assertEvent(t, events, protocol.EventCommandRejected{
		ClientID:  testClientID,
		RequestID: 2,
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Command:   "pane-resize-delta",
		Reason:    "minimum pane size reached",
	})
}

func TestHub_swapPaneByDirection_allDirections(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ref, events := startWindowWithEvents(t, ctx, "test-swap-directions")

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 2, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID: testClientID, RequestID: 1, SessionID: initSessionID, WindowID: initWindowID,
		TargetPaneID: initPaneID, NewPaneID: initPane2ID, Revision: 2,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 2},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitHorizontal,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane3ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 3, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane3ID, Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID: testClientID, RequestID: 2, SessionID: initSessionID, WindowID: initWindowID,
		TargetPaneID: initPaneID, NewPaneID: initPane3ID, Revision: 3,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 3},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPane2ID,
		Direction:    protocol.SplitHorizontal,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane4ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 4, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane3ID, Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane4ID, Col: 40, Row: 12, Cols: 40, Rows: 12},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID: testClientID, RequestID: 3, SessionID: initSessionID, WindowID: initWindowID,
		TargetPaneID: initPane2ID, NewPaneID: initPane4ID, Revision: 4,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSwap{
		Meta:      protocol.CommandMeta{ClientID: testClientID, RequestID: 4},
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
		Direction: protocol.PaneDirectionRight,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 5, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPane2ID, Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane3ID, Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: initPaneID, Col: 40, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane4ID, Col: 40, Row: 12, Cols: 40, Rows: 12},
		},
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSwap{
		Meta:      protocol.CommandMeta{ClientID: testClientID, RequestID: 5},
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
		Direction: protocol.PaneDirectionDown,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 6, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPane2ID, Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane3ID, Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: initPane4ID, Col: 40, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPaneID, Col: 40, Row: 12, Cols: 40, Rows: 12},
		},
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSwap{
		Meta:      protocol.CommandMeta{ClientID: testClientID, RequestID: 6},
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
		Direction: protocol.PaneDirectionLeft,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 7, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPane2ID, Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPaneID, Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: initPane4ID, Col: 40, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane3ID, Col: 40, Row: 12, Cols: 40, Rows: 12},
		},
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSwap{
		Meta:      protocol.CommandMeta{ClientID: testClientID, RequestID: 7},
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
		Direction: protocol.PaneDirectionUp,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 8, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane2ID, Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: initPane4ID, Col: 40, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane3ID, Col: 40, Row: 12, Cols: 40, Rows: 12},
		},
	})
}

func TestHub_selectLayoutPresets_and_swapPaneByDirection(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ref, events := startWindowWithEvents(t, ctx, "test-layout-presets")

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 2, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID: testClientID, RequestID: 1, SessionID: initSessionID, WindowID: initWindowID,
		TargetPaneID: initPaneID, NewPaneID: initPane2ID, Revision: 2,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 2},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitHorizontal,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane3ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 3, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane3ID, Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID: testClientID, RequestID: 2, SessionID: initSessionID, WindowID: initWindowID,
		TargetPaneID: initPaneID, NewPaneID: initPane3ID, Revision: 3,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 3},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPane2ID,
		Direction:    protocol.SplitHorizontal,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane4ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 4, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane3ID, Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane4ID, Col: 40, Row: 12, Cols: 40, Rows: 12},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID: testClientID, RequestID: 3, SessionID: initSessionID, WindowID: initWindowID,
		TargetPaneID: initPane2ID, NewPaneID: initPane4ID, Revision: 4,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandWindowSelectLayout{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 4},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		ActivePaneID: initPaneID,
		Preset:       protocol.LayoutPresetEvenVertical,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 5, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 20, Rows: 24},
			{PaneID: initPane3ID, Col: 20, Row: 0, Cols: 20, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 20, Rows: 24},
			{PaneID: initPane4ID, Col: 60, Row: 0, Cols: 20, Rows: 24},
		},
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandWindowSelectLayout{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 5},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		ActivePaneID: initPane2ID,
		Preset:       protocol.LayoutPresetEvenHorizontal,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 6, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 80, Rows: 6},
			{PaneID: initPane3ID, Col: 0, Row: 6, Cols: 80, Rows: 6},
			{PaneID: initPane2ID, Col: 0, Row: 12, Cols: 80, Rows: 6},
			{PaneID: initPane4ID, Col: 0, Row: 18, Cols: 80, Rows: 6},
		},
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandWindowSelectLayout{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 6},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		ActivePaneID: initPane2ID,
		Preset:       protocol.LayoutPresetMainHorizontal,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 7, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPane2ID, Col: 0, Row: 0, Cols: 80, Rows: 12},
			{PaneID: initPaneID, Col: 0, Row: 12, Cols: 26, Rows: 12},
			{PaneID: initPane3ID, Col: 26, Row: 12, Cols: 27, Rows: 12},
			{PaneID: initPane4ID, Col: 53, Row: 12, Cols: 27, Rows: 12},
		},
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSwap{
		Meta:      protocol.CommandMeta{ClientID: testClientID, RequestID: 7},
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPane3ID,
		Direction: protocol.PaneDirectionLeft,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID, WindowID: initWindowID, Revision: 8, Cols: 80, Rows: 24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPane2ID, Col: 0, Row: 0, Cols: 80, Rows: 12},
			{PaneID: initPane3ID, Col: 0, Row: 12, Cols: 26, Rows: 12},
			{PaneID: initPaneID, Col: 26, Row: 12, Cols: 27, Rows: 12},
			{PaneID: initPane4ID, Col: 53, Row: 12, Cols: 27, Rows: 12},
		},
	})
}

func TestHub_rejectsSwapPaneWithoutNeighbor(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ref, events := startWindowWithEvents(t, ctx, "test-swap-reject")
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSwap{
		Meta:      protocol.CommandMeta{ClientID: testClientID, RequestID: 11},
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
		Direction: protocol.PaneDirectionLeft,
	})
	assertEvent(t, events, protocol.EventCommandRejected{
		ClientID:  testClientID,
		RequestID: 11,
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Command:   "swap-pane",
		Reason:    "no neighbor in direction",
	})
}

func TestHub_splitWhileZoomed_restoresThenSplits(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ref, events := startWindowWithEvents(t, ctx, "test-split-while-zoomed")
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  2,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    1,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane2ID,
		Revision:     2,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneZoomToggle{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		Revision:     3,
		Cols:         80,
		Rows:         24,
		ZoomedPaneID: initPaneID,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 80, Rows: 24},
		},
		SavedPanes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 2},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitHorizontal,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  4,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane3ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  5,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: initPane3ID, Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    2,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane3ID,
		Revision:     5,
	})
}

func TestHub_closeWhileZoomed_restoresThenCloses(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ref, events := startWindowWithEvents(t, ctx, "test-close-while-zoomed")
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  2,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    1,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane2ID,
		Revision:     2,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneZoomToggle{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		Revision:     3,
		Cols:         80,
		Rows:         24,
		ZoomedPaneID: initPaneID,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 80, Rows: 24},
		},
		SavedPanes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneClose{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  4,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneClosed{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPaneID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  5,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPane2ID, Col: 0, Row: 0, Cols: 80, Rows: 24},
		},
	})
}

func TestHub_fansOutPaneScreenChanged(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 2)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: "test-screen", Sink: events}); err != nil {
		t.Fatal(err)
	}
	want := protocol.EventPaneScreenChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
		Revision:  1,
		Cols:      2,
		Rows:      1,
		Lines: []protocol.EventPaneScreenLine{{
			Text: "ok",
			Cells: []protocol.EventPaneScreenCell{
				{Text: "o", Bold: true},
				{Text: "k"},
			},
		}},
	}
	if err := eref.Send(ctx, want); err != nil {
		t.Fatal(err)
	}
	assertEvent(t, events, want)
}

func TestHub_windowClose_updatesSessionWindowList(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ref, events := startWindowWithEvents(t, ctx, "test-window-close")
	if err := ref.Send(ctx, protocol.CommandPaneClose{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
	}); err != nil {
		t.Fatal(err)
	}

	assertEvent(t, events, protocol.EventPaneClosed{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPaneID})
	assertCloseRelatedEvents(t, events)
}

func TestPaneInputCommandsRouteThroughActorTree(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ref, _ := startWindowWithEvents(t, ctx, "test-pane-input")
	commands := []protocol.Command{
		protocol.CommandPaneKey{
			SessionID: initSessionID,
			WindowID:  initWindowID,
			PaneID:    initPaneID,
			Action:    protocol.KeyActionPress,
			Key:       "x",
			Text:      "x",
		},
		protocol.CommandPanePaste{
			SessionID: initSessionID,
			WindowID:  initWindowID,
			PaneID:    initPaneID,
			Data:      []byte("echo shux\n"),
		},
		protocol.CommandPaneMouse{
			SessionID: initSessionID,
			WindowID:  initWindowID,
			PaneID:    initPaneID,
			Action:    protocol.MouseActionPress,
			Button:    protocol.MouseButtonLeft,
			CellCol:   1,
			CellRow:   1,
		},
	}
	for _, cmd := range commands {
		testutil.MustSend(t, ctx, ref, cmd)
	}
}

func TestHub_paneMove_breakAndJoin(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ref, events := startWindowWithEvents(t, ctx, "test-pane-move")
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 20},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  2,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    20,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane2ID,
		Revision:     2,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneMove{
		SessionID:      initSessionID,
		SourceWindowID: initWindowID,
		PaneID:         initPane2ID,
	})
	assertEvent(t, events, protocol.EventWindowCreated{SessionID: initSessionID, WindowID: protocol.WindowID("w-2")})
	assertEvent(t, events, protocol.EventSessionWindowsChanged{
		SessionID: initSessionID,
		Revision:  2,
		Windows:   []protocol.WindowID{initWindowID, "w-2"},
	})
	assertEvent(t, events, protocol.EventPaneClosed{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  3,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 80, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  "w-2",
		Revision:  1,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPane2ID, Col: 0, Row: 0, Cols: 80, Rows: 24},
		},
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneMove{
		SessionID:      initSessionID,
		SourceWindowID: "w-2",
		TargetWindowID: initWindowID,
		PaneID:         initPane2ID,
	})
	assertEvent(t, events, protocol.EventPaneClosed{SessionID: initSessionID, WindowID: "w-2", PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowClosed{SessionID: initSessionID, WindowID: "w-2"})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  4,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventSessionWindowsChanged{
		SessionID: initSessionID,
		Revision:  3,
		Windows:   []protocol.WindowID{initWindowID},
	})
}

func TestHub_windowAndPaneRenameEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	ref, events := startWindowWithEvents(t, ctx, "test-rename")
	testutil.MustSend(t, ctx, ref, protocol.CommandWindowRename{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Name:      "dev",
	})
	assertEvent(t, events, protocol.EventWindowRenamed{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Name:      "dev",
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandPaneRename{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
		Name:      "shell",
	})
	assertEvent(t, events, protocol.EventPaneRenamed{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
		Name:      "shell",
	})
}

func TestWindowSyncPanes_fansOutPaneKeyInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 64)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: "test-sync-panes", Sink: events}); err != nil {
		t.Fatal(err)
	}

	ref := supervisor.StartWithHub(ctx, &eref)
	bootstrapWindow(t, ctx, ref, events)
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  2,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     testClientID,
		RequestID:    1,
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		NewPaneID:    initPane2ID,
		Revision:     2,
	})

	testutil.MustSend(t, ctx, ref, protocol.CommandWindowToggleSyncPanes{
		SessionID: initSessionID,
		WindowID:  initWindowID,
	})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  3,
		Cols:      80,
		Rows:      24,
		SyncPanes: true,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})

	sendPaneKeyText(t, ctx, ref, initPaneID, "echo sync-panes-15")
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneKey{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		PaneID:    initPaneID,
		Action:    protocol.KeyActionPress,
		Key:       "enter",
	})

	waitForPaneScreenText(t, events, initPaneID, "sync-panes-15")
	waitForPaneScreenText(t, events, initPane2ID, "sync-panes-15")
}

func startWindowWithEvents(t *testing.T, ctx context.Context, clientID protocol.ClientID) (commandSender, <-chan protocol.Event) {
	t.Helper()
	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 16)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: clientID, Sink: events}); err != nil {
		t.Fatal(err)
	}
	ref := supervisor.StartWithHub(ctx, &eref)
	bootstrapWindow(t, ctx, ref, events)
	return ref, events
}

func bootstrapWindow(t *testing.T, ctx context.Context, ref commandSender, events <-chan protocol.Event) {
	t.Helper()
	testutil.MustSend(t, ctx, ref, protocol.CommandCreateSession{})
	testutil.MustSend(t, ctx, ref, protocol.CommandCreateWindow{SessionID: initSessionID})
	testutil.MustSend(t, ctx, ref, protocol.CommandCreatePane{SessionID: initSessionID, WindowID: initWindowID})
	assertEvent(t, events, protocol.EventSessionCreated{SessionID: initSessionID, Name: initSessionName})
	assertEvent(t, events, protocol.EventWindowCreated{SessionID: initSessionID, WindowID: initWindowID})
	assertEvent(t, events, protocol.EventSessionWindowsChanged{SessionID: initSessionID, Revision: 1, Windows: []protocol.WindowID{initWindowID}})
	assertEvent(t, events, protocol.EventPaneCreated{SessionID: initSessionID, WindowID: initWindowID, PaneID: initPaneID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Revision:  1,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 80, Rows: 24},
		},
	})
}

func drainCancel(cancel context.CancelFunc) {
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func assertEvent(t *testing.T, events <-chan protocol.Event, want protocol.Event) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case got := <-events:
			if _, skip := got.(protocol.EventPaneScreenChanged); skip {
				if _, wantScreen := want.(protocol.EventPaneScreenChanged); !wantScreen {
					continue
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("event = %#v, want %#v", got, want)
			}
			return
		case <-deadline:
			t.Fatalf("timed out waiting for %#v", want)
		}
	}
}

func assertCloseRelatedEvents(t *testing.T, events <-chan protocol.Event) {
	t.Helper()

	got1 := nextEvent(t, events)
	got2 := nextEvent(t, events)
	if hasWindowClosed(got1) && hasSessionWindowPruned(got2) {
		return
	}
	if hasWindowClosed(got2) && hasSessionWindowPruned(got1) {
		return
	}
	t.Fatalf("close events = [%#v, %#v], want EventWindowClosed and zero-window EventSessionWindowsChanged", got1, got2)
}

func sendPaneKeyText(t *testing.T, ctx context.Context, ref commandSender, paneID protocol.PaneID, text string) {
	t.Helper()
	for _, r := range text {
		testutil.MustSend(t, ctx, ref, protocol.CommandPaneKey{
			SessionID: initSessionID,
			WindowID:  initWindowID,
			PaneID:    paneID,
			Action:    protocol.KeyActionPress,
			Key:       string(r),
			Text:      string(r),
		})
	}
}

func waitForPaneScreenText(t *testing.T, events <-chan protocol.Event, paneID protocol.PaneID, want string) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case evt := <-events:
			screen, ok := evt.(protocol.EventPaneScreenChanged)
			if !ok || screen.PaneID != paneID {
				continue
			}
			for _, line := range screen.Lines {
				if strings.Contains(line.Text, want) {
					return
				}
			}
		case <-deadline:
			t.Fatalf("timed out waiting for pane %s to contain %q", paneID, want)
		}
	}
}

func nextEvent(t *testing.T, events <-chan protocol.Event) protocol.Event {
	t.Helper()
	select {
	case evt := <-events:
		return evt
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
		return nil
	}
}

func hasWindowClosed(evt protocol.Event) bool {
	closed, ok := evt.(protocol.EventWindowClosed)
	return ok && closed.SessionID == initSessionID && closed.WindowID == initWindowID
}

func hasSessionWindowPruned(evt protocol.Event) bool {
	changed, ok := evt.(protocol.EventSessionWindowsChanged)
	return ok && changed.SessionID == initSessionID && changed.Revision == 2 && len(changed.Windows) == 0
}
