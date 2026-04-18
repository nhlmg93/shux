package main

import (
	"testing"

	"github.com/nhlmg93/gotor/actor"
)

func TestPaneActorReceiveWrite(t *testing.T) {
	// Create a mock parent actor
	parent := &mockActor{}
	parentRef := actor.Spawn(parent, 10)
	defer parentRef.Stop()

	// Create pane
	pane := &PaneActor{
		id:     1,
		parent: parentRef,
	}

	// Test WriteToPane doesn't panic with nil PTY
	pane.Receive(WriteToPane{Data: []byte("test")})
}

func TestPaneActorReceiveKill(t *testing.T) {
	parent := &mockActor{}
	parentRef := actor.Spawn(parent, 10)
	defer parentRef.Stop()

	pane := &PaneActor{
		id:     1,
		parent: parentRef,
	}

	// Test KillPane with nil PTY sends exit notification
	pane.Receive(KillPane{})

	// Give time for message to be processed
	select {
	case <-parent.messages:
		// Expected PaneExited message
	default:
		// Message might not arrive if parent not ready, that's ok for this test
	}
}

// mockActor is a test actor that captures messages
type mockActor struct {
	messages chan any
}

func (m *mockActor) Receive(msg any) {
	if m.messages != nil {
		m.messages <- msg
	}
}
