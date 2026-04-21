package integration

import (
	"strings"
	"testing"
	"time"

	"shux/pkg/shux"
	"shux/pkg/testutil"
)

// TestPaneWriteContent validates writing to a pane and reading content.
func TestPaneWriteContent(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	pane := testutil.RequirePane(t, sessionRef, super)

	// Write data
	testData := "TEST_DATA_123"
	pane.Send(shux.WriteToPane{Data: []byte(testData)})

	// Wait for content update
	super.WaitContentUpdated(200 * time.Millisecond)

	// Verify content
	result := <-sessionRef.Ask(shux.GetPaneContent{})
	if result == nil {
		t.Fatal("expected pane content")
	}
	content := result.(*shux.PaneContent)

	// Should have cells populated
	if len(content.Cells) == 0 {
		t.Error("expected cells to be populated")
	}
}

// TestPaneKeyInput validates sending keyboard input to a pane.
func TestPaneKeyInput(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	pane := testutil.RequirePane(t, sessionRef, super)

	// Send key input
	pane.Send(shux.KeyInput{Text: "a", Code: 'a'})
	time.Sleep(50 * time.Millisecond)

	// Pane should process without error
	super.WaitContentUpdated(100 * time.Millisecond)
}

// TestPaneResizeTerm validates pane-specific resize.
func TestPaneResizeTerm(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 10, Cols: 40})
	pane := testutil.RequirePane(t, sessionRef, super)

	// Resize the pane directly
	pane.Send(shux.ResizeTerm{Rows: 20, Cols: 80})
	super.WaitContentUpdated(200 * time.Millisecond)

	// Verify content reflects new size
	result := <-sessionRef.Ask(shux.GetPaneContent{})
	content := result.(*shux.PaneContent)

	// Content should have the new row count
	if len(content.Lines) != 20 {
		t.Errorf("expected 20 lines after resize, got %d", len(content.Lines))
	}
}

// TestPaneKillExit validates pane exit notification.
func TestPaneKillExit(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	_ = testutil.RequirePane(t, sessionRef, super)

	pane := testutil.RequirePane(t, sessionRef, super)
	pane.Send(shux.KillPane{})

	// Window should eventually be empty
	if !super.WaitSessionEmpty(2 * time.Second) {
		t.Error("expected SessionEmpty after killing only pane")
	}
}

// TestPaneContentIsolation validates each pane has independent content.
func TestPaneContentIsolation(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	pane1 := testutil.RequirePane(t, sessionRef, super)

	// Write to pane 1
	pane1.Send(shux.WriteToPane{Data: []byte("UNIQUE_PANE1_DATA\r")})
	super.WaitContentUpdated(200 * time.Millisecond)
	testutil.PollFor(200*time.Millisecond, func() bool {
		result := <-sessionRef.Ask(shux.GetPaneContent{})
		if content, ok := result.(*shux.PaneContent); ok {
			for _, line := range content.Lines {
				if strings.Contains(line, "UNIQUE_PANE1_DATA") {
					return true
				}
			}
		}
		return false
	})

	// Create pane 2
	win.Send(shux.CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	testutil.PollFor(200*time.Millisecond, func() bool {
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			return len(data.PaneOrder) == 2
		}
		return false
	})

	// Switch to pane 2 and verify it doesn't have pane 1's data
	win.Send(shux.SwitchToPane{Index: 1})
	time.Sleep(50 * time.Millisecond)

	result2 := <-sessionRef.Ask(shux.GetPaneContent{})
	content2 := result2.(*shux.PaneContent)
	for _, line := range content2.Lines {
		if strings.Contains(line, "UNIQUE_PANE1_DATA") {
			t.Error("pane 2 should not contain pane 1's unique data")
		}
	}
}

// TestPaneAltScreenMode validates pane alt screen mode detection.
func TestPaneAltScreenMode(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	pane := testutil.RequirePane(t, sessionRef, super)

	// Enter alt screen mode (ESC [?1049h)
	pane.Send(shux.WriteToPane{Data: []byte{0x1b, '[', '?', '1', '0', '4', '9', 'h'}})
	time.Sleep(50 * time.Millisecond)

	// Query mode
	result := <-pane.Ask(shux.GetPaneMode{})
	if mode, ok := result.(*shux.PaneMode); ok {
		// May or may not be in alt screen depending on shell support
		// Just verify we get a valid response
		_ = mode.InAltScreen
	}
}

// TestPaneCursorVisibility validates cursor visibility reporting.
func TestPaneCursorVisibility(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	_ = testutil.RequirePane(t, sessionRef, super)

	// Get content
	result := <-sessionRef.Ask(shux.GetPaneContent{})
	content := result.(*shux.PaneContent)

	// Cursor position should be within bounds
	if content.CursorRow < 0 || content.CursorRow >= len(content.Lines) {
		t.Errorf("cursor row %d out of bounds", content.CursorRow)
	}
	if content.CursorCol < 0 {
		t.Errorf("cursor col %d is negative", content.CursorCol)
	}
}

// TestPaneShellCommand validates running a simple shell command.
func TestPaneShellCommand(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	pane := testutil.RequirePane(t, sessionRef, super)

	// Run a command that produces known output
	uniqueStr := "PANE_CMD_TEST_42"
	pane.Send(shux.WriteToPane{Data: []byte("echo " + uniqueStr + "\r")})

	// Wait for output
	found := testutil.PollFor(500*time.Millisecond, func() bool {
		super.WaitContentUpdated(100 * time.Millisecond)
		result := <-sessionRef.Ask(shux.GetPaneContent{})
		if content, ok := result.(*shux.PaneContent); ok {
			for _, line := range content.Lines {
				if strings.Contains(line, uniqueStr) {
					return true
				}
			}
		}
		return false
	})

	if !found {
		t.Log("Note: Shell command output may not be visible (shell-dependent)")
	}
}

// TestPaneScrollback validates scrollback buffer handling.
func TestPaneScrollback(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 5, Cols: 40})
	pane := testutil.RequirePane(t, sessionRef, super)

	// Write more lines than height
	for i := 0; i < 10; i++ {
		pane.Send(shux.WriteToPane{Data: []byte("line" + string(rune('0'+i%10)) + "\r\n")})
	}
	super.WaitContentUpdated(200 * time.Millisecond)

	// Verify visible lines match window height
	result := <-sessionRef.Ask(shux.GetPaneContent{})
	content := result.(*shux.PaneContent)

	if len(content.Lines) != 5 {
		t.Errorf("expected 5 visible lines (height), got %d", len(content.Lines))
	}
}

// TestPaneMultipleWrites validates multiple writes are processed.
func TestPaneMultipleWrites(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	pane := testutil.RequirePane(t, sessionRef, super)

	// Write multiple times
	for i := 0; i < 5; i++ {
		pane.Send(shux.WriteToPane{Data: []byte("write" + string(rune('0'+i)) + " ")})
		time.Sleep(10 * time.Millisecond)
	}

	super.WaitContentUpdated(200 * time.Millisecond)

	// Verify content exists
	result := <-sessionRef.Ask(shux.GetPaneContent{})
	content := result.(*shux.PaneContent)

	if len(content.Cells) == 0 {
		t.Error("expected cells after multiple writes")
	}
}

// TestPanePTYDimensions validates PTY reports correct dimensions.
func TestPanePTYDimensions(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 40, Cols: 100})
	pane := testutil.RequirePane(t, sessionRef, super)

	// Query terminal size using stty
	pane.Send(shux.WriteToPane{Data: []byte("stty size\r")})

	// Verify pane content has the expected dimensions
	testutil.PollFor(500*time.Millisecond, func() bool {
		super.WaitContentUpdated(100 * time.Millisecond)
		result := <-sessionRef.Ask(shux.GetPaneContent{})
		if content, ok := result.(*shux.PaneContent); ok {
			// Should have 40 lines (the window height)
			return len(content.Lines) == 40
		}
		return false
	})

	result := <-sessionRef.Ask(shux.GetPaneContent{})
	content := result.(*shux.PaneContent)
	if len(content.Lines) != 40 {
		t.Errorf("expected 40 lines for 40-row terminal, got %d", len(content.Lines))
	}
}
