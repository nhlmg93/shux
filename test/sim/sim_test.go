package sim

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mitchellh/go-libghostty"
	"shux/internal/hub"
	"shux/internal/protocol"
	"shux/internal/shux"
	"shux/internal/supervisor"
)

const (
	simSessionID = protocol.SessionID("s-1")
	simWindowID  = protocol.WindowID("w-1")
	simPaneID    = protocol.PaneID("p-1")
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

// TestTestBed_LibghosttyVT proves the sim container has the real Ghostty VT
// library wired through CGO/PKG_CONFIG_PATH. The deterministic sim below runs
// in that same Docker test bed via `make test-sim-docker` / `make test-docker`.
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

func TestSim_deterministicSessionWindowPaneFuzz(t *testing.T) {
	for _, seed := range []int64{0x5eed5eed, 0x51a7e001, 0xc105ed} {
		t.Run(fmt.Sprintf("seed-%x", seed), func(t *testing.T) {
			runDeterministicSimFuzz(t, seed)
		})
	}
}

func runDeterministicSimFuzz(t *testing.T, seed int64) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 1024)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: "sim-fuzz", Sink: events}); err != nil {
		t.Fatal(err)
	}
	rec := newSimRecorder(t, events)
	defer rec.stop()

	ref := supervisor.StartWithHub(ctx, &eref)
	sendSim(t, ctx, ref, protocol.CommandCreateSession{})
	sendSim(t, ctx, ref, protocol.CommandCreateWindow{
		Meta:      protocol.CommandMeta{ClientID: simClientID, RequestID: 1},
		SessionID: simSessionID,
		Cols:      80,
		Rows:      24,
		AutoPane:  true,
	})
	rec.waitUntil(func(s simState) bool {
		w := s.windows[simWindowID]
		return s.sessionCreated && w != nil && len(w.panes) == 1 && w.panes[simPaneID]
	})

	rng := rand.New(rand.NewSource(seed))
	var req protocol.RequestID = 1
	for step := 0; step < 96; step++ {
		snap := rec.snapshot()
		wid, ok := snap.randomWindow(rng)
		if !ok {
			t.Fatalf("step %d: no windows", step)
		}
		w := snap.windows[wid]
		switch roll := rng.Intn(100); {
		case roll < 18 && len(snap.windowOrder) < 6:
			req++
			sendSim(t, ctx, ref, protocol.CommandCreateWindow{
				Meta:      protocol.CommandMeta{ClientID: simClientID, RequestID: req},
				SessionID: simSessionID,
				Cols:      uint16(40 + rng.Intn(80)),
				Rows:      uint16(12 + rng.Intn(36)),
				AutoPane:  true,
			})
		case roll < 48:
			pid, ok := w.randomPane(rng)
			if !ok {
				continue
			}
			req++
			dir := protocol.SplitDirection(rng.Intn(2))
			sendSim(t, ctx, ref, protocol.CommandPaneSplit{
				Meta:         protocol.CommandMeta{ClientID: simClientID, RequestID: req},
				SessionID:    simSessionID,
				WindowID:     wid,
				TargetPaneID: pid,
				Direction:    dir,
			})
		case roll < 70:
			sendSim(t, ctx, ref, protocol.CommandWindowResize{
				SessionID: simSessionID,
				WindowID:  wid,
				Cols:      uint16(20 + rng.Intn(120)),
				Rows:      uint16(8 + rng.Intn(48)),
			})
		case roll < 84 && (len(w.panes) > 1 || len(snap.windowOrder) > 1):
			pid, ok := w.randomPane(rng)
			if !ok {
				continue
			}
			req++
			sendSim(t, ctx, ref, protocol.CommandPaneClose{
				Meta:      protocol.CommandMeta{ClientID: simClientID, RequestID: req},
				SessionID: simSessionID,
				WindowID:  wid,
				PaneID:    pid,
			})
		default:
			pid, ok := w.randomPane(rng)
			if !ok {
				continue
			}
			sendSim(t, ctx, ref, protocol.CommandPanePaste{
				SessionID: simSessionID,
				WindowID:  wid,
				PaneID:    pid,
				Data:      []byte("printf fuzz-step-" + strings.Repeat("x", step%7) + "\\n\n"),
			})
		}
		rec.waitQuiet()
		rec.snapshot().validate(t)
	}
}

func TestSim_shellPTYInputOutputAndResize(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()

	eref := hub.Start(ctx)
	events := make(protocol.EventChanAdapter, 64)
	if err := eref.Send(ctx, protocol.EventRegisterSubscriber{ClientID: "sim-pty", Sink: events}); err != nil {
		t.Fatal(err)
	}
	ref := supervisor.StartWithHub(ctx, &eref)
	sendSim(t, ctx, ref, protocol.CommandCreateSession{})
	sendSim(t, ctx, ref, protocol.CommandCreateWindow{SessionID: simSessionID})
	sendSim(t, ctx, ref, protocol.CommandCreatePane{SessionID: simSessionID, WindowID: simWindowID})
	waitForPaneScreen(t, events, simPaneID, func(e protocol.EventPaneScreenChanged) bool {
		return e.Cols == 78 && e.Rows == 22
	})

	sendSim(t, ctx, ref, protocol.CommandPanePaste{
		SessionID: simSessionID,
		WindowID:  simWindowID,
		PaneID:    simPaneID,
		Data:      []byte("printf shux-pty-ok\\n\n"),
	})
	waitForPaneScreen(t, events, simPaneID, func(e protocol.EventPaneScreenChanged) bool {
		return screenContains(e, "shux-pty-ok")
	})

	sendSim(t, ctx, ref, protocol.CommandWindowResize{SessionID: simSessionID, WindowID: simWindowID, Cols: 100, Rows: 30})
	waitForPaneScreen(t, events, simPaneID, func(e protocol.EventPaneScreenChanged) bool {
		return e.Cols == 98 && e.Rows == 28
	})
}

type commandSender interface {
	Send(context.Context, protocol.Command) error
}

func sendSim(t *testing.T, ctx context.Context, ref commandSender, cmd protocol.Command) {
	t.Helper()
	if err := ref.Send(ctx, cmd); err != nil {
		t.Fatalf("send %T: %v", cmd, err)
	}
}

type simRecorder struct {
	t      *testing.T
	events <-chan protocol.Event
	done   chan struct{}
	mu     sync.Mutex
	state  simState
}

type simState struct {
	sessionCreated bool
	windowOrder    []protocol.WindowID
	windows        map[protocol.WindowID]*simWindow
	closed         map[protocol.WindowID]bool
}

type simWindow struct {
	panes  map[protocol.PaneID]bool
	layout protocol.EventWindowLayoutChanged
}

func newSimRecorder(t *testing.T, events <-chan protocol.Event) *simRecorder {
	r := &simRecorder{
		t:      t,
		events: events,
		done:   make(chan struct{}),
		state:  simState{windows: make(map[protocol.WindowID]*simWindow), closed: make(map[protocol.WindowID]bool)},
	}
	go r.run()
	return r
}

func (r *simRecorder) run() {
	for {
		select {
		case <-r.done:
			return
		case e := <-r.events:
			r.mu.Lock()
			r.state.apply(e)
			r.mu.Unlock()
		}
	}
}

func (r *simRecorder) stop() { close(r.done) }

func (r *simRecorder) snapshot() simState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state.clone()
}

func (r *simRecorder) waitUntil(match func(simState) bool) {
	r.t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if snap := r.snapshot(); match(snap) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	r.t.Fatalf("timed out waiting for sim state")
}

func (r *simRecorder) waitQuiet() {
	r.t.Helper()
	prev := -1
	stable := 0
	deadline := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(deadline) {
		n := r.snapshot().eventWeight()
		if n == prev {
			stable++
			if stable >= 3 {
				return
			}
		} else {
			prev = n
			stable = 0
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (s *simState) apply(e protocol.Event) {
	s.ensure()
	switch e := e.(type) {
	case protocol.EventSessionCreated:
		s.sessionCreated = true
	case protocol.EventSessionWindowsChanged:
		s.windowOrder = s.windowOrder[:0]
		for _, wid := range e.Windows {
			if !s.closed[wid] {
				s.windowOrder = append(s.windowOrder, wid)
			}
		}
	case protocol.EventWindowCreated:
		s.ensureWindow(e.WindowID)
		s.addWindowOrder(e.WindowID)
	case protocol.EventPaneCreated:
		w := s.ensureWindow(e.WindowID)
		w.panes[e.PaneID] = true
	case protocol.EventPaneClosed:
		w := s.ensureWindow(e.WindowID)
		delete(w.panes, e.PaneID)
	case protocol.EventWindowClosed:
		if s.closed == nil {
			s.closed = make(map[protocol.WindowID]bool)
		}
		s.closed[e.WindowID] = true
		delete(s.windows, e.WindowID)
		for i, wid := range s.windowOrder {
			if wid == e.WindowID {
				s.windowOrder = append(s.windowOrder[:i], s.windowOrder[i+1:]...)
				break
			}
		}
	case protocol.EventWindowLayoutChanged:
		w := s.ensureWindow(e.WindowID)
		w.layout = e
	}
}

func (s *simState) ensure() {
	if s.windows == nil {
		s.windows = make(map[protocol.WindowID]*simWindow)
	}
	if s.closed == nil {
		s.closed = make(map[protocol.WindowID]bool)
	}
}

func (s *simState) ensureWindow(id protocol.WindowID) *simWindow {
	s.ensure()
	w := s.windows[id]
	if w == nil {
		w = &simWindow{panes: make(map[protocol.PaneID]bool)}
		s.windows[id] = w
	}
	return w
}

func (s *simState) addWindowOrder(id protocol.WindowID) {
	for _, existing := range s.windowOrder {
		if existing == id {
			return
		}
	}
	s.windowOrder = append(s.windowOrder, id)
}

func (s simState) clone() simState {
	out := simState{sessionCreated: s.sessionCreated, windowOrder: append([]protocol.WindowID(nil), s.windowOrder...), windows: make(map[protocol.WindowID]*simWindow, len(s.windows)), closed: make(map[protocol.WindowID]bool, len(s.closed))}
	for wid, closed := range s.closed {
		out.closed[wid] = closed
	}
	for wid, w := range s.windows {
		cw := &simWindow{panes: make(map[protocol.PaneID]bool, len(w.panes)), layout: w.layout}
		for pid, ok := range w.panes {
			cw.panes[pid] = ok
		}
		out.windows[wid] = cw
	}
	return out
}

func (s simState) eventWeight() int {
	n := len(s.windowOrder)
	for _, w := range s.windows {
		n += len(w.panes) + int(w.layout.Revision)
	}
	return n
}

func (s simState) randomWindow(rng *rand.Rand) (protocol.WindowID, bool) {
	if len(s.windowOrder) == 0 {
		return "", false
	}
	return s.windowOrder[rng.Intn(len(s.windowOrder))], true
}

func (w *simWindow) randomPane(rng *rand.Rand) (protocol.PaneID, bool) {
	if w == nil || len(w.panes) == 0 {
		return "", false
	}
	idx := rng.Intn(len(w.panes))
	for pid := range w.panes {
		if idx == 0 {
			return pid, true
		}
		idx--
	}
	return "", false
}

func (s simState) validate(t *testing.T) {
	t.Helper()
	if !s.sessionCreated {
		t.Fatalf("session not created")
	}
	seenWindows := map[protocol.WindowID]bool{}
	for _, wid := range s.windowOrder {
		if seenWindows[wid] {
			t.Fatalf("duplicate window in order: %s", wid)
		}
		seenWindows[wid] = true
		if s.windows[wid] == nil {
			t.Fatalf("ordered missing window: %s", wid)
		}
	}
	for wid, w := range s.windows {
		if !seenWindows[wid] {
			t.Fatalf("window %s missing from order", wid)
		}
		validateLayout(t, wid, w)
	}
}

func validateLayout(t *testing.T, wid protocol.WindowID, w *simWindow) {
	t.Helper()
	if w.layout.Revision == 0 {
		return
	}
	if w.layout.Cols <= 0 || w.layout.Rows <= 0 {
		t.Fatalf("%s invalid layout size %dx%d", wid, w.layout.Cols, w.layout.Rows)
	}
	seen := map[protocol.PaneID]bool{}
	for _, p := range w.layout.Panes {
		if seen[p.PaneID] {
			t.Fatalf("%s duplicate layout pane %s", wid, p.PaneID)
		}
		seen[p.PaneID] = true
		if !w.panes[p.PaneID] {
			t.Fatalf("%s layout references unknown pane %s", wid, p.PaneID)
		}
		if p.Cols <= 0 || p.Rows <= 0 {
			t.Fatalf("%s pane %s invalid size %dx%d", wid, p.PaneID, p.Cols, p.Rows)
		}
		if p.Col < 0 || p.Row < 0 || p.Col+p.Cols > w.layout.Cols || p.Row+p.Rows > w.layout.Rows {
			t.Fatalf("%s pane %s out of bounds: %+v in %dx%d", wid, p.PaneID, p, w.layout.Cols, w.layout.Rows)
		}
	}
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
