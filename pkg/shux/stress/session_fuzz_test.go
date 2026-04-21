//go:build stress
// +build stress

package stress

import (
	"math/rand"
	"testing"
	"time"

	"shux/pkg/shux"
	"shux/pkg/testutil"
)

// FuzzSessionOperations performs randomized session operations and validates invariants.
func FuzzSessionOperations(f *testing.F) {
	// Seed corpus: sequences of operations
	f.Add([]byte{0})                                        // Single operation
	f.Add([]byte{0, 1, 2})                                  // Mixed operations
	f.Add([]byte{0, 0, 0, 0, 0})                            // Repeated same operation
	f.Add([]byte{0, 1, 0, 1, 0, 1})                         // Alternating pattern
	f.Add([]byte{0, 1, 2, 3, 0, 1, 2, 3})                   // Cycling pattern
	f.Add([]byte{1, 3, 5, 7, 9, 2, 4, 6, 8, 0})             // Alternating odd/even operations
	f.Add([]byte{255, 255, 255})                            // Invalid operations
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}) // All operations

	f.Fuzz(func(t *testing.T, ops []byte) {
		sessionRef, super, cleanup := testutil.SetupSession(t)
		defer cleanup()

		// Initial window
		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		if !super.WaitContentUpdated(500 * time.Millisecond) {
			t.Fatal("timeout waiting for initial window")
		}

		win := testutil.RequireWindow(t, sessionRef, super)

		// Execute random operations
		for i, op := range ops {
			if i >= 30 { // Limit operations per test
				break
			}

			switch op % 10 {
			case 0: // Split horizontal
				win.Send(shux.Split{Dir: shux.SplitH})
			case 1: // Split vertical
				win.Send(shux.Split{Dir: shux.SplitV})
			case 2: // Navigate pane
				win.Send(shux.NavigatePane{Dir: shux.PaneNavDir(rand.Intn(4))})
			case 3: // Switch pane
				win.Send(shux.SwitchToPane{Index: rand.Intn(5)})
			case 4: // Resize pane
				win.Send(shux.ResizePane{Dir: shux.PaneNavDir(rand.Intn(4)), Amount: 1 + rand.Intn(3)})
			case 5: // Switch window
				sessionRef.Send(shux.SwitchWindow{Delta: rand.Intn(3) - 1})
			case 6: // Resize window
				win.Send(shux.ResizeMsg{Rows: 10 + rand.Intn(30), Cols: 40 + rand.Intn(80)})
			case 7: // Create pane
				win.Send(shux.CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
			case 8: // Get snapshot data
				sessionRef.Ask(shux.GetSessionSnapshotData{})
			case 9: // Get window view
				sessionRef.Ask(shux.GetWindowView{})
			}

			time.Sleep(5 * time.Millisecond)
		}

		// Validate final state
		result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
		if result == nil {
			t.Error("session should still be responsive after fuzz operations")
		}

		// Verify window invariants if window exists
		winResult := <-sessionRef.Ask(shux.GetActiveWindow{})
		if winResult != nil {
			winRef := winResult.(*shux.WindowRef)
			winSnapshot := <-winRef.Ask(shux.GetWindowSnapshotData{})
			if winData, ok := winSnapshot.(shux.WindowSnapshot); ok {
				// PaneOrder and Pane count should match
				if len(winData.PaneOrder) > 0 && winData.ActivePane != 0 {
					found := false
					for _, pid := range winData.PaneOrder {
						if pid == winData.ActivePane {
							found = true
							break
						}
					}
					if !found {
						t.Error("active pane not found in pane order")
					}
				}
			}
		}
	})
}

// TestSessionRandomizedOperations runs a deterministic random session test.
func TestSessionRandomizedOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	seed := int64(12345) // Deterministic seed
	rng := rand.New(rand.NewSource(seed))

	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	if !super.WaitContentUpdated(500 * time.Millisecond) {
		t.Fatal("timeout waiting for initial window")
	}

	win := testutil.RequireWindow(t, sessionRef, super)

	// Run 100 random operations
	for i := 0; i < 100; i++ {
		op := rng.Intn(8)
		switch op {
		case 0:
			win.Send(shux.Split{Dir: shux.SplitH})
		case 1:
			win.Send(shux.Split{Dir: shux.SplitV})
		case 2:
			win.Send(shux.NavigatePane{Dir: shux.PaneNavDir(rng.Intn(4))})
		case 3:
			win.Send(shux.SwitchToPane{Index: rng.Intn(10)})
		case 4:
			win.Send(shux.ResizePane{Dir: shux.PaneNavDir(rng.Intn(4)), Amount: rng.Intn(5)})
		case 5:
			sessionRef.Send(shux.SwitchWindow{Delta: rng.Intn(3) - 1})
		case 6:
			win.Send(shux.ResizeMsg{Rows: 10 + rng.Intn(40), Cols: 40 + rng.Intn(80)})
		case 7:
			win.Send(shux.CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
		}

		if i%10 == 0 {
			// Verify state every 10 operations
			result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
			if result == nil {
				t.Fatalf("session unresponsive after %d operations", i)
			}
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Logf("Completed 100 randomized operations with seed %d", seed)
}

// TestSessionConcurrentOperations validates concurrent access patterns.
func TestSessionConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	if !super.WaitContentUpdated(500 * time.Millisecond) {
		t.Fatal("timeout waiting for initial window")
	}

	win := testutil.RequireWindow(t, sessionRef, super)

	// Create panes for concurrent access
	for i := 0; i < 3; i++ {
		win.Send(shux.Split{Dir: shux.SplitV})
		testutil.PollFor(200*time.Millisecond, func() bool {
			result := <-win.Ask(shux.GetWindowSnapshotData{})
			if data, ok := result.(shux.WindowSnapshot); ok {
				return len(data.PaneOrder) >= i+2
			}
			return false
		})
	}

	// Concurrent operations
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 20; j++ {
				switch j % 4 {
				case 0:
					win.Send(shux.NavigatePane{Dir: shux.PaneNavRight})
				case 1:
					sessionRef.Ask(shux.GetActivePane{})
				case 2:
					sessionRef.Ask(shux.GetSessionSnapshotData{})
				case 3:
					win.Send(shux.ResizeMsg{Rows: 24, Cols: 80})
				}
				time.Sleep(5 * time.Millisecond)
			}
			done <- nil
		}(i)
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("concurrent operation error: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent operations timeout")
		}
	}
}

// TestSessionMemoryPressure validates behavior under memory pressure simulation.
func TestSessionMemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	sessionRef, _, cleanup := testutil.SetupSession(t)
	defer cleanup()

	// Create many windows
	for i := 0; i < 10; i++ {
		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		testutil.PollFor(300*time.Millisecond, func() bool {
			result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
			if data, ok := result.(shux.SessionSnapshotData); ok {
				return len(data.WindowOrder) == i+1
			}
			return false
		})
	}

	// Verify all windows are tracked
	result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
	data := result.(shux.SessionSnapshotData)
	if len(data.WindowOrder) != 10 {
		t.Errorf("expected 10 windows, got %d", len(data.WindowOrder))
	}

	// Rapidly switch between windows
	for i := 0; i < 50; i++ {
		sessionRef.Send(shux.SwitchWindow{Delta: 1})
		time.Sleep(10 * time.Millisecond)
	}

	// Session should still be responsive
	final := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
	if final == nil {
		t.Error("session should still be responsive after stress")
	}
}
