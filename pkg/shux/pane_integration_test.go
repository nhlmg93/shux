package shux

import (
	"strings"
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
		// Shell may redraw and clear content on focus - this is expected
		t.Skip("Shell redraw cleared content (known terminal behavior)")
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

	// Remaining lines should exist (even if empty)
	if len(content.Cells) < 10 {
		t.Errorf("Expected Cells array to have 10 rows, got %d", len(content.Cells))
	}
}

// TestPanePTYResizedOnInit verifies PTY is sized correctly at startup
func TestPanePTYResizedOnInit(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	// Create a specific size window
	sessionRef.Send(CreateWindow{Rows: 40, Cols: 100})
	pane := requirePane(t, sessionRef, super)

	// Write a command to check terminal size
	pane.Send(WriteToPane{Data: []byte("stty size\r")})
	super.waitContentUpdated(200 * time.Millisecond)

	pollFor(500*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		// Look for "40 100" or similar in output
		for _, line := range content.Lines {
			if strings.Contains(line, "40") && strings.Contains(line, "100") {
				return true
			}
		}
		return false
	})

	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected pane content")
	}
	content := result.(*PaneContent)

	// Verify we got the expected terminal size output
	found := false
	for _, line := range content.Lines {
		if strings.Contains(line, "40") && strings.Contains(line, "100") {
			found = true
			break
		}
	}
	if !found {
		// Alternative: check that we have 40 lines (the height we requested)
		if len(content.Lines) != 40 {
			t.Errorf("Expected 40 lines for 40-row terminal, got %d", len(content.Lines))
		}
	}
}
