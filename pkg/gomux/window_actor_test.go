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
	if len(w.panes) != 0 {
		t.Error("Expected new window to have no panes")
	}
}

func TestWindowActorHandlePaneExited(t *testing.T) {
	parent := &mockActor{}
	parentRef := actor.Spawn(parent, 10)
	defer parentRef.Stop()

	w := NewWindowActor(1, parentRef)
	ref := actor.Spawn(w, 10)
	w.self = ref

	// Simulate having a pane
	w.panes[1] = ref
	w.active = 1

	// Handle pane exited
	w.Receive(PaneExited{ID: 1})

	// Pane should be removed
	if _, exists := w.panes[1]; exists {
		t.Error("Expected pane to be removed after exit")
	}
}
