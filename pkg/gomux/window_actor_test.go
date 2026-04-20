package gomux

import (
	"testing"

	"github.com/nhlmg93/gotor/actor"
)

func TestWindowActorCreate(t *testing.T) {
	w := NewWindowActor(1)
	if w.id != 1 {
		t.Errorf("Expected window ID 1, got %d", w.id)
	}
	if len(w.terms) != 0 {
		t.Error("Expected new window to have no terms")
	}
}

func TestWindowActorHandleTermExited(t *testing.T) {
	w := NewWindowActor(1)

	// Simulate having a term (use a mock ref)
	mockRef := actor.Spawn(&mockActor{}, 10)
	defer mockRef.Stop()

	w.terms[1] = mockRef
	w.active = 1

	// Handle term exited
	w.Receive(TermExited{ID: 1})

	// Term should be removed
	if _, exists := w.terms[1]; exists {
		t.Error("Expected term to be removed after exit")
	}
}
