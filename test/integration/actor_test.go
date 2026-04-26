package integration_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"shux/internal/hub"
	"shux/internal/protocol"
	"shux/internal/supervisor"
)

// ID suffixes match supervisor/session/window counters: first create in each actor is "1".
const (
	initSessionID = protocol.SessionID("s-1")
	initWindowID  = protocol.WindowID("w-1")
	initPaneID    = protocol.PaneID("p-1")
	initPane2ID   = protocol.PaneID("p-2")
)

func TestStart_acceptsCommandNoop(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ref := supervisor.Start(ctx)
	if err := ref.Send(ctx, protocol.CommandNoop{}); err != nil {
		t.Fatal(err)
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
}

// Exercises supervisor -> session -> window -> pane with IDs routed on commands.
// Requires libghostty (CGO) because pane is linked; see Makefile.
func TestCreate_session_window_pane(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ref := supervisor.Start(ctx)
	if err := ref.Send(ctx, protocol.CommandCreateSession{}); err != nil {
		t.Fatal(err)
	}
	if err := ref.Send(ctx, protocol.CommandCreateWindow{SessionID: initSessionID}); err != nil {
		t.Fatal(err)
	}
	if err := ref.Send(ctx, protocol.CommandCreatePane{SessionID: initSessionID, WindowID: initWindowID}); err != nil {
		t.Fatal(err)
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
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
	if err := ref.Send(ctx, protocol.CommandCreateSession{}); err != nil {
		t.Fatal(err)
	}
	if err := ref.Send(ctx, protocol.CommandCreateWindow{SessionID: initSessionID}); err != nil {
		t.Fatal(err)
	}
	if err := ref.Send(ctx, protocol.CommandCreatePane{SessionID: initSessionID, WindowID: initWindowID}); err != nil {
		t.Fatal(err)
	}

	assertEvent(t, events, protocol.EventSessionCreated{SessionID: initSessionID})
	assertEvent(t, events, protocol.EventWindowCreated{SessionID: initSessionID, WindowID: initWindowID})
	assertEvent(t, events, protocol.EventPaneCreated{WindowID: initWindowID, PaneID: initPaneID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID:  initSessionID,
		WindowID:   initWindowID,
		Cols:       80,
		Rows:       24,
		ActivePane: initPaneID,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 80, Rows: 24},
		},
	})

	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestHub_pane_split_and_window_resize(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 12)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: "test-split", Sink: events}); err != nil {
		t.Fatal(err)
	}

	ref := supervisor.StartWithHub(ctx, &eref)
	if err := ref.Send(ctx, protocol.CommandCreateSession{}); err != nil {
		t.Fatal(err)
	}
	if err := ref.Send(ctx, protocol.CommandCreateWindow{SessionID: initSessionID}); err != nil {
		t.Fatal(err)
	}
	if err := ref.Send(ctx, protocol.CommandCreatePane{SessionID: initSessionID, WindowID: initWindowID}); err != nil {
		t.Fatal(err)
	}

	assertEvent(t, events, protocol.EventSessionCreated{SessionID: initSessionID})
	assertEvent(t, events, protocol.EventWindowCreated{SessionID: initSessionID, WindowID: initWindowID})
	assertEvent(t, events, protocol.EventPaneCreated{WindowID: initWindowID, PaneID: initPaneID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID:  initSessionID,
		WindowID:   initWindowID,
		Cols:       80,
		Rows:       24,
		ActivePane: initPaneID,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 80, Rows: 24},
		},
	})

	if err := ref.Send(ctx, protocol.CommandPaneSplit{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Direction: protocol.SplitVertical,
	}); err != nil {
		t.Fatal(err)
	}

	assertEvent(t, events, protocol.EventPaneCreated{WindowID: initWindowID, PaneID: initPane2ID})
	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID:  initSessionID,
		WindowID:   initWindowID,
		Cols:       80,
		Rows:       24,
		ActivePane: initPane2ID,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: initPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})

	if err := ref.Send(ctx, protocol.CommandWindowResize{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Cols:      100,
		Rows:      30,
	}); err != nil {
		t.Fatal(err)
	}

	assertEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID:  initSessionID,
		WindowID:   initWindowID,
		Cols:       100,
		Rows:       30,
		ActivePane: initPane2ID,
		Panes: []protocol.EventLayoutPane{
			{PaneID: initPaneID, Col: 0, Row: 0, Cols: 50, Rows: 30},
			{PaneID: initPane2ID, Col: 50, Row: 0, Cols: 50, Rows: 30},
		},
	})

	cancel()
	time.Sleep(50 * time.Millisecond)
}

func assertEvent(t *testing.T, events <-chan protocol.Event, want protocol.Event) {
	t.Helper()
	select {
	case got := <-events:
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("event = %#v, want %#v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %#v", want)
	}
}
