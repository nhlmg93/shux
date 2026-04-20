package shux

import (
	"testing"
	"time"
)

func TestWindowCreateAndSwitchPanes(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)
	pane1 := requirePane(t, sessionRef, super)

	win.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/true"})

	var switched bool
	pollFor(200*time.Millisecond, func() bool {
		win.Send(SwitchToPane{Index: 1})
		result := <-sessionRef.Ask(GetActivePane{})
		if result == nil {
			return false
		}
		if result.(*PaneRef) != pane1 {
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
	requirePane(t, sessionRef, super)

	win.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	pollFor(200*time.Millisecond, func() bool { return <-win.Ask(GetActivePane{}) != nil })

	win.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	pollFor(200*time.Millisecond, func() bool { return <-win.Ask(GetActivePane{}) != nil })

	win.Send(SwitchToPane{Index: 2})
	var pane3 *PaneRef
	if !pollFor(100*time.Millisecond, func() bool {
		result := <-sessionRef.Ask(GetActivePane{})
		if result == nil {
			return false
		}
		pane3 = result.(*PaneRef)
		return pane3 != nil
	}) {
		t.Fatal("Expected to get pane 3")
	}
	pane3.Send(KillPane{})

	pollFor(200*time.Millisecond, func() bool { return <-sessionRef.Ask(GetActivePane{}) != nil })

	win.Send(SwitchToPane{Index: 0})
	var pane1Again bool
	pollFor(100*time.Millisecond, func() bool {
		pane1Again = <-sessionRef.Ask(GetActivePane{}) != nil
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
	requirePane(t, sessionRef, super)

	win.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	pollFor(200*time.Millisecond, func() bool { return <-win.Ask(GetActivePane{}) != nil })

	win.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	pollFor(200*time.Millisecond, func() bool { return <-win.Ask(GetActivePane{}) != nil })

	win.Send(SwitchToPane{Index: 1})
	var pane2 *PaneRef
	if !pollFor(100*time.Millisecond, func() bool {
		result := <-sessionRef.Ask(GetActivePane{})
		if result == nil {
			return false
		}
		pane2 = result.(*PaneRef)
		return pane2 != nil
	}) {
		t.Fatal("Expected to get pane 2")
	}
	pane2.Send(KillPane{})

	pollFor(200*time.Millisecond, func() bool { return <-sessionRef.Ask(GetActivePane{}) != nil })

	if survivor := <-sessionRef.Ask(GetActivePane{}); survivor == nil {
		t.Error("Expected window to have switched to another pane after killing active middle pane")
	}
}

func TestWindowSwitchToInvalidPane(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)
	pane := requirePane(t, sessionRef, super)

	win.Send(SwitchToPane{Index: 99})

	var stillOnPane1 bool
	pollFor(50*time.Millisecond, func() bool {
		result := <-sessionRef.Ask(GetActivePane{})
		stillOnPane1 = result != nil && result.(*PaneRef) == pane
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

	result := <-sessionRef.Ask(GetPaneContent{})
	if result == nil {
		t.Fatal("Expected pane content after resize")
	}
	if result.(*PaneContent) == nil {
		t.Fatal("Result should be a *PaneContent")
	}
}

func TestWindowBroadcastResize(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)

	for i := 0; i < 2; i++ {
		win.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
		pollFor(100*time.Millisecond, func() bool { return <-win.Ask(GetActivePane{}) != nil })
	}

	var initialSizes []int
	for i := 0; i < 3; i++ {
		win.Send(SwitchToPane{Index: i})
		pollFor(50*time.Millisecond, func() bool {
			result := <-sessionRef.Ask(GetPaneContent{})
			if result == nil {
				return false
			}
			content := result.(*PaneContent)
			initialSizes = append(initialSizes, len(content.Lines))
			return len(content.Lines) > 0
		})
	}

	sessionRef.Send(ResizeMsg{Rows: 30, Cols: 100})
	super.waitContentUpdated(200 * time.Millisecond)

	allResized := true
	for i := 0; i < 3; i++ {
		win.Send(SwitchToPane{Index: i})
		pollFor(50*time.Millisecond, func() bool {
			result := <-sessionRef.Ask(GetPaneContent{})
			if result == nil {
				return false
			}
			content := result.(*PaneContent)
			return len(content.Lines) == 30
		})

		result := <-sessionRef.Ask(GetPaneContent{})
		if result == nil || len(result.(*PaneContent).Lines) != 30 {
			allResized = false
		}
	}

	if !allResized {
		t.Error("Expected all panes to be resized")
	}
}
