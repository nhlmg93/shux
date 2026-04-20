package gomux

import (
	"testing"
	"time"

	"github.com/nhlmg93/gotor/actor"
)

func TestWindowCreateAndSwitchPanes(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)
	pane1 := requirePane(t, sessionRef, super)

	// Create second pane
	win.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/true"})
	
	// Poll until we have a different pane available
	var switched bool
	pollFor(200*time.Millisecond, func() bool {
		win.Send(SwitchToPane{Index: 1})
		reply := sessionRef.Ask(GetActivePane{})
		result := <-reply
		if result == nil {
			return false
		}
		if result.(*actor.Ref) != pane1 {
			switched = true
			return true
		}
		return false
	})
	
	if !switched {
		t.Error("Expected to get different pane after creating and switching")
	}
}

func TestWindowKillNonActivePane(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)
	requirePane(t, sessionRef, super) // pane 1

	// Create 2 more panes (3 total)
	win.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	pollFor(200*time.Millisecond, func() bool {
		reply := win.Ask(GetActivePane{})
		result := <-reply
		return result != nil
	})
	
	win.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	pollFor(200*time.Millisecond, func() bool {
		reply := win.Ask(GetActivePane{})
		result := <-reply
		return result != nil
	})

	// Switch to pane 3 and kill it
	win.Send(SwitchToPane{Index: 2})
	var pane3 *actor.Ref
	if !pollFor(100*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetActivePane{})
		result := <-reply
		if result == nil {
			return false
		}
		pane3 = result.(*actor.Ref)
		return pane3 != nil
	}) {
		t.Fatal("Expected to get pane 3")
	}
	pane3.Send(KillPane{})

	pollFor(200*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetActivePane{})
		result := <-reply
		return result != nil
	})

	// Switch back to pane 1 and verify it still exists
	win.Send(SwitchToPane{Index: 0})
	var pane1Again bool
	pollFor(100*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetActivePane{})
		result := <-reply
		pane1Again = result != nil
		return pane1Again
	})
	if !pane1Again {
		t.Error("Expected pane 1 to still exist after killing pane 3")
	}
}

func TestWindowKillActiveMiddlePane(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)
	requirePane(t, sessionRef, super) // pane 1

	// Create 2 more panes
	win.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	pollFor(200*time.Millisecond, func() bool {
		reply := win.Ask(GetActivePane{})
		result := <-reply
		return result != nil
	})
	
	win.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	pollFor(200*time.Millisecond, func() bool {
		reply := win.Ask(GetActivePane{})
		result := <-reply
		return result != nil
	})

	// Switch to middle pane
	win.Send(SwitchToPane{Index: 1})
	var pane2 *actor.Ref
	if !pollFor(100*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetActivePane{})
		result := <-reply
		if result == nil {
			return false
		}
		pane2 = result.(*actor.Ref)
		return pane2 != nil
	}) {
		t.Fatal("Expected to get pane 2")
	}
	pane2.Send(KillPane{})

	pollFor(200*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetActivePane{})
		result := <-reply
		return result != nil
	})

	// Window should have auto-switched to another pane
	reply := sessionRef.Ask(GetActivePane{})
	survivor := <-reply
	if survivor == nil {
		t.Error("Expected window to have switched to another pane after killing active middle pane")
	}
}

func TestWindowSwitchToInvalidPane(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)
	pane := requirePane(t, sessionRef, super)

	// Try to switch to non-existent pane index
	win.Send(SwitchToPane{Index: 99})
	
	// Quick poll to let switch propagate
	var stillOnPane1 bool
	pollFor(50*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetActivePane{})
		result := <-reply
		stillOnPane1 = result != nil && result.(*actor.Ref) == pane
		return stillOnPane1
	})
	
	if !stillOnPane1 {
		t.Error("Expected to still be on original pane after invalid switch")
	}
}

func TestWindowResizePropagation(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	requirePane(t, sessionRef, super)

	sessionRef.Send(ResizeMsg{Rows: 30, Cols: 100})
	
	super.waitContentUpdated(200 * time.Millisecond)

	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected pane content after resize")
	}

	content := result.(*PaneContent)
	if content == nil {
		t.Fatal("Result should be a *PaneContent")
	}
}
