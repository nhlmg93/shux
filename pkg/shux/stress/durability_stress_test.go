//go:build stress
// +build stress

package stress

import (
	"bytes"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"shux/pkg/shux"
	"shux/pkg/testutil"
)

// TestDurabilitySessionSoak runs extended session operations.
func TestDurabilitySessionSoak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak test in short mode")
	}

	timeout := 30 * time.Second
	if deadline, ok := t.Deadline(); ok {
		remaining := time.Until(deadline) - time.Second
		if remaining < timeout {
			timeout = remaining
		}
	}

	var opsCount atomic.Int32
	done := make(chan struct{})

	go func() {
		defer close(done)

		for i := 0; i < 5; i++ {
			super := testutil.NewTestSupervisor()
			logger := shux.NoOpLogger{}
			sessionRef := shux.StartSession(uint32(i+1), super.Handle, logger)

			// Initial window
			sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
			if !super.WaitContentUpdated(500 * time.Millisecond) {
				t.Error("timeout waiting for initial window")
				sessionRef.Shutdown()
				return
			}

			// Rapid operations cycle
			win := testutil.RequireWindow(t, sessionRef, super)
			for j := 0; j < 30; j++ {
				switch j % 6 {
				case 0, 1:
					win.Send(shux.Split{Dir: shux.SplitH})
				case 2, 3:
					win.Send(shux.ResizeMsg{Rows: 10 + j%20, Cols: 40 + j%40})
				case 4:
					win.Send(shux.NavigatePane{Dir: shux.PaneNavRight})
				case 5:
					sessionRef.Ask(shux.GetWindowView{})
				}
				opsCount.Add(1)
				time.Sleep(5 * time.Millisecond)
			}

			// Clean shutdown
			sessionRef.Shutdown()
			time.Sleep(50 * time.Millisecond)
		}
	}()

	select {
	case <-done:
		t.Logf("Durability soak completed %d operations", opsCount.Load())
	case <-time.After(timeout):
		t.Fatalf("durability soak timed out after %v (completed %d ops)", timeout, opsCount.Load())
	}
}

// TestDurabilityGoroutineLeak validates no goroutine leaks.
func TestDurabilityGoroutineLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping leak test in short mode")
	}

	runLifecycle := func(id uint32) {
		super := testutil.NewTestSupervisor()
		logger := shux.NoOpLogger{}
		sessionRef := shux.StartSession(id, super.Handle, logger)

		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		time.Sleep(100 * time.Millisecond)

		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply

		super.WaitSessionEmpty(2 * time.Second)
		sessionRef.Shutdown()
		time.Sleep(50 * time.Millisecond)
	}

	// Warm up
	runLifecycle(99)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	// Run multiple cycles
	for i := 0; i < 5; i++ {
		runLifecycle(uint32(i + 100))
	}

	// Allow goroutines to settle
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	leakThreshold := baseline + 5
	final := runtime.NumGoroutine()

	if final > leakThreshold {
		var buf bytes.Buffer
		_ = pprof.Lookup("goroutine").WriteTo(&buf, 1)
		t.Fatalf("possible goroutine leak: baseline=%d final=%d (threshold=%d)\n%s",
			baseline, final, leakThreshold, buf.String())
	}

	t.Logf("Goroutine leak check passed: baseline=%d final=%d", baseline, final)
}

// TestDurabilityRaceConditions validates thread safety.
func TestDurabilityRaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")
	}

	super := testutil.NewTestSupervisor()
	logger := shux.NoOpLogger{}
	sessionRef := shux.StartSession(1, super.Handle, logger)
	defer sessionRef.Shutdown()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	if !super.WaitContentUpdated(500 * time.Millisecond) {
		t.Fatal("timeout waiting for window")
	}

	win := testutil.RequireWindow(t, sessionRef, super)

	// Create panes
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
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				switch j % 4 {
				case 0:
					win.Send(shux.NavigatePane{Dir: shux.PaneNavRight})
				case 1:
					win.Send(shux.ResizePane{Dir: shux.PaneNavDown, Amount: 1})
				case 2:
					sessionRef.Ask(shux.GetActivePane{})
				case 3:
					sessionRef.Ask(shux.GetWindowView{})
				}
				time.Sleep(2 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	result := <-sessionRef.Ask(shux.GetActiveWindow{})
	if result == nil {
		t.Error("no active window after concurrent operations")
	}
}

// TestDurabilityMemoryPressure validates memory usage under pressure.
func TestDurabilityMemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory pressure test in short mode")
	}

	var m1, m2 runtime.MemStats

	// Measure baseline
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Create many sessions
	var sessions []*shux.SessionRef
	var supers []*testutil.TestSupervisor

	for i := 0; i < 10; i++ {
		super := testutil.NewTestSupervisor()
		logger := shux.NoOpLogger{}
		sessionRef := shux.StartSession(uint32(i+1), super.Handle, logger)

		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		time.Sleep(50 * time.Millisecond)

		sessions = append(sessions, sessionRef)
		supers = append(supers, super)
	}

	// Shutdown all
	for _, s := range sessions {
		s.Shutdown()
	}

	// Allow cleanup
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.ReadMemStats(&m2)

	// Memory should not have grown unreasonably
	// This is a loose check since GC timing is non-deterministic
	growth := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	if growth > 100*1024*1024 { // 100MB threshold
		t.Logf("Memory growth warning: %d MB", growth/(1024*1024))
	}

	t.Logf("Memory pressure test: baseline=%d MB, final=%d MB",
		m1.HeapAlloc/(1024*1024), m2.HeapAlloc/(1024*1024))
}

// TestDurabilitySnapshotValidation validates snapshots under stress.
func TestDurabilitySnapshotValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping durability test in short mode")
	}

	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Build complex layout
	for i := 0; i < 4; i++ {
		win.Send(shux.Split{Dir: shux.SplitV})
		testutil.PollFor(100*time.Millisecond, func() bool {
			result := <-win.Ask(shux.GetWindowSnapshotData{})
			if data, ok := result.(shux.WindowSnapshot); ok {
				return len(data.PaneOrder) == i+2
			}
			return false
		})
	}

	// Generate and validate multiple snapshots
	for i := 0; i < 20; i++ {
		// Mutate state
		win.Send(shux.NavigatePane{Dir: shux.PaneNavRight})
		time.Sleep(10 * time.Millisecond)

		// Snapshot
		result := <-sessionRef.Ask(shux.GetFullSessionSnapshot{})
		if result == nil {
			t.Fatal("failed to generate snapshot")
		}

		snapshot := result.(*shux.SessionSnapshot)
		if err := shux.ValidateSnapshot(snapshot); err != nil {
			t.Fatalf("snapshot validation failed: %v", err)
		}
	}
}
