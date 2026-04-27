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

type commandSender interface {
	Send(context.Context, protocol.Command) error
}

// ID suffixes match supervisor/session/window counters: first create in each actor is "1".
const (
	initSessionID = protocol.SessionID("s-1")
	initWindowID  = protocol.WindowID("w-1")
	initPaneID    = protocol.PaneID("p-1")
	initPane2ID   = protocol.PaneID("p-2")
	initPane3ID   = protocol.PaneID("p-3")
	testClientID  = protocol.ClientID("test-client")
	testClient2ID = protocol.ClientID("test-client-2")
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

	cancel()
	time.Sleep(50 * time.Millisecond)
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

	if err := ref.Send(ctx, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	}); err != nil {
		t.Fatal(err)
	}

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

	if err := ref.Send(ctx, protocol.CommandWindowResize{
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Cols:      100,
		Rows:      30,
	}); err != nil {
		t.Fatal(err)
	}

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

	cancel()
	time.Sleep(50 * time.Millisecond)
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

	if err := ref.Send(ctx, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 1},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitVertical,
	}); err != nil {
		t.Fatal(err)
	}
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

	if err := ref.Send(ctx, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClient2ID, RequestID: 2},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: initPaneID,
		Direction:    protocol.SplitHorizontal,
	}); err != nil {
		t.Fatal(err)
	}
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
	if err := ref.Send(ctx, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: testClientID, RequestID: 7},
		SessionID:    initSessionID,
		WindowID:     initWindowID,
		TargetPaneID: protocol.PaneID("missing"),
		Direction:    protocol.SplitVertical,
	}); err != nil {
		t.Fatal(err)
	}
	assertEvent(t, events, protocol.EventCommandRejected{
		ClientID:  testClientID,
		RequestID: 7,
		SessionID: initSessionID,
		WindowID:  initWindowID,
		Command:   "pane-split",
		Reason:    "target pane missing",
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
		if err := ref.Send(ctx, cmd); err != nil {
			t.Fatalf("send %T: %v", cmd, err)
		}
	}
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
