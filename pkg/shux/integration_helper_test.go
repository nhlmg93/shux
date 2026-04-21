package shux

import (
	"testing"
	"time"
)

// testSupervisor captures events via channels for synchronization.
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

func (s *testSupervisor) handle(msg any) {
	switch m := msg.(type) {
	case SessionEmpty:
		select {
		case s.sessionEmpty <- m.ID:
		default:
		}
	case PaneContentUpdated:
		select {
		case s.contentUpdated <- m.ID:
		default:
		}
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

func setupSession(t *testing.T) (*SessionRef, *testSupervisor, func()) {
	t.Helper()
	super := newTestSupervisor()
	logger := &StdLogger{Logger}
	sessionRef := StartSession(1, super.handle, logger)
	cleanup := func() {
		sessionRef.Shutdown()
	}
	return sessionRef, super, cleanup
}

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

func requirePane(t *testing.T, sessionRef *SessionRef, super *testSupervisor) *PaneRef {
	t.Helper()
	var paneRef any
	if !pollFor(500*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetActivePane{})
		paneRef = <-reply
		return paneRef != nil
	}) {
		t.Fatal("timeout waiting for pane creation")
	}
	return paneRef.(*PaneRef)
}

func requireWindow(t *testing.T, sessionRef *SessionRef, super *testSupervisor) *WindowRef {
	t.Helper()
	var winRef any
	if !pollFor(500*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetActiveWindow{})
		winRef = <-reply
		return winRef != nil
	}) {
		t.Fatal("timeout waiting for window creation")
	}
	return winRef.(*WindowRef)
}
