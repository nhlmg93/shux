package gomux

import (
	"testing"
	"time"

	"github.com/nhlmg93/gotor/actor"
)

// TestSupervisor is a simple supervisor for testing
type TestSupervisor struct {
	quitChan chan struct{}
}

func NewTestSupervisor() *TestSupervisor {
	return &TestSupervisor{
		quitChan: make(chan struct{}),
	}
}

func (s *TestSupervisor) Receive(msg any) {
	switch msg.(type) {
	case SessionEmpty:
		close(s.quitChan)
	}
}

func (s *TestSupervisor) WaitForQuit() {
	<-s.quitChan
}

// Test creating and switching panes
func TestIntegrationCreateAndSwitchPanes(t *testing.T) {
	supervisor := NewTestSupervisor()
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
	supervisor := NewTestSupervisor()
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
	supervisor := NewTestSupervisor()
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
	supervisor := NewTestSupervisor()
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
	supervisor := NewTestSupervisor()
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

// TestGetGridBeforeWindowCreated verifies we get nil when no window exists
func TestGetGridBeforeWindowCreated(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)

	// Try to get grid before any window is created
	reply := sessionRef.Ask(GetGrid{})
	result := <-reply
	if result != nil {
		t.Error("Expected nil grid when no window exists")
	}
}

// TestGridUpdatedFlow verifies GridUpdated flows through the chain
func TestGridUpdatedFlow(t *testing.T) {
	gridUpdatedReceived := make(chan bool, 1)

	// Create a supervisor that captures GridUpdated
	testSuper := &TestGridSupervisor{
		gridChan: gridUpdatedReceived,
	}
	superRef := actor.Spawn(testSuper, 10)
	defer superRef.Stop()

	sessionRef := SpawnSessionActor(1, superRef)
	sessionRef.Send(CreateWindow{})

	// Wait for grid update
	select {
	case <-gridUpdatedReceived:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("Timeout waiting for GridUpdated")
	}
}

type TestGridSupervisor struct {
	gridChan chan bool
}

func (t *TestGridSupervisor) Receive(msg any) {
	switch msg.(type) {
	case GridUpdated:
		t.gridChan <- true
	}
}

// Test escape sequence handling - clear screen
func TestEscapeSequenceClearScreen(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{})
	time.Sleep(100 * time.Millisecond)

	// Get pane and write some content
	paneReply := sessionRef.Ask(GetActivePane{})
	paneRef := <-paneReply
	if paneRef == nil {
		t.Fatal("Expected active pane")
	}

	// Test clear screen with a fresh grid
	grid := NewGrid(10, 5)
	grid.WriteChar('h')
	grid.WriteChar('e')
	grid.WriteChar('l')
	grid.WriteChar('l')
	grid.WriteChar('o')

	if grid.GetRow(0)[:5] != "hello" {
		t.Error("Expected 'hello' in grid before clear")
	}

	// Process clear escape sequence
	p := &PaneActor{grid: grid}
	p.processBytes([]byte{0x1b, '[', '2', 'J'})

	// Verify grid is cleared
	row := grid.GetRow(0)
	for i, ch := range row {
		if i < 5 && ch != ' ' && ch != 0 {
			t.Errorf("Expected row to be cleared after ESC[2J, got %q at pos %d", string(ch), i)
			break
		}
	}
}

// Test backspace handling
func TestBackspaceHandling(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{})
	time.Sleep(100 * time.Millisecond)

	paneReply := sessionRef.Ask(GetActivePane{})
	paneRef := <-paneReply
	if paneRef == nil {
		t.Fatal("Expected active pane")
	}

	// Test backspace with a fresh grid
	grid := NewGrid(10, 5)
	grid.WriteChar('h')
	grid.WriteChar('i')

	// Process backspace
	p := &PaneActor{grid: grid}
	p.processBytes([]byte{0x08}) // BS

	// Verify 'i' was deleted
	if grid.Cells[0][0].Char != 'h' {
		t.Errorf("Expected 'h' at position 0 after backspace, got %q", string(grid.Cells[0][0].Char))
	}
	if grid.Cells[0][1].Char != ' ' && grid.Cells[0][1].Char != 0 {
		t.Errorf("Expected position 1 to be space after backspace, got %q", string(grid.Cells[0][1].Char))
	}
}

// Test Ctrl+C handling (sent but not displayed)
func TestCtrlCHandling(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{})
	time.Sleep(100 * time.Millisecond)

	paneReply := sessionRef.Ask(GetActivePane{})
	paneRef := <-paneReply
	if paneRef == nil {
		t.Fatal("Expected active pane")
	}

	// Send Ctrl+C (0x03) - should be processed but not displayed
	paneRef.(*actor.Ref).Send(WriteToPane{Data: []byte{0x03}})
	time.Sleep(50 * time.Millisecond)

	// Grid should be empty (Ctrl+C produces no visible output)
	gridReply := sessionRef.Ask(GetGrid{})
	result := <-gridReply
	if result == nil {
		t.Fatal("Expected grid")
	}
	grid := result.(*Grid)
	// Just verify grid exists and no control char was written
	if grid.Cells[0][0].Char == 0x03 {
		t.Error("Ctrl+C (0x03) should not appear in grid")
	}
}

// Test writing to pane updates grid
func TestIntegrationWriteToPaneUpdatesGrid(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{})
	time.Sleep(100 * time.Millisecond)

	// Write directly to pane
	paneReply := sessionRef.Ask(GetActivePane{})
	paneRef := <-paneReply
	if paneRef == nil {
		t.Fatal("Expected active pane")
	}

	paneRef.(*actor.Ref).Send(WriteToPane{Data: []byte("hello")})
	time.Sleep(100 * time.Millisecond)

	// Verify grid was updated
	gridReply := sessionRef.Ask(GetGrid{})
	result := <-gridReply
	if result == nil {
		t.Fatal("Expected grid")
	}

	grid := result.(*Grid)
	found := false
	for i := 0; i < grid.Height; i++ {
		row := grid.GetRow(i)
		for _, ch := range row {
			if ch == 'h' || ch == 'e' || ch == 'l' || ch == 'o' {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Error("Expected to find 'hello' characters in grid")
	}
}

// Test GetGrid chain from session to pane
func TestIntegrationGetGrid(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{})
	time.Sleep(50 * time.Millisecond)

	// Get the pane ref to write to it
	paneReply := sessionRef.Ask(GetActivePane{})
	paneRef := <-paneReply
	if paneRef == nil {
		t.Fatal("Expected active pane")
	}

	// Write some text to the pane
	paneRef.(*actor.Ref).Send(WriteToPane{Data: []byte("hi")})
	time.Sleep(100 * time.Millisecond)

	// Get grid through the chain
	gridReply := sessionRef.Ask(GetGrid{})
	result := <-gridReply
	if result == nil {
		t.Fatal("Expected to get grid through session chain")
	}

	grid := result.(*Grid)
	if grid == nil {
		t.Fatal("Result should be a *Grid")
	}

	// Check that grid was populated (may contain shell prompt or echoed text)
	foundContent := false
	for i := 0; i < grid.Height; i++ {
		row := grid.GetRow(i)
		for _, ch := range row {
			if ch == 'h' || ch == 'i' {
				foundContent = true
				break
			}
		}
		if foundContent {
			break
		}
	}
	if !foundContent {
		t.Error("Expected to find 'h' or 'i' characters in grid after writing to pane")
	}
}
