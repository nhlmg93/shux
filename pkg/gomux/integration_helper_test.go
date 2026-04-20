package gomux

import (
	"testing"
	"time"

	"github.com/nhlmg93/gotor/actor"
)

// testSupervisor captures events via channels for synchronization
type testSupervisor struct {
	sessionEmpty   chan uint32
	contentUpdated chan uint32
}

func newTestSupervisor() *testSupervisor {
	return &testSupervisor{
		sessionEmpty:   make(chan uint32, 10),
		contentUpdated: make(chan uint32, 10),
	}
}

func (s *testSupervisor) Receive(msg any) {
	switch m := msg.(type) {
	case SessionEmpty:
		select { case s.sessionEmpty <- m.ID: default: }
	case PaneContentUpdated:
		select { case s.contentUpdated <- m.ID: default: }
	}
}

func (s *testSupervisor) waitSessionEmpty(timeout time.Duration) bool {
	select {
	case <-s.sessionEmpty:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (s *testSupervisor) waitContentUpdated(timeout time.Duration) bool {
	select {
	case <-s.contentUpdated:
		return true
	case <-time.After(timeout):
		return false
	}
}

// setupSession creates a test session with supervisor, returns session ref and cleanup
func setupSession(t *testing.T) (*actor.Ref, *testSupervisor, func()) {
	super := newTestSupervisor()
	superRef := actor.Spawn(super, 10)
	sessionRef := SpawnSession(1, superRef)

	cleanup := func() {
		superRef.Stop()
	}

	return sessionRef, super, cleanup
}

// pollFor waits for a condition with timeout (for initial setup)
func pollFor(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// requirePane polls for and returns an active pane reference
func requirePane(t *testing.T, sessionRef *actor.Ref, super *testSupervisor) *actor.Ref {
	t.Helper()
	// Poll until pane exists (actor spawned, not waiting for content)
	var paneRef any
	if !pollFor(500*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetActivePane{})
		paneRef = <-reply
		return paneRef != nil
	}) {
		t.Fatal("timeout waiting for pane creation")
	}
	return paneRef.(*actor.Ref)
}

// requirePaneInWindow polls for a pane in a specific window
func requirePaneInWindow(t *testing.T, winRef *actor.Ref, super *testSupervisor) *actor.Ref {
	t.Helper()
	var paneRef any
	if !pollFor(500*time.Millisecond, func() bool {
		reply := winRef.Ask(GetActivePane{})
		paneRef = <-reply
		return paneRef != nil
	}) {
		t.Fatal("timeout waiting for pane in window")
	}
	return paneRef.(*actor.Ref)
}

// requireWindow polls for and returns an active window reference  
func requireWindow(t *testing.T, sessionRef *actor.Ref, super *testSupervisor) *actor.Ref {
	t.Helper()
	// Poll until window exists
	var winRef any
	if !pollFor(500*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetActiveWindow{})
		winRef = <-reply
		return winRef != nil
	}) {
		t.Fatal("timeout waiting for window creation")
	}
	return winRef.(*actor.Ref)
}
