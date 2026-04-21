//go:build stress
// +build stress

package stress

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"shux/pkg/shux"
	"shux/pkg/testutil"
)

// ChaosMonkey randomly injects faults and validates recovery.
type ChaosMonkey struct {
	rng       *rand.Rand
	faults    int
	recovered int
}

// NewChaosMonkey creates a new chaos tester with deterministic seed.
func NewChaosMonkey(seed int64) *ChaosMonkey {
	return &ChaosMonkey{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// ShouldFault returns true with given probability.
func (c *ChaosMonkey) ShouldFault(probability float64) bool {
	return c.rng.Float64() < probability
}

// RecordFault records a fault occurrence.
func (c *ChaosMonkey) RecordFault() {
	c.faults++
}

// RecordRecovery records a successful recovery.
func (c *ChaosMonkey) RecordRecovery() {
	c.recovered++
}

// Stats returns chaos statistics.
func (c *ChaosMonkey) Stats() (faults, recovered int) {
	return c.faults, c.recovered
}

// TestChaosRandomPaneKills validates session survives random pane kills.
func TestChaosRandomPaneKills(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	monkey := NewChaosMonkey(42)

	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Build up panes
	for i := 0; i < 5; i++ {
		win.Send(shux.Split{Dir: shux.SplitV})
		testutil.PollFor(200*time.Millisecond, func() bool {
			result := <-win.Ask(shux.GetWindowSnapshotData{})
			if data, ok := result.(shux.WindowSnapshot); ok {
				return len(data.PaneOrder) == i+2
			}
			return false
		})
	}

	// Randomly kill panes with 30% probability per operation
	for i := 0; i < 20; i++ {
		if monkey.ShouldFault(0.3) {
			monkey.RecordFault()
			result := <-sessionRef.Ask(shux.GetActivePane{})
			if result != nil {
				pane := result.(*shux.PaneRef)
				pane.Send(shux.KillPane{})
				time.Sleep(50 * time.Millisecond)
			}
		}

		// Perform normal operation
		win.Send(shux.NavigatePane{Dir: shux.PaneNavDir(monkey.rng.Intn(4))})
		time.Sleep(10 * time.Millisecond)

		// Verify session still valid
		result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
		if result != nil {
			monkey.RecordRecovery()
		}
	}

	faults, recovered := monkey.Stats()
	t.Logf("Chaos test: %d faults, %d recoveries", faults, recovered)

	if recovered < faults {
		t.Errorf("recovered (%d) < faults (%d): session did not survive all chaos", recovered, faults)
	}
}

// TestChaosSessionDetachInterrupt validates handling of interrupted detaches.
func TestChaosSessionDetachInterrupt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	testutil.WithTempHome(t, func(tempDir string) {
		sessionRef, super, cleanup := testutil.SetupNamedSession(t, "chaos-detach")

		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		testutil.RequireWindow(t, sessionRef, super)

		// Corrupt the snapshot file mid-detach
		go func() {
			time.Sleep(50 * time.Millisecond)
			path := shux.SessionSnapshotPath("chaos-detach")
			if _, err := os.Stat(path); err == nil {
				// Corrupt the file
				os.WriteFile(path, []byte("corrupted"), 0o644)
			}
		}()

		// Attempt detach
		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply
		super.WaitSessionEmpty(2 * time.Second)
		cleanup()

		// Try to restore - should handle gracefully
		super2 := testutil.NewTestSupervisor()
		_, err := shux.RestoreSessionFromSnapshot("chaos-detach", super2.Handle, testutil.TestLogger())
		// Should error on corrupted snapshot, not panic
		if err == nil {
			t.Log("Note: restore succeeded despite chaos (file may not have been corrupted in time)")
		}
	})
}

// TestChaosConcurrentSnapshots validates concurrent snapshot operations.
func TestChaosConcurrentSnapshots(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	testutil.WithTempHome(t, func(tempDir string) {
		sessionRef, super, cleanup := testutil.SetupNamedSession(t, "chaos-snapshot")
		defer cleanup()

		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		win := testutil.RequireWindow(t, sessionRef, super)
		_ = testutil.RequirePane(t, sessionRef, super)

		// Build some state
		win.Send(shux.Split{Dir: shux.SplitV})
		testutil.PollFor(200*time.Millisecond, func() bool {
			result := <-win.Ask(shux.GetWindowSnapshotData{})
			if data, ok := result.(shux.WindowSnapshot); ok {
				return len(data.PaneOrder) == 2
			}
			return false
		})

		// Concurrent snapshot requests
		done := make(chan *shux.SessionSnapshot, 5)
		for i := 0; i < 5; i++ {
			go func() {
				result := <-sessionRef.Ask(shux.GetFullSessionSnapshot{})
				if result != nil {
					done <- result.(*shux.SessionSnapshot)
				} else {
					done <- nil
				}
			}()
		}

		// Collect results
		var snapshots []*shux.SessionSnapshot
		for i := 0; i < 5; i++ {
			select {
			case snap := <-done:
				if snap != nil {
					snapshots = append(snapshots, snap)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("concurrent snapshot timeout")
			}
		}

		// All snapshots should be valid
		for i, snap := range snapshots {
			if err := shux.ValidateSnapshot(snap); err != nil {
				t.Errorf("snapshot %d invalid: %v", i, err)
			}
		}

		t.Logf("Concurrent snapshots: %d valid snapshots generated", len(snapshots))
	})
}

// TestChaosRapidResize validates rapid resize operations.
func TestChaosRapidResize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	win.Send(shux.Split{Dir: shux.SplitV})
	testutil.PollFor(200*time.Millisecond, func() bool {
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			return len(data.PaneOrder) == 2
		}
		return false
	})

	monkey := NewChaosMonkey(123)

	// Rapidly resize with random dimensions
	for i := 0; i < 50; i++ {
		rows := 5 + monkey.rng.Intn(50)
		cols := 20 + monkey.rng.Intn(100)

		win.Send(shux.ResizeMsg{Rows: rows, Cols: cols})

		// Interleave with pane operations
		if monkey.ShouldFault(0.2) {
			win.Send(shux.ResizePane{Dir: shux.PaneNavDir(monkey.rng.Intn(4)), Amount: monkey.rng.Intn(10)})
		}

		time.Sleep(5 * time.Millisecond)
	}

	// Verify system is still responsive
	result := <-win.Ask(shux.GetWindowSnapshotData{})
	if result == nil {
		t.Error("window unresponsive after rapid resize chaos")
	}
}

// TestChaosTopologyMutations validates random topology changes.
func TestChaosTopologyMutations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	monkey := NewChaosMonkey(456)

	// Randomly split and kill panes
	for i := 0; i < 30; i++ {
		op := monkey.rng.Intn(3)
		switch op {
		case 0: // Split
			dir := shux.SplitH
			if monkey.rng.Intn(2) == 1 {
				dir = shux.SplitV
			}
			win.Send(shux.Split{Dir: dir})

		case 1: // Kill active pane
			if monkey.ShouldFault(0.3) {
				result := <-sessionRef.Ask(shux.GetActivePane{})
				if result != nil {
					pane := result.(*shux.PaneRef)
					pane.Send(shux.KillPane{})
				}
			}

		case 2: // Navigate
			win.Send(shux.NavigatePane{Dir: shux.PaneNavDir(monkey.rng.Intn(4))})
		}

		time.Sleep(20 * time.Millisecond)

		// Verify topology is still valid
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			// Active pane must exist in pane order
			if data.ActivePane != 0 {
				found := false
				for _, pid := range data.PaneOrder {
					if pid == data.ActivePane {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("topology corruption: active pane %d not in order %v", data.ActivePane, data.PaneOrder)
				}
			}
		}
	}
}
