package gomux

import (
	"testing"

	"github.com/nhlmg93/gotor/actor"
)

func TestSessionActorCreate(t *testing.T) {
	s := NewSessionActor(1)
	if s.id != 1 {
		t.Errorf("Expected session ID 1, got %d", s.id)
	}
	if len(s.windows) != 0 {
		t.Error("Expected new session to have no windows")
	}
}

func TestSessionActorHandleWindowEmpty(t *testing.T) {
	s := NewSessionActor(1)

	// Simulate having a window (use a mock ref)
	mockRef := actor.Spawn(&mockActor{}, 10)
	defer mockRef.Stop()

	s.windows[1] = mockRef
	s.active = 1

	// Handle window empty
	s.Receive(WindowEmpty{ID: 1})

	// Window should be removed
	if _, exists := s.windows[1]; exists {
		t.Error("Expected window to be removed after empty")
	}
}

func TestSessionActorSwitchWindow(t *testing.T) {
	s := NewSessionActor(1)

	// Simulate having multiple windows
	s.windows[1] = nil
	s.windows[2] = nil
	s.windows[3] = nil
	s.active = 1

	// Switch to next
	s.Receive(SwitchWindow{Delta: 1})
	if s.active == 1 {
		t.Error("Expected to switch to a different window")
	}

	// Switch to prev
	current := s.active
	s.Receive(SwitchWindow{Delta: -1})
	if s.active == current {
		t.Error("Expected to switch to previous window")
	}
}
