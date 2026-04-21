package shux

import (
	"strings"
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

	// With tiling, panes should all have usable height.
	for _, s := range initialSizes {
		if s <= 0 {
			t.Fatalf("Initial pane sizes should be positive: got %v", initialSizes)
		}
	}

	sessionRef.Send(ResizeMsg{Rows: 30, Cols: 100})
	super.waitContentUpdated(200 * time.Millisecond)

	// After resize with multiple panes, each pane gets a share of the space
	allHaveContent := true
	for i := 0; i < 3; i++ {
		win.Send(SwitchToPane{Index: i})
		pollFor(50*time.Millisecond, func() bool {
			result := <-sessionRef.Ask(GetPaneContent{})
			if result == nil {
				return false
			}
			content := result.(*PaneContent)
			return len(content.Lines) > 0
		})

		result := <-sessionRef.Ask(GetPaneContent{})
		if result == nil {
			allHaveContent = false
			continue
		}
		content := result.(*PaneContent)
		if len(content.Lines) == 0 {
			allHaveContent = false
		}
	}

	if !allHaveContent {
		t.Error("Expected all panes to have content after resize")
	}
}

func TestWindowSplitAndRender(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)

	pollFor(100*time.Millisecond, func() bool {
		result := <-win.Ask(GetActivePane{})
		return result != nil
	})

	win.Send(Split{Dir: SplitH})
	time.Sleep(50 * time.Millisecond)

	winDataResult := <-win.Ask(GetWindowSnapshotData{})
	if winDataResult == nil {
		t.Fatal("Expected window snapshot data")
	}
	winData := winDataResult.(WindowSnapshot)
	if len(winData.PaneOrder) != 2 {
		t.Fatalf("Expected 2 panes after split, got %d", len(winData.PaneOrder))
	}

	sessionRef.Send(ResizeMsg{Rows: 24, Cols: 80})
	super.waitContentUpdated(200 * time.Millisecond)

	viewResult := <-sessionRef.Ask(GetWindowView{})
	if viewResult == nil {
		t.Fatal("Expected window view after resize")
	}
	view, ok := viewResult.(WindowView)
	if !ok || view.Content == "" {
		t.Fatal("Window view should be non-empty")
	}
	if !strings.Contains(view.Content, "─") {
		t.Fatal("Expected horizontal divider in window view")
	}
}

func TestWindowNestedSplitCrossRender(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)

	pollFor(100*time.Millisecond, func() bool {
		result := <-win.Ask(GetActivePane{})
		return result != nil
	})

	// Start with left/right split.
	win.Send(Split{Dir: SplitV})
	time.Sleep(50 * time.Millisecond)

	// Split left pane top/bottom.
	win.Send(SwitchToPane{Index: 0})
	time.Sleep(20 * time.Millisecond)
	win.Send(Split{Dir: SplitH})
	time.Sleep(50 * time.Millisecond)

	// Split right pane top/bottom.
	win.Send(SwitchToPane{Index: 1})
	time.Sleep(20 * time.Millisecond)
	win.Send(Split{Dir: SplitH})
	time.Sleep(50 * time.Millisecond)

	sessionRef.Send(ResizeMsg{Rows: 24, Cols: 80})
	super.waitContentUpdated(200 * time.Millisecond)

	viewResult := <-sessionRef.Ask(GetWindowView{})
	if viewResult == nil {
		t.Fatal("Expected window view after nested splits")
	}
	view, ok := viewResult.(WindowView)
	if !ok || view.Content == "" {
		t.Fatal("Window view should be non-empty")
	}
	if !strings.Contains(view.Content, "│") {
		t.Fatal("Expected vertical divider in nested split view")
	}
	if !strings.Contains(view.Content, "─") {
		t.Fatal("Expected horizontal divider in nested split view")
	}
	if !strings.Contains(view.Content, "┼") {
		t.Fatal("Expected cross intersection in nested split view")
	}
}

func TestWindowNavigatePanesByDirection(t *testing.T) {
	sessionRef, _, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := <-sessionRef.Ask(GetActiveWindow{})
	if win == nil {
		t.Fatal("expected active window")
	}
	windowRef := win.(*WindowRef)

	// Build a 2x2 layout:
	// 1 | 2
	// 3 | 4
	windowRef.Send(Split{Dir: SplitV})
	time.Sleep(50 * time.Millisecond)
	windowRef.Send(SwitchToPane{Index: 0})
	time.Sleep(20 * time.Millisecond)
	windowRef.Send(Split{Dir: SplitH})
	time.Sleep(50 * time.Millisecond)
	windowRef.Send(SwitchToPane{Index: 1})
	time.Sleep(20 * time.Millisecond)
	windowRef.Send(Split{Dir: SplitH})
	time.Sleep(50 * time.Millisecond)
	windowRef.Send(ResizeMsg{Rows: 24, Cols: 80})
	time.Sleep(50 * time.Millisecond)

	assertActive := func(want uint32) {
		t.Helper()
		got := (<-windowRef.Ask(GetWindowSnapshotData{})).(WindowSnapshot)
		if got.ActivePane != want {
			t.Fatalf("expected active pane %d, got %d", want, got.ActivePane)
		}
	}

	windowRef.Send(SwitchToPane{Index: 0})
	time.Sleep(20 * time.Millisecond)
	assertActive(1)

	windowRef.Send(NavigatePane{Dir: PaneNavRight})
	time.Sleep(20 * time.Millisecond)
	assertActive(2)

	windowRef.Send(NavigatePane{Dir: PaneNavDown})
	time.Sleep(20 * time.Millisecond)
	assertActive(4)

	windowRef.Send(NavigatePane{Dir: PaneNavLeft})
	time.Sleep(20 * time.Millisecond)
	assertActive(3)

	windowRef.Send(NavigatePane{Dir: PaneNavUp})
	time.Sleep(20 * time.Millisecond)
	assertActive(1)
}

func TestWindowOneSidedSplitRendersTJunction(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)

	pollFor(100*time.Millisecond, func() bool {
		result := <-win.Ask(GetActivePane{})
		return result != nil
	})

	// Split left/right, then split only the left side top/bottom.
	win.Send(Split{Dir: SplitV})
	time.Sleep(50 * time.Millisecond)
	win.Send(SwitchToPane{Index: 0})
	time.Sleep(20 * time.Millisecond)
	win.Send(Split{Dir: SplitH})
	time.Sleep(50 * time.Millisecond)

	sessionRef.Send(ResizeMsg{Rows: 24, Cols: 80})
	super.waitContentUpdated(200 * time.Millisecond)

	viewResult := <-sessionRef.Ask(GetWindowView{})
	if viewResult == nil {
		t.Fatal("Expected window view after split")
	}
	view, ok := viewResult.(WindowView)
	if !ok || view.Content == "" {
		t.Fatal("Window view should be non-empty")
	}
	if !strings.Contains(view.Content, "┤") && !strings.Contains(view.Content, "├") {
		t.Fatal("Expected T-junction in one-sided split view")
	}
	if strings.Contains(view.Content, "┼") {
		t.Fatal("Did not expect full cross intersection in one-sided split view")
	}
}

func TestWindowClosePaneCollapsesSplit(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)

	pollFor(100*time.Millisecond, func() bool {
		result := <-win.Ask(GetActivePane{})
		return result != nil
	})

	// Build a one-sided split: left/right, then split only left top/bottom.
	win.Send(Split{Dir: SplitV})
	time.Sleep(50 * time.Millisecond)
	win.Send(SwitchToPane{Index: 0})
	time.Sleep(20 * time.Millisecond)
	win.Send(Split{Dir: SplitH})
	time.Sleep(50 * time.Millisecond)

	// Kill the newly-created lower-left pane. Layout should collapse back to two panes.
	active := <-sessionRef.Ask(GetActivePane{})
	if active == nil {
		t.Fatal("expected active pane before kill")
	}
	active.(*PaneRef).Send(KillPane{})
	time.Sleep(100 * time.Millisecond)

	sessionRef.Send(ResizeMsg{Rows: 24, Cols: 80})
	super.waitContentUpdated(200 * time.Millisecond)

	viewResult := <-sessionRef.Ask(GetWindowView{})
	if viewResult == nil {
		t.Fatal("Expected window view after pane close")
	}
	view, ok := viewResult.(WindowView)
	if !ok || view.Content == "" {
		t.Fatal("Window view should be non-empty")
	}
	if !strings.Contains(view.Content, "│") {
		t.Fatal("Expected remaining vertical divider after pane close")
	}
	for _, glyph := range []string{"─", "├", "┤", "┬", "┴", "┼"} {
		if strings.Contains(view.Content, glyph) {
			t.Fatalf("Did not expect %q after collapsed split", glyph)
		}
	}
}

func TestWindowResizePaneAdjustsSplitRatio(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)

	pollFor(100*time.Millisecond, func() bool {
		result := <-win.Ask(GetActivePane{})
		return result != nil
	})

	win.Send(Split{Dir: SplitV})
	time.Sleep(50 * time.Millisecond)
	sessionRef.Send(ResizeMsg{Rows: 24, Cols: 80})
	super.waitContentUpdated(200 * time.Millisecond)

	widthAt := func(index int) int {
		win.Send(SwitchToPane{Index: index})
		time.Sleep(20 * time.Millisecond)
		result := <-sessionRef.Ask(GetPaneContent{})
		if result == nil {
			return 0
		}
		content := result.(*PaneContent)
		if len(content.Cells) == 0 {
			return 0
		}
		return len(content.Cells[0])
	}

	beforeLeft := widthAt(0)
	beforeRight := widthAt(1)

	win.Send(SwitchToPane{Index: 0})
	time.Sleep(20 * time.Millisecond)
	win.Send(ResizePane{Dir: PaneNavRight, Amount: 5})
	time.Sleep(50 * time.Millisecond)

	afterLeft := widthAt(0)
	afterRight := widthAt(1)

	if afterLeft <= beforeLeft {
		t.Fatalf("expected left pane width to grow, before=%d after=%d", beforeLeft, afterLeft)
	}
	if afterRight >= beforeRight {
		t.Fatalf("expected right pane width to shrink, before=%d after=%d", beforeRight, afterRight)
	}
}

func TestWindowMouseDragDividerResizesPanes(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)

	pollFor(100*time.Millisecond, func() bool {
		result := <-win.Ask(GetActivePane{})
		return result != nil
	})

	win.Send(Split{Dir: SplitV})
	time.Sleep(50 * time.Millisecond)
	sessionRef.Send(ResizeMsg{Rows: 24, Cols: 80})
	super.waitContentUpdated(200 * time.Millisecond)

	widthAt := func(index int) int {
		win.Send(SwitchToPane{Index: index})
		time.Sleep(20 * time.Millisecond)
		result := <-sessionRef.Ask(GetPaneContent{})
		if result == nil {
			return 0
		}
		content := result.(*PaneContent)
		if len(content.Cells) == 0 {
			return 0
		}
		return len(content.Cells[0])
	}

	beforeLeft := widthAt(0)
	beforeRight := widthAt(1)

	win.Send(MouseInput{Action: MouseActionPress, Button: MouseButtonLeft, Row: 10, Col: 40})
	win.Send(MouseInput{Action: MouseActionMotion, Button: MouseButtonLeft, Row: 10, Col: 45})
	win.Send(MouseInput{Action: MouseActionRelease, Button: MouseButtonLeft, Row: 10, Col: 45})
	time.Sleep(50 * time.Millisecond)

	afterLeft := widthAt(0)
	afterRight := widthAt(1)
	if afterLeft <= beforeLeft {
		t.Fatalf("expected left pane width to grow after mouse drag, before=%d after=%d", beforeLeft, afterLeft)
	}
	if afterRight >= beforeRight {
		t.Fatalf("expected right pane width to shrink after mouse drag, before=%d after=%d", beforeRight, afterRight)
	}
}
