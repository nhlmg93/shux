package sim

import (
	"context"
	"testing"
	"time"

	"github.com/mitchellh/go-libghostty"
	"shux/internal/hub"
	"shux/internal/protocol"
	"shux/internal/supervisor"
	"shux/internal/shux"
)

// IDs line up with first session / window / pane in supervisor counters (see integration).
const (
	simSessionID = protocol.SessionID("s-1")
	simWindowID  = protocol.WindowID("w-1")
	simPaneID    = protocol.PaneID("p-1")
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
	if app.SessionID != protocol.SessionID("s-1") || app.WindowID != protocol.WindowID("w-1") || app.PaneID != protocol.PaneID("p-1") {
		t.Fatalf("ids = %q %q %q", app.SessionID, app.WindowID, app.PaneID)
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
	events := make(eventChanSink, 4)
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
		SessionID: simSessionID,
		WindowID:  simWindowID,
		Cols:      80,
		Rows:      24,
	})

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

func assertSimEvent(t *testing.T, events <-chan protocol.Event, want protocol.Event) {
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
