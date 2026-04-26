package sim

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/mitchellh/go-libghostty"
	"shux/internal/hub"
	"shux/internal/protocol"
	"shux/internal/shux"
	"shux/internal/supervisor"
)

// IDs line up with first session / window / pane in supervisor counters (see integration).
const (
	simSessionID = protocol.SessionID("s-1")
	simWindowID  = protocol.WindowID("w-1")
	simPaneID    = protocol.PaneID("p-1")
	simPane2ID   = protocol.PaneID("p-2")
)

func TestShux_bootstrapsDefaultSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	app, err := shux.NewShux()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	if err := app.BootstrapDefaultSession(ctx); err != nil {
		t.Fatal(err)
	}
	if app.DefaultSessionID != protocol.SessionID("s-1") || app.DefaultWindowID != protocol.WindowID("w-1") || app.DefaultPaneID != protocol.PaneID("p-1") {
		t.Fatalf("ids = %q %q %q", app.DefaultSessionID, app.DefaultWindowID, app.DefaultPaneID)
	}
}

// TestTestBed_LibghosttyVT checks that the sim test bed (CGO, Ghostty lib-vt on PKG_CONFIG_PATH)
// is wired correctly. It does not cover full shux behavior; see test/e2e.
func TestTestBed_LibghosttyVT(t *testing.T) {
	term, err := libghostty.NewTerminal(libghostty.WithSize(80, 24))
	if err != nil {
		t.Fatal(err)
	}
	if term == nil {
		t.Fatal("NewTerminal: expected non-nil *Terminal")
	}
	defer term.Close()
}

// TestSim_emits_default_window_layout_on_pane_create checks the same layout publication
// as test/integration, under the test-sim (CGO) target.
func TestSim_emits_default_window_layout_on_pane_create(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 4)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: "sim-layout", Sink: events}); err != nil {
		t.Fatal(err)
	}

	ref := supervisor.StartWithHub(ctx, &eref)
	if err := ref.Send(ctx, protocol.CommandCreateSession{}); err != nil {
		t.Fatal(err)
	}
	if err := ref.Send(ctx, protocol.CommandCreateWindow{SessionID: simSessionID}); err != nil {
		t.Fatal(err)
	}
	if err := ref.Send(ctx, protocol.CommandCreatePane{SessionID: simSessionID, WindowID: simWindowID}); err != nil {
		t.Fatal(err)
	}

	assertSimEvent(t, events, protocol.EventSessionCreated{SessionID: simSessionID})
	assertSimEvent(t, events, protocol.EventWindowCreated{SessionID: simSessionID, WindowID: simWindowID})
	assertSimEvent(t, events, protocol.EventPaneCreated{WindowID: simWindowID, PaneID: simPaneID})
	assertSimEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID:  simSessionID,
		WindowID:   simWindowID,
		Cols:       80,
		Rows:       24,
		ActivePane: simPaneID,
		Panes: []protocol.EventLayoutPane{
			{PaneID: simPaneID, Col: 0, Row: 0, Cols: 80, Rows: 24},
		},
	})

	cancel()
	time.Sleep(50 * time.Millisecond)
}

func assertSimEvent(t *testing.T, events <-chan protocol.Event, want protocol.Event) {
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

// TestSim_split_and_resize matches integration coverage on the sim (CGO) path.
func TestSim_split_and_resize(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 12)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: "sim-split", Sink: events}); err != nil {
		t.Fatal(err)
	}

	ref := supervisor.StartWithHub(ctx, &eref)
	if err := ref.Send(ctx, protocol.CommandCreateSession{}); err != nil {
		t.Fatal(err)
	}
	if err := ref.Send(ctx, protocol.CommandCreateWindow{SessionID: simSessionID}); err != nil {
		t.Fatal(err)
	}
	if err := ref.Send(ctx, protocol.CommandCreatePane{SessionID: simSessionID, WindowID: simWindowID}); err != nil {
		t.Fatal(err)
	}
	drain4(t, events)
	if err := ref.Send(ctx, protocol.CommandPaneSplit{
		SessionID: simSessionID,
		WindowID:  simWindowID,
		Direction: protocol.SplitVertical,
	}); err != nil {
		t.Fatal(err)
	}
	assertSimEvent(t, events, protocol.EventPaneCreated{WindowID: simWindowID, PaneID: simPane2ID})
	assertSimEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID:  simSessionID,
		WindowID:   simWindowID,
		Cols:       80,
		Rows:       24,
		ActivePane: simPane2ID,
		Panes: []protocol.EventLayoutPane{
			{PaneID: simPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: simPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func drain4(t *testing.T, ch <-chan protocol.Event) {
	t.Helper()
	for i := 0; i < 4; i++ {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("drain4: short read at %d", i)
		}
	}
}
