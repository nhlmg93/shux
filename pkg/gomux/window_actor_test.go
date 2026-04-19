package gomux

import (
	"testing"

	"github.com/nhlmg93/gotor/actor"
)

func TestWindowActorCreate(t *testing.T) {
	parent := &mockActor{}
	parentRef := actor.Spawn(parent, 10)
	defer parentRef.Stop()

	w := NewWindowActor(1, parentRef)
	if w.id != 1 {
		t.Errorf("Expected window ID 1, got %d", w.id)
	}
	if len(w.terms) != 0 {
		t.Error("Expected new window to have no terms")
	}
}

func TestWindowActorHandleTermExited(t *testing.T) {
	parent := &mockActor{}
	parentRef := actor.Spawn(parent, 10)
	defer parentRef.Stop()

	w := NewWindowActor(1, parentRef)
	ref := actor.Spawn(w, 10)
	w.self = ref

	// Simulate having a term
	w.terms[1] = ref
	w.active = 1

	// Handle term exited
	w.Receive(TermExited{ID: 1})

	// Term should be removed
	if _, exists := w.terms[1]; exists {
		t.Error("Expected term to be removed after exit")
	}
}
