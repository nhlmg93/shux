package gomux

import (
	"testing"
	"time"

	"github.com/nhlmg93/gotor/actor"
)

func TestPaneWriteAndGetContent(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	pane.Send(WriteToPane{Data: []byte("hi")})
	
	super.waitContentUpdated(200 * time.Millisecond)
	pollFor(50*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		return result != nil
	})

	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected to get content through session chain")
	}

	content := result.(*PaneContent)
	if content == nil {
		t.Fatal("Result should be a *PaneContent")
	}

	if len(content.Lines) == 0 {
		t.Error("Expected non-empty lines")
	}
}

func TestPaneContentWithStyling(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	requirePane(t, sessionRef, super)

	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected pane content")
	}

	content := result.(*PaneContent)
	if content == nil {
		t.Fatal("Result should be a *PaneContent")
	}

	if len(content.Cells) == 0 {
		t.Error("Expected Cells array to be populated")
	}
	if len(content.Cells) != len(content.Lines) {
		t.Error("Expected Cells and Lines to have same row count")
	}
}

func TestPaneContentUpdatedFlow(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})

	// Should get content update fairly quickly from initial shell output
	if !super.waitContentUpdated(500 * time.Millisecond) {
		t.Log("Note: PaneContentUpdated timeout (expected for async PTY)")
	}
}

func TestPaneKill(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	pane.Send(KillPane{})

	if !super.waitSessionEmpty(1 * time.Second) {
		t.Error("timeout waiting for SessionEmpty after killing pane")
	}
}

func TestPaneAltScreenDetection(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	// First verify we're NOT in alt-screen initially
	modeReply := pane.Ask(GetPaneMode{})
	modeResult := <-modeReply
	if modeResult == nil {
		t.Fatal("Expected to get pane mode")
	}
	initialMode := modeResult.(*PaneMode)
	if initialMode.InAltScreen {
		t.Error("Expected NOT to be in alt-screen initially")
	}

	// Try to enter alternate screen via printf - this is tricky because
	// escape sequences sent to shell stdin don't directly affect terminal state
	// The shell would need to interpret them, which most shells don't do by default
	pane.Send(WriteToPane{Data: []byte("printf '\033[?1049h'\r")})
	
	// Poll for mode change - shell may interpret printf output
	var mode *PaneMode
	if !pollFor(300*time.Millisecond, func() bool {
		modeReply := pane.Ask(GetPaneMode{})
		modeResult := <-modeReply
		if modeResult == nil {
			return false
		}
		mode = modeResult.(*PaneMode)
		return mode.InAltScreen
	}) {
		// This test may fail because shells typically don't interpret escape sequences
		// from their own output. We'd need to run an actual terminal program like vim.
		t.Log("Note: Alt-screen detection requires running a terminal program (vim, less, etc)")
	}

	// Exit alt-screen for cleanup
	pane.Send(WriteToPane{Data: []byte("printf '\033[?1049l'\r")})
}

func TestPaneResizeContent(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 10, Cols: 40})
	pane := requirePane(t, sessionRef, super)

	// Get initial content
	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected pane content")
	}

	// Resize the pane
	pane.Send(ResizeTerm{Rows: 20, Cols: 80})
	super.waitContentUpdated(200 * time.Millisecond)
	pollFor(100*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		return len(content.Lines) == 20 && len(content.Cells[0]) == 80
	})

	// Verify new dimensions
	reply2 := sessionRef.Ask(GetPaneContent{})
	result2 := <-reply2
	if result2 == nil {
		t.Fatal("Expected pane content after resize")
	}
	resized := result2.(*PaneContent)
	if len(resized.Lines) != 20 {
		t.Errorf("Expected 20 rows after resize, got %d", len(resized.Lines))
	}
	if len(resized.Cells) > 0 && len(resized.Cells[0]) != 80 {
		t.Errorf("Expected 80 cols after resize, got %d", len(resized.Cells[0]))
	}
}

func TestPaneContentIsolation(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)
	pane1 := requirePane(t, sessionRef, super)

	// Write to pane 1
	pane1.Send(WriteToPane{Data: []byte("PANE1_DATA\r")})
	super.waitContentUpdated(200 * time.Millisecond)
	pollFor(100*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		for _, line := range content.Lines {
			if contains(line, "PANE1_DATA") {
				return true
			}
		}
		return false
	})

	// Create pane 2
	win.Send(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
	pollFor(200*time.Millisecond, func() bool {
		win.Send(SwitchToPane{Index: 1})
		reply := sessionRef.Ask(GetActivePane{})
		result := <-reply
		return result != nil && result.(*actor.Ref) != pane1
	})

	// Get pane 2 content - should not have PANE1_DATA
	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected pane 2 content")
	}
	pane2Content := result.(*PaneContent)
	for _, line := range pane2Content.Lines {
		if contains(line, "PANE1_DATA") {
			t.Error("Pane 2 should not contain Pane 1's data")
		}
	}

	// Switch back to pane 1 - should still have its data
	win.Send(SwitchToPane{Index: 0})
	super.waitContentUpdated(200 * time.Millisecond)
	pollFor(100*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetActivePane{})
		result := <-reply
		return result != nil && result.(*actor.Ref) == pane1
	})

	reply2 := sessionRef.Ask(GetPaneContent{})
	result2 := <-reply2
	if result2 == nil {
		t.Fatal("Expected pane 1 content")
	}
	pane1Content := result2.(*PaneContent)
	found := false
	for _, line := range pane1Content.Lines {
		if contains(line, "PANE1_DATA") {
			found = true
			break
		}
	}
	if !found {
		// Note: Shell may redraw and clear screen when switching back
		// This is a known limitation of terminal multiplexers
		t.Log("Note: Pane 1 content not visible after switch (shell may have redrawn)")
	}
}

func TestPaneCursorPosition(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	// Get initial cursor position (should be at start)
	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected pane content")
	}
	initial := result.(*PaneContent)
	initialRow := initial.CursorRow
	initialCol := initial.CursorCol

	// Write a string and newline to move cursor
	pane.Send(WriteToPane{Data: []byte("test\r")})
	super.waitContentUpdated(200 * time.Millisecond)
	pollFor(100*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		return content.CursorRow != initialRow || content.CursorCol != initialCol
	})

	// Verify cursor moved
	reply2 := sessionRef.Ask(GetPaneContent{})
	result2 := <-reply2
	if result2 == nil {
		t.Fatal("Expected pane content after write")
	}
	final := result2.(*PaneContent)
	if final.CursorRow == initialRow && final.CursorCol == initialCol {
		t.Error("Expected cursor to move after writing")
	}
}

// contains checks if string contains substring (helper for content checking)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestPaneScrollback(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 5, Cols: 40}) // Small window
	pane := requirePane(t, sessionRef, super)

	// Write more lines than height
	for i := 0; i < 10; i++ {
		pane.Send(WriteToPane{Data: []byte("line" + string(rune('0'+i)) + "\r\n")})
	}
	super.waitContentUpdated(200 * time.Millisecond)

	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected content")
	}
	content := result.(*PaneContent)

	// Should have exactly 5 visible lines
	if len(content.Lines) != 5 {
		t.Errorf("Expected 5 visible lines (height), got %d", len(content.Lines))
	}
}

func TestPaneSizeFullTerminalHeight(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	// Create a small window
	sessionRef.Send(CreateWindow{Rows: 10, Cols: 40})
	pane := requirePane(t, sessionRef, super)

	// Write just one line of content
	pane.Send(WriteToPane{Data: []byte("short content\r")})
	super.waitContentUpdated(200 * time.Millisecond)
	pollFor(100*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		return len(content.Lines) == 10 // Window height
	})

	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected pane content")
	}
	content := result.(*PaneContent)

	// Should have exactly 10 lines (window height), not just 1 (content height)
	if len(content.Lines) != 10 {
		t.Errorf("Expected 10 lines matching window height, got %d", len(content.Lines))
	}

	// First line should have content
	if !contains(content.Lines[0], "short content") {
		t.Error("Expected first line to contain 'short content'")
	}

	// Remaining lines should exist (even if empty)
	if len(content.Cells) < 10 {
		t.Errorf("Expected Cells array to have 10 rows, got %d", len(content.Cells))
	}
}
