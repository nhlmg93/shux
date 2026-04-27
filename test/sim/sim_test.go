package sim

import (
	"context"
	"reflect"
	"strings"
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
	simClientID  = protocol.ClientID("sim-client")
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
		SessionID: simSessionID,
		WindowID:  simWindowID,
		Revision:  1,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: simPaneID, Col: 0, Row: 0, Cols: 80, Rows: 24},
		},
	})

	cancel()
	time.Sleep(50 * time.Millisecond)
}

func assertSimEvent(t *testing.T, events <-chan protocol.Event, want protocol.Event) {
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
		Meta:         protocol.CommandMeta{ClientID: simClientID, RequestID: 1},
		SessionID:    simSessionID,
		WindowID:     simWindowID,
		TargetPaneID: simPaneID,
		Direction:    protocol.SplitVertical,
	}); err != nil {
		t.Fatal(err)
	}
	assertSimEvent(t, events, protocol.EventPaneCreated{WindowID: simWindowID, PaneID: simPane2ID})
	assertSimEvent(t, events, protocol.EventWindowLayoutChanged{
		SessionID: simSessionID,
		WindowID:  simWindowID,
		Revision:  2,
		Cols:      80,
		Rows:      24,
		Panes: []protocol.EventLayoutPane{
			{PaneID: simPaneID, Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: simPane2ID, Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	assertSimEvent(t, events, protocol.EventPaneSplitCompleted{
		ClientID:     simClientID,
		RequestID:    1,
		SessionID:    simSessionID,
		WindowID:     simWindowID,
		TargetPaneID: simPaneID,
		NewPaneID:    simPane2ID,
		Revision:     2,
	})
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestSim_shellPTYInputOutputAndResize(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 64)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: "sim-pty", Sink: events}); err != nil {
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
	waitForPaneScreen(t, events, simPaneID, func(e protocol.EventPaneScreenChanged) bool {
		return e.Cols == 78 && e.Rows == 22
	})

	if err := ref.Send(ctx, protocol.CommandPanePaste{
		SessionID: simSessionID,
		WindowID:  simWindowID,
		PaneID:    simPaneID,
		Data:      []byte("printf shux-pty-ok\\n\n"),
	}); err != nil {
		t.Fatal(err)
	}
	waitForPaneScreen(t, events, simPaneID, func(e protocol.EventPaneScreenChanged) bool {
		return screenContains(e, "shux-pty-ok")
	})

	if err := ref.Send(ctx, protocol.CommandWindowResize{SessionID: simSessionID, WindowID: simWindowID, Cols: 100, Rows: 30}); err != nil {
		t.Fatal(err)
	}
	waitForPaneScreen(t, events, simPaneID, func(e protocol.EventPaneScreenChanged) bool {
		return e.Cols == 98 && e.Rows == 28
	})
}

func waitForPaneScreen(t *testing.T, events <-chan protocol.Event, paneID protocol.PaneID, match func(protocol.EventPaneScreenChanged) bool) protocol.EventPaneScreenChanged {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-events:
			screen, ok := event.(protocol.EventPaneScreenChanged)
			if !ok || screen.PaneID != paneID {
				continue
			}
			if match(screen) {
				return screen
			}
		case <-deadline:
			t.Fatalf("timed out waiting for pane screen %s", paneID)
		}
	}
}

func screenContains(screen protocol.EventPaneScreenChanged, needle string) bool {
	for _, line := range screen.Lines {
		if strings.Contains(line.Text, needle) {
			return true
		}
	}
	return false
}

func drain4(t *testing.T, ch <-chan protocol.Event) {
	t.Helper()
	seen := 0
	deadline := time.After(time.Second)
	for seen < 4 {
		select {
		case event := <-ch:
			if _, skip := event.(protocol.EventPaneScreenChanged); skip {
				continue
			}
			seen++
		case <-deadline:
			t.Fatalf("drain4: short read at %d", seen)
		}
	}
}
