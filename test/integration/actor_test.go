package integration_test

import (
	"context"
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

type eventChanSink chan protocol.Event

func (s eventChanSink) DeliverEvent(ctx context.Context, e protocol.Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s <- e:
		return nil
	}
}

func TestHub_fansOutLifecycleEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eref := hub.Start(ctx)
	events := make(eventChanSink, 3)
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

	cancel()
	time.Sleep(50 * time.Millisecond)
}

func assertEvent(t *testing.T, events <-chan protocol.Event, want protocol.Event) {
	t.Helper()
	select {
	case got := <-events:
		if got != want {
			t.Fatalf("event = %#v, want %#v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %#v", want)
	}
}
