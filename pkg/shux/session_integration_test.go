package shux

import (
	"testing"
	"time"
)

func TestSessionCreateAndSwitchWindows(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win1 := requireWindow(t, sessionRef, super)

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	_ = requireWindow(t, sessionRef, super)

	sessionRef.Send(SwitchWindow{Delta: 1})
	var win2 *WindowRef
	if !pollFor(100*time.Millisecond, func() bool {
		result := <-sessionRef.Ask(GetActiveWindow{})
		if result == nil {
			return false
		}
		win2 = result.(*WindowRef)
		return win2 != win1
	}) {
		t.Fatal("Expected to switch to window 2")
	}

	sessionRef.Send(SwitchWindow{Delta: -1})
	var backToWin1 bool
	pollFor(100*time.Millisecond, func() bool {
		result := <-sessionRef.Ask(GetActiveWindow{})
		if result == nil {
			return false
		}
		backToWin1 = result.(*WindowRef) == win1
		return backToWin1
	})
	if !backToWin1 {
		t.Error("Expected to be back at window 1")
	}
}

func TestSessionWindowNavigationWrap(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	for i := 0; i < 3; i++ {
		sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
		requireWindow(t, sessionRef, super)
	}
	win1 := requireWindow(t, sessionRef, super)

	sessionRef.Send(SwitchWindow{Delta: 1})
	var win2 *WindowRef
	if !pollFor(100*time.Millisecond, func() bool {
		result := <-sessionRef.Ask(GetActiveWindow{})
		if result == nil {
			return false
		}
		win2 = result.(*WindowRef)
		return win2 != win1
	}) {
		t.Fatal("Expected to switch from window 1")
	}

	sessionRef.Send(SwitchWindow{Delta: 2})
	var wrapped bool
	pollFor(100*time.Millisecond, func() bool {
		result := <-sessionRef.Ask(GetActiveWindow{})
		if result == nil {
			return false
		}
		wrapped = result.(*WindowRef) == win1
		return wrapped
	})
	if !wrapped {
		t.Error("Expected wrap forward to window 1")
	}
}

func TestSessionKillLastPane(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	pane.Send(KillPane{})

	empty := pollFor(5*time.Second, func() bool {
		select {
		case <-super.sessionEmpty:
			return true
		default:
		}
		if <-sessionRef.Ask(GetActivePane{}) == nil {
			return true
		}
		return <-sessionRef.Ask(GetActiveWindow{}) == nil
	})
	if !empty {
		t.Error("timeout waiting for SessionEmpty after killing last pane")
	}
}

func TestSessionMultipleWindowsWithPanes(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win1 := requireWindow(t, sessionRef, super)
	win1.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	pollFor(200*time.Millisecond, func() bool { return <-win1.Ask(GetActivePane{}) != nil })

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win2 := requireWindow(t, sessionRef, super)
	win2.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	pollFor(200*time.Millisecond, func() bool { return <-win2.Ask(GetActivePane{}) != nil })

	win2Pane := requirePane(t, sessionRef, super)

	sessionRef.Send(SwitchWindow{Delta: -1})
	var win1Pane *PaneRef
	if !pollFor(100*time.Millisecond, func() bool {
		result := <-sessionRef.Ask(GetActivePane{})
		if result == nil {
			return false
		}
		win1Pane = result.(*PaneRef)
		return win1Pane != win2Pane
	}) {
		t.Fatal("Expected different pane in window 1")
	}

	if win1Pane == win2Pane {
		t.Error("Expected different pane refs for different windows")
	}
}

func TestSessionWindowPersistence(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	_ = requireWindow(t, sessionRef, super)

	pane1 := requirePane(t, sessionRef, super)
	pane1.Send(WriteToPane{Data: []byte("window1data")})
	super.waitContentUpdated(200 * time.Millisecond)

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	_ = requireWindow(t, sessionRef, super)

	sessionRef.Send(SwitchWindow{Delta: -1})
	pollFor(50*time.Millisecond, func() bool { return <-sessionRef.Ask(GetActiveWindow{}) != nil })

	sessionRef.Send(SwitchWindow{Delta: 1})
	var backToWin2 bool
	pollFor(50*time.Millisecond, func() bool {
		backToWin2 = <-sessionRef.Ask(GetActiveWindow{}) != nil
		return backToWin2
	})
	if !backToWin2 {
		t.Fatal("Expected window 2 after switching back")
	}
}

func TestSessionGetPaneContentBeforeWindowCreated(t *testing.T) {
	sessionRef, _, cleanup := setupSession(t)
	defer cleanup()

	if result := <-sessionRef.Ask(GetPaneContent{}); result != nil {
		t.Error("Expected nil content when no window exists")
	}
}
