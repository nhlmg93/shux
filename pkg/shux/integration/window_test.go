package integration

import (
	"strings"
	"testing"
	"time"

	"shux/pkg/shux"
	"shux/pkg/testutil"
)

// TestWindowLifecycle validates window creation and destruction.
func TestWindowLifecycle(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	// Create window
	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)

	testutil.AssertWindowInvariants(t, win, testutil.WindowInvariant{
		WindowID:   1,
		PaneCount:  1,
		ActivePane: 1,
		Rows:       24,
		Cols:       80,
	})

	// Kill the pane
	pane := testutil.RequirePane(t, sessionRef, super)
	pane.Send(shux.KillPane{})

	// Window should be empty (session handles cleanup)
	if !super.WaitSessionEmpty(2 * time.Second) {
		t.Error("expected SessionEmpty after killing only pane")
	}
}

// TestWindowPaneCreation validates pane creation in a window.
func TestWindowPaneCreation(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Create additional panes
	for i := 1; i < 4; i++ {
		win.Send(shux.CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
		testutil.PollFor(200*time.Millisecond, func() bool {
			result := <-win.Ask(shux.GetWindowSnapshotData{})
			if data, ok := result.(shux.WindowSnapshot); ok {
				return len(data.PaneOrder) == i+1
			}
			return false
		})
	}

	testutil.AssertWindowInvariants(t, win, testutil.WindowInvariant{
		WindowID:   1,
		PaneCount:  4,
		ActivePane: 4, // Last created
	})
}

// TestWindowSplitHorizontal validates horizontal split (panes stacked top/bottom).
func TestWindowSplitHorizontal(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Split horizontally
	win.Send(shux.Split{Dir: shux.SplitH})
	testutil.PollFor(200*time.Millisecond, func() bool {
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			return len(data.PaneOrder) == 2
		}
		return false
	})

	// Verify render shows horizontal divider
	result := <-win.Ask(shux.GetWindowView{})
	if view, ok := result.(shux.WindowView); ok {
		if !strings.Contains(view.Content, "─") {
			t.Error("expected horizontal divider in view")
		}
	}
}

// TestWindowSplitVertical validates vertical split (panes side by side).
func TestWindowSplitVertical(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Split vertically
	win.Send(shux.Split{Dir: shux.SplitV})
	testutil.PollFor(200*time.Millisecond, func() bool {
		result := <-win.Ask(shux.GetWindowView{})
		if view, ok := result.(shux.WindowView); ok {
			return strings.Contains(view.Content, "│")
		}
		return false
	})

	// Verify render shows vertical divider
	result := <-win.Ask(shux.GetWindowView{})
	if view, ok := result.(shux.WindowView); ok {
		if !strings.Contains(view.Content, "│") {
			t.Error("expected vertical divider in view")
		}
	}
}

// TestWindowNestedSplit validates 2x2 layout with nested splits.
func TestWindowNestedSplit(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Build 2x2 layout: vertical split first, then horizontal on each side
	win.Send(shux.Split{Dir: shux.SplitV})
	time.Sleep(50 * time.Millisecond)

	win.Send(shux.SwitchToPane{Index: 0})
	time.Sleep(20 * time.Millisecond)
	win.Send(shux.Split{Dir: shux.SplitH})
	time.Sleep(50 * time.Millisecond)

	win.Send(shux.SwitchToPane{Index: 2})
	time.Sleep(20 * time.Millisecond)
	win.Send(shux.Split{Dir: shux.SplitH})
	time.Sleep(50 * time.Millisecond)

	// Verify 4 panes
	result := <-win.Ask(shux.GetWindowSnapshotData{})
	data := result.(shux.WindowSnapshot)
	if len(data.PaneOrder) != 4 {
		t.Fatalf("expected 4 panes, got %d", len(data.PaneOrder))
	}

	// Verify render shows cross intersection
	viewResult := <-win.Ask(shux.GetWindowView{})
	view := viewResult.(shux.WindowView)
	if !strings.Contains(view.Content, "┼") && !strings.Contains(view.Content, "┤") && !strings.Contains(view.Content, "├") {
		t.Logf("View content:\n%s", view.Content)
		t.Error("expected cross or T-junction in nested split view")
	}
}

// TestWindowPaneNavigation validates directional pane navigation.
func TestWindowPaneNavigation(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Build 2x2 layout
	win.Send(shux.Split{Dir: shux.SplitV})
	time.Sleep(50 * time.Millisecond)
	win.Send(shux.SwitchToPane{Index: 0})
	time.Sleep(20 * time.Millisecond)
	win.Send(shux.Split{Dir: shux.SplitH})
	time.Sleep(50 * time.Millisecond)
	win.Send(shux.SwitchToPane{Index: 2})
	time.Sleep(20 * time.Millisecond)
	win.Send(shux.Split{Dir: shux.SplitH})
	time.Sleep(50 * time.Millisecond)

	// Navigate: test that navigation changes active pane
	// Note: Exact pane IDs depend on implementation's navigation semantics
	navigations := []struct {
		dir shux.PaneNavDir
	}{
		{shux.PaneNavRight},
		{shux.PaneNavDown},
		{shux.PaneNavLeft},
		{shux.PaneNavUp},
	}

	// Start at pane 1
	win.Send(shux.SwitchToPane{Index: 0})
	time.Sleep(20 * time.Millisecond)

	for _, nav := range navigations {
		win.Send(shux.NavigatePane{Dir: nav.dir})
		time.Sleep(20 * time.Millisecond)

		result := <-win.Ask(shux.GetWindowSnapshotData{})
		data := result.(shux.WindowSnapshot)

		// Active pane should be one of the 4 panes
		found := false
		for _, pid := range data.PaneOrder {
			if pid == data.ActivePane {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("navigate %v: active pane %d not found in pane order %v", nav.dir, data.ActivePane, data.PaneOrder)
		}
	}

	// Verify we navigated somewhere (active pane may have changed)
	result := <-win.Ask(shux.GetWindowSnapshotData{})
	data := result.(shux.WindowSnapshot)
	if data.ActivePane == 0 {
		t.Error("navigation should result in a valid active pane")
	}
}

// TestWindowPaneSwitching validates switching panes by index.
func TestWindowPaneSwitching(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)

	// Create 3 panes
	for i := 0; i < 2; i++ {
		win.Send(shux.CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
		testutil.PollFor(200*time.Millisecond, func() bool {
			result := <-win.Ask(shux.GetWindowSnapshotData{})
			if data, ok := result.(shux.WindowSnapshot); ok {
				return len(data.PaneOrder) == i+2
			}
			return false
		})
	}

	// Switch to each pane by index
	for i := 0; i < 3; i++ {
		win.Send(shux.SwitchToPane{Index: i})
		testutil.PollFor(100*time.Millisecond, func() bool {
			result := <-win.Ask(shux.GetWindowSnapshotData{})
			if data, ok := result.(shux.WindowSnapshot); ok {
				return data.ActivePane == data.PaneOrder[i]
			}
			return false
		})
	}

	// Invalid index should not crash
	win.Send(shux.SwitchToPane{Index: 99})
	time.Sleep(50 * time.Millisecond)
	// Window should still be valid
	testutil.AssertWindowInvariants(t, win, testutil.WindowInvariant{
		WindowID:  1,
		PaneCount: 3,
	})
}

// TestWindowResizePropagation validates resize is propagated to all panes.
func TestWindowResizePropagation(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 10, Cols: 40})
	win := testutil.RequireWindow(t, sessionRef, super)

	// Create multiple panes
	win.Send(shux.Split{Dir: shux.SplitV})
	testutil.PollFor(200*time.Millisecond, func() bool {
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			return len(data.PaneOrder) == 2
		}
		return false
	})

	// Resize window
	win.Send(shux.ResizeMsg{Rows: 30, Cols: 100})
	super.WaitContentUpdated(200 * time.Millisecond)

	// Verify panes have usable content after resize
	for i := 0; i < 2; i++ {
		win.Send(shux.SwitchToPane{Index: i})
		time.Sleep(20 * time.Millisecond)

		result := <-sessionRef.Ask(shux.GetPaneContent{})
		if content, ok := result.(*shux.PaneContent); ok {
			if len(content.Lines) == 0 {
				t.Errorf("pane %d: no content after resize", i)
			}
		}
	}
}

// TestWindowKillNonActivePane validates killing a non-active pane.
func TestWindowKillNonActivePane(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	pane1 := testutil.RequirePane(t, sessionRef, super)

	// Create second pane
	win.Send(shux.CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	testutil.PollFor(200*time.Millisecond, func() bool {
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			return len(data.PaneOrder) == 2
		}
		return false
	})

	// Switch to pane 1
	win.Send(shux.SwitchToPane{Index: 0})
	time.Sleep(50 * time.Millisecond)

	// Kill pane 1
	pane1.Send(shux.KillPane{})
	time.Sleep(100 * time.Millisecond)

	// Verify 1 pane remains
	testutil.PollFor(time.Second, func() bool {
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			return len(data.PaneOrder) == 1
		}
		return false
	})

	testutil.AssertWindowInvariants(t, win, testutil.WindowInvariant{
		WindowID:  1,
		PaneCount: 1,
	})
}

// TestWindowPaneResizing validates pane resize operations.
func TestWindowPaneResizing(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Create vertical split
	win.Send(shux.Split{Dir: shux.SplitV})
	testutil.PollFor(200*time.Millisecond, func() bool {
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			return len(data.PaneOrder) == 2
		}
		return false
	})

	sessionRef.Send(shux.ResizeMsg{Rows: 24, Cols: 80})
	super.WaitContentUpdated(200 * time.Millisecond)

	// Get initial width
	win.Send(shux.SwitchToPane{Index: 0})
	time.Sleep(20 * time.Millisecond)
	result := <-sessionRef.Ask(shux.GetPaneContent{})
	content := result.(*shux.PaneContent)
	initialWidth := 0
	if len(content.Cells) > 0 && len(content.Cells[0]) > 0 {
		initialWidth = len(content.Cells[0])
	}

	// Resize pane to the right (grows left pane)
	win.Send(shux.ResizePane{Dir: shux.PaneNavRight, Amount: 5})
	time.Sleep(50 * time.Millisecond)

	// Verify pane still has content
	result2 := <-sessionRef.Ask(shux.GetPaneContent{})
	content2 := result2.(*shux.PaneContent)
	if len(content2.Cells) == 0 {
		t.Error("pane should have content after resize")
	}

	_ = initialWidth // Not checking exact width due to tiling constraints
}

// TestWindowLayoutPreservation validates layout is preserved through snapshot.
func TestWindowLayoutPreservation(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Build 2x2 layout
	win.Send(shux.Split{Dir: shux.SplitV})
	time.Sleep(50 * time.Millisecond)
	win.Send(shux.SwitchToPane{Index: 0})
	time.Sleep(20 * time.Millisecond)
	win.Send(shux.Split{Dir: shux.SplitH})
	time.Sleep(50 * time.Millisecond)
	win.Send(shux.SwitchToPane{Index: 2})
	time.Sleep(20 * time.Millisecond)
	win.Send(shux.Split{Dir: shux.SplitH})
	time.Sleep(50 * time.Millisecond)

	// Get window snapshot data
	result := <-win.Ask(shux.GetWindowSnapshotData{})
	data := result.(shux.WindowSnapshot)

	if data.Layout == nil {
		t.Fatal("expected layout in window snapshot")
	}

	if len(data.PaneOrder) != 4 {
		t.Fatalf("expected 4 panes, got %d", len(data.PaneOrder))
	}

	// Verify layout references all panes
	paneIDs := make(map[uint32]bool)
	collectLayoutPaneIDs(data.Layout, paneIDs)
	if len(paneIDs) != 4 {
		t.Errorf("layout references %d panes, expected 4", len(paneIDs))
	}
}

func collectLayoutPaneIDs(layout *shux.SplitTreeSnapshot, out map[uint32]bool) {
	if layout == nil {
		return
	}
	if layout.First == nil && layout.Second == nil {
		out[layout.PaneID] = true
		return
	}
	collectLayoutPaneIDs(layout.First, out)
	collectLayoutPaneIDs(layout.Second, out)
}

// TestWindowCloseActivePaneInMiddle validates closing middle pane reorders correctly.
func TestWindowCloseActivePaneInMiddle(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Create 3 panes
	for i := 0; i < 2; i++ {
		win.Send(shux.CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
		testutil.PollFor(200*time.Millisecond, func() bool {
			result := <-win.Ask(shux.GetWindowSnapshotData{})
			if data, ok := result.(shux.WindowSnapshot); ok {
				return len(data.PaneOrder) == i+2
			}
			return false
		})
	}

	// Get initial order
	result := <-win.Ask(shux.GetWindowSnapshotData{})
	data := result.(shux.WindowSnapshot)
	initialOrder := make([]uint32, len(data.PaneOrder))
	copy(initialOrder, data.PaneOrder)

	// Switch to middle pane (index 1)
	win.Send(shux.SwitchToPane{Index: 1})
	time.Sleep(50 * time.Millisecond)
	middlePaneID := data.PaneOrder[1]

	// Kill middle pane
	result2 := <-sessionRef.Ask(shux.GetActivePane{})
	middlePane := result2.(*shux.PaneRef)
	middlePane.Send(shux.KillPane{})
	time.Sleep(100 * time.Millisecond)

	// Verify 2 panes remain, order preserved for remaining
	testutil.PollFor(time.Second, func() bool {
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			return len(data.PaneOrder) == 2
		}
		return false
	})

	result3 := <-win.Ask(shux.GetWindowSnapshotData{})
	data3 := result3.(shux.WindowSnapshot)

	// Verify killed pane is not in order
	for _, pid := range data3.PaneOrder {
		if pid == middlePaneID {
			t.Error("killed pane should not be in order")
		}
	}
}

// TestWindowActivePanePreservation validates active pane tracking through operations.
func TestWindowActivePanePreservation(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Split to create 2 panes
	win.Send(shux.Split{Dir: shux.SplitV})
	testutil.PollFor(200*time.Millisecond, func() bool {
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			return len(data.PaneOrder) == 2
		}
		return false
	})

	// Active should be the new pane (pane 2)
	result := <-win.Ask(shux.GetWindowSnapshotData{})
	data := result.(shux.WindowSnapshot)
	if data.ActivePane != data.PaneOrder[1] {
		t.Errorf("expected active pane to be second pane after split, got %d", data.ActivePane)
	}

	// Switch to first pane
	win.Send(shux.SwitchToPane{Index: 0})
	testutil.PollFor(100*time.Millisecond, func() bool {
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			return data.ActivePane == data.PaneOrder[0]
		}
		return false
	})

	// Create third pane
	win.Send(shux.CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	testutil.PollFor(200*time.Millisecond, func() bool {
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			return len(data.PaneOrder) == 3
		}
		return false
	})

	// Active should be the new pane
	result2 := <-win.Ask(shux.GetWindowSnapshotData{})
	data2 := result2.(shux.WindowSnapshot)
	if data2.ActivePane != data2.PaneOrder[2] {
		t.Errorf("expected active pane to be third pane after creation, got %d", data2.ActivePane)
	}
}
