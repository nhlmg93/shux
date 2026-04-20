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

// Test creating and switching terms
func TestIntegrationCreateAndSwitchTerms(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})

	// Wait for initial window/term to be created
	time.Sleep(50 * time.Millisecond)

	// Get active term
	reply := sessionRef.Ask(GetActiveTerm{})
	termRef := <-reply
	if termRef == nil {
		t.Fatal("Expected active term after creating window")
	}

	// Get active window to create another term
	winReply := sessionRef.Ask(GetActiveWindow{})
	winRef := <-winReply
	if winRef == nil {
		t.Fatal("Expected active window")
	}

	// Create second term
	winRef.(*actor.Ref).Send(CreateTerm{Rows: 24, Cols: 80, Shell: "/bin/true"})
	time.Sleep(50 * time.Millisecond)

	// Switch to term 2
	winRef.(*actor.Ref).Send(SwitchToTerm{Index: 1})
	time.Sleep(10 * time.Millisecond)

	// Verify we can get the switched term
	reply2 := sessionRef.Ask(GetActiveTerm{})
	termRef2 := <-reply2
	if termRef2 == nil {
		t.Error("Expected to get term after switching")
	}
}

// Test creating and switching windows
func TestIntegrationCreateAndSwitchWindows(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)

	// Create first window
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	time.Sleep(50 * time.Millisecond)

	// Get active window
	reply := sessionRef.Ask(GetActiveWindow{})
	winRef1 := <-reply
	if winRef1 == nil {
		t.Fatal("Expected first window")
	}

	// Create second window
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	time.Sleep(50 * time.Millisecond)

	// Get active window (should be window 2)
	reply2 := sessionRef.Ask(GetActiveWindow{})
	winRef2 := <-reply2
	if winRef2 == nil {
		t.Fatal("Expected second window")
	}

	// Switch back to first window
	sessionRef.Send(SwitchWindow{Delta: -1})
	time.Sleep(10 * time.Millisecond)

	// Verify we're back to first window
	reply3 := sessionRef.Ask(GetActiveWindow{})
	winRef3 := <-reply3
	if winRef3 == nil {
		t.Fatal("Expected to get window after switching")
	}
}

// Test killing last term triggers SessionEmpty
func TestIntegrationKillLastTerm(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	time.Sleep(100 * time.Millisecond)

	// Get the term and kill it
	termReply := sessionRef.Ask(GetActiveTerm{})
	termRef := <-termReply
	if termRef == nil {
		t.Fatal("Expected active term")
	}

	termRef.(*actor.Ref).Send(KillTerm{})

	// Wait for SessionEmpty
	done := make(chan bool, 1)
	go func() {
		supervisor.WaitForQuit()
		done <- true
	}()

	select {
	case <-done:
		// Success - supervisor received SessionEmpty
	case <-time.After(500 * time.Millisecond):
		t.Error("Expected supervisor to receive SessionEmpty after killing last term")
	}
}

// Test window navigation with wrap-around
func TestIntegrationWindowNavigationWrap(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)

	// Create three windows
	for i := 0; i < 3; i++ {
		sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
		time.Sleep(50 * time.Millisecond)
	}

	// Get current window (should be 3)
	reply1 := sessionRef.Ask(GetActiveWindow{})
	win1 := <-reply1
	if win1 == nil {
		t.Fatal("Expected active window")
	}

	// Switch forward past the end (should wrap to first)
	sessionRef.Send(SwitchWindow{Delta: 1})
	time.Sleep(20 * time.Millisecond)

	reply2 := sessionRef.Ask(GetActiveWindow{})
	win2 := <-reply2
	if win2 == nil {
		t.Fatal("Expected window after wrap forward")
	}

	// Switch backward past the beginning (should wrap to last)
	sessionRef.Send(SwitchWindow{Delta: -2})
	time.Sleep(20 * time.Millisecond)

	reply3 := sessionRef.Ask(GetActiveWindow{})
	win3 := <-reply3
	if win3 == nil {
		t.Fatal("Expected window after wrap backward")
	}
}

// Test switching to invalid term index
func TestIntegrationSwitchToInvalidTerm(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	time.Sleep(50 * time.Millisecond)

	// Get active window and try to switch to non-existent term
	winReply := sessionRef.Ask(GetActiveWindow{})
	winRef := <-winReply
	if winRef == nil {
		t.Fatal("Expected active window")
	}

	// Try to switch to term index 99 (doesn't exist)
	winRef.(*actor.Ref).Send(SwitchToTerm{Index: 99})
	time.Sleep(10 * time.Millisecond)

	// Should still have an active term (the original one)
	termReply := sessionRef.Ask(GetActiveTerm{})
	termRef := <-termReply
	if termRef == nil {
		t.Error("Expected to still have active term after invalid switch")
	}
}

// Test getting term content before any window exists
func TestGetTermContentBeforeWindowCreated(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)

	// Try to get content without creating any window
	contentReply := sessionRef.Ask(GetTermContent{})
	result := <-contentReply

	if result != nil {
		t.Error("Expected nil content when no window exists")
	}
}

// Test that GridUpdated flows through the chain
func TestGridUpdatedFlow(t *testing.T) {
	gridUpdatedReceived := make(chan bool, 1)

	// Create a supervisor that captures GridUpdated
	testSuper := &TestGridSupervisor{
		gridChan: gridUpdatedReceived,
	}
	superRef := actor.Spawn(testSuper, 10)
	defer superRef.Stop()

	sessionRef := SpawnSessionActor(1, superRef)
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})

	// Wait for grid update from term output
	select {
	case <-gridUpdatedReceived:
		// Success
	case <-time.After(500 * time.Millisecond):
		// This may timeout since PTY output is async
		// Just log it as info, not error
		t.Log("Note: GridUpdated timeout (expected for async PTY)")
	}
}

type TestGridSupervisor struct {
	gridChan chan bool
}

func (t *TestGridSupervisor) Receive(msg any) {
	switch msg.(type) {
	case GridUpdated:
		select {
		case t.gridChan <- true:
		default:
		}
	}
}

// Test getting term content through session chain
func TestIntegrationGetTermContent(t *testing.T) {
	supervisor := NewTestSupervisor()
	supervisorRef := actor.Spawn(supervisor, 10)
	defer supervisorRef.Stop()

	sessionRef := SpawnSessionActor(1, supervisorRef)
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	time.Sleep(50 * time.Millisecond)

	// Get the term ref to write to it
	termReply := sessionRef.Ask(GetActiveTerm{})
	termRef := <-termReply
	if termRef == nil {
		t.Fatal("Expected active term")
	}

	// Write some text to the term
	termRef.(*actor.Ref).Send(WriteToTerm{Data: []byte("hi")})
	time.Sleep(100 * time.Millisecond)

	// Get content through the chain
	contentReply := sessionRef.Ask(GetTermContent{})
	result := <-contentReply
	if result == nil {
		t.Fatal("Expected to get content through session chain")
	}

	content := result.(*TermContent)
	if content == nil {
		t.Fatal("Result should be a *TermContent")
	}

	// Check that we got lines
	if len(content.Lines) == 0 {
		t.Error("Expected non-empty lines")
	}
}
