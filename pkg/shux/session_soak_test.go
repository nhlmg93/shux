package shux

import (
	"math/rand"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSessionSoak performs a rapid stress test of session operations.
// Runs create/split/resize/navigate/kill/restore cycles to detect deadlocks
// and race conditions.
func TestSessionSoak(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping soak test in short mode")
	}

	timeout := 30 * time.Second
	if deadline, ok := t.Deadline(); ok {
		remaining := time.Until(deadline) - time.Second
		if remaining < timeout {
			timeout = remaining
		}
	}

	done := make(chan struct{})
	var wg sync.WaitGroup

	// Track operations
	var opsCount atomic.Int32

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)

		for i := 0; i < 5; i++ { // 5 iterations
			func() {
				super := newTestSupervisor()
				logger := &StdLogger{Logger}
				sessionRef := StartSession(1, super.handle, logger)
				defer sessionRef.Shutdown()

				// Initial window
				sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
				if !super.waitContentUpdated(500 * time.Millisecond) {
					t.Error("Timeout waiting for initial window")
					return
				}

				win := requireWindow(t, sessionRef, super)

				// Perform rapid operations
				for j := 0; j < 20; j++ {
					op := rand.Intn(8)
					switch op {
					case 0, 1: // Split
						dir := SplitH
						if rand.Intn(2) == 0 {
							dir = SplitV
						}
						win.Send(Split{Dir: dir})
						opsCount.Add(1)

					case 2, 3: // Resize rapidly
						rows := 10 + rand.Intn(20)
						cols := 40 + rand.Intn(40)
						win.Send(ResizeMsg{Rows: rows, Cols: cols})
						opsCount.Add(1)

					case 4: // Navigate panes
						dir := PaneNavDir(rand.Intn(4))
						win.Send(NavigatePane{Dir: dir})
						opsCount.Add(1)

					case 5: // Switch panes
						idx := rand.Intn(5) // May be out of bounds
						win.Send(SwitchToPane{Index: idx})
						opsCount.Add(1)

					case 6: // Resize pane
						dir := PaneNavDir(rand.Intn(4))
						amount := 1 + rand.Intn(3)
						win.Send(ResizePane{Dir: dir, Amount: amount})
						opsCount.Add(1)

					case 7: // Switch windows (on session)
						delta := rand.Intn(3) - 1 // -1, 0, 1
						sessionRef.Send(SwitchWindow{Delta: delta})
						opsCount.Add(1)
					}

					time.Sleep(10 * time.Millisecond)
				}

				// Verify state is consistent
				reply := sessionRef.Ask(GetActiveWindow{})
				result := <-reply
				if result == nil && i > 0 {
					t.Error("No active window after soak operations")
				}

				// Create additional windows
				for k := 0; k < 3; k++ {
					sessionRef.Send(CreateWindow{Rows: 20, Cols: 60})
					time.Sleep(50 * time.Millisecond)
				}

				// Detach and verify clean shutdown
				reply = sessionRef.Ask(DetachSession{})
				select {
				case <-reply:
					// Detach started
				case <-time.After(2 * time.Second):
					t.Error("Detach timeout")
				}

				// Wait for session empty
				if !super.waitSessionEmpty(3 * time.Second) {
					t.Error("Timeout waiting for session empty")
				}
			}()
		}
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		// Success
	case <-time.After(timeout):
		t.Fatalf("Soak test timed out after %v (completed %d ops)", timeout, opsCount.Load())
	}

	wg.Wait()

	t.Logf("Session soak completed %d operations", opsCount.Load())
}

// TestGoroutineLeak verifies no goroutine leaks after session lifecycle.
func TestGoroutineLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping leak test in short mode")
	}
	if os.Getenv("SHUX_RUN_LEAK_TESTS") == "" {
		t.Skip("Skipping flaky goroutine leak check by default; set SHUX_RUN_LEAK_TESTS=1 to enable")
	}

	runLifecycle := func(id uint32) {
		t.Helper()
		super := newTestSupervisor()
		logger := &StdLogger{Logger}
		sessionRef := StartSession(id, super.handle, logger)

		sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
		time.Sleep(100 * time.Millisecond)

		reply := sessionRef.Ask(DetachSession{})
		<-reply

		if !super.waitSessionEmpty(2 * time.Second) {
			t.Fatal("Timeout waiting for session empty")
		}
		sessionRef.Shutdown()
		time.Sleep(100 * time.Millisecond)
	}

	// Warm up one-time runtime/libghostty goroutines before measuring a baseline.
	runLifecycle(99)
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	for i := 0; i < 3; i++ {
		runLifecycle(uint32(i + 100))
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	final := runtime.NumGoroutine()

	leakThreshold := baseline + 4
	if final > leakThreshold {
		t.Errorf("Possible goroutine leak: baseline=%d final=%d (threshold=%d)",
			baseline, final, leakThreshold)
	}
}

// TestRaceConditions runs operations that might trigger race detector.
func TestRaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race test in short mode")
	}

	super := newTestSupervisor()
	logger := &StdLogger{Logger}
	sessionRef := StartSession(1, super.handle, logger)
	defer sessionRef.Shutdown()

	// Create window with multiple panes
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	if !super.waitContentUpdated(500 * time.Millisecond) {
		t.Fatal("Timeout waiting for window")
	}

	win := requireWindow(t, sessionRef, super)

	// Create multiple panes
	for i := 0; i < 3; i++ {
		win.Send(Split{Dir: SplitV})
		time.Sleep(50 * time.Millisecond)
	}

	// Concurrent operations from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				switch j % 4 {
				case 0:
					win.Send(NavigatePane{Dir: PaneNavRight})
				case 1:
					win.Send(ResizePane{Dir: PaneNavDown, Amount: 1})
				case 2:
					sessionRef.Ask(GetActivePane{})
				case 3:
					sessionRef.Ask(GetWindowView{})
				}
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Verify final state is valid
	reply := sessionRef.Ask(GetActiveWindow{})
	if result := <-reply; result == nil {
		t.Error("No active window after concurrent operations")
	}
}
