package main

import (
	"testing"
	"time"

	"github.com/nhlmg93/gotor/actor"
)

// Test creating and switching panes
func TestIntegrationCreateAndSwitchPanes(t *testing.T) {
	supervisor := NewSupervisorActor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{})

	// Wait for initial window/pane to be created
	time.Sleep(50 * time.Millisecond)

	// Get active pane
	reply := sessionRef.Ask(GetActivePane{})
	paneRef := <-reply
	if paneRef == nil {
		t.Fatal("Expected active pane after creating window")
	}

	// Get active window to create another pane
	winReply := sessionRef.Ask(GetActiveWindow{})
	winRef := <-winReply
	if winRef == nil {
		t.Fatal("Expected active window")
	}

	// Create second pane
	winRef.(*actor.Ref).Send(CreatePane{Cmd: "/bin/true", Args: []string{}})
	time.Sleep(50 * time.Millisecond)

	// Switch to pane 2
	winRef.(*actor.Ref).Send(SwitchToPane{Index: 1})
	time.Sleep(10 * time.Millisecond)

	// Verify we can get the switched pane
	reply2 := sessionRef.Ask(GetActivePane{})
	paneRef2 := <-reply2
	if paneRef2 == nil {
		t.Error("Expected to get pane after switching")
	}
}

// Test creating and switching windows
func TestIntegrationCreateAndSwitchWindows(t *testing.T) {
	supervisor := NewSupervisorActor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{})
	time.Sleep(50 * time.Millisecond)

	// Get first window
	reply1 := sessionRef.Ask(GetActiveWindow{})
	win1 := <-reply1
	if win1 == nil {
		t.Fatal("Expected first window")
	}

	// Create second window
	sessionRef.Send(CreateWindow{})
	time.Sleep(50 * time.Millisecond)

	// Switch to next window
	sessionRef.Send(SwitchWindow{Delta: 1})
	time.Sleep(10 * time.Millisecond)

	// Get second window
	reply2 := sessionRef.Ask(GetActiveWindow{})
	win2 := <-reply2
	if win2 == nil {
		t.Fatal("Expected second window after switching")
	}

	// Should be different windows
	if win1 == win2 {
		t.Error("Expected different window after switching")
	}
}

// Test killing last pane closes window and session
func TestIntegrationKillLastPane(t *testing.T) {
	supervisor := NewSupervisorActor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{})
	time.Sleep(100 * time.Millisecond)

	// Get active pane and kill it
	reply := sessionRef.Ask(GetActivePane{})
	paneRef := <-reply
	if paneRef == nil {
		t.Fatal("Expected active pane")
	}

	paneRef.(*actor.Ref).Send(KillPane{})

	// Wait for pane to exit and session to signal empty
	select {
	case <-supervisor.quitChan:
		// Expected - session should signal empty when last pane killed
	case <-time.After(500 * time.Millisecond):
		t.Error("Expected supervisor to receive SessionEmpty after killing last pane")
	}
}

// Test window navigation wraps around
func TestIntegrationWindowNavigationWrap(t *testing.T) {
	supervisor := NewSupervisorActor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)

	// Create two windows
	sessionRef.Send(CreateWindow{})
	sessionRef.Send(CreateWindow{})
	time.Sleep(100 * time.Millisecond)

	// Get first window
	reply1 := sessionRef.Ask(GetActiveWindow{})
	win1 := <-reply1

	// Switch next twice should wrap back to first
	sessionRef.Send(SwitchWindow{Delta: 1})
	time.Sleep(10 * time.Millisecond)
	sessionRef.Send(SwitchWindow{Delta: 1})
	time.Sleep(10 * time.Millisecond)

	reply2 := sessionRef.Ask(GetActiveWindow{})
	win2 := <-reply2

	if win1 != win2 {
		t.Error("Expected to wrap back to first window after switching past end")
	}

	// Switch prev should go to last window (wrap backward)
	sessionRef.Send(SwitchWindow{Delta: -1})
	time.Sleep(10 * time.Millisecond)

	reply3 := sessionRef.Ask(GetActiveWindow{})
	win3 := <-reply3

	if win3 == win1 {
		t.Error("Expected to wrap to last window when switching prev from first")
	}
}

// Test switching to non-existent pane doesn't panic
func TestIntegrationSwitchToInvalidPane(t *testing.T) {
	supervisor := NewSupervisorActor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{})
	time.Sleep(50 * time.Millisecond)

	winReply := sessionRef.Ask(GetActiveWindow{})
	winRef := <-winReply

	// Try switching to pane index 99 (doesn't exist)
	winRef.(*actor.Ref).Send(SwitchToPane{Index: 99})
	time.Sleep(10 * time.Millisecond)

	// Should still have valid active pane
	reply := sessionRef.Ask(GetActivePane{})
	paneRef := <-reply
	if paneRef == nil {
		t.Error("Expected to still have active pane after invalid switch")
	}
}
