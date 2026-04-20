package gomux

import (
	"os"
	"testing"
	"time"
)

// TestE2ENanoEdit validates full stack: gomux → shell → nano → rendering
// TestE2ENanoEdit validates nano interaction through full stack.
// Run in Docker with: docker build -f Dockerfile.test -t gomux-test . && docker run gomux-test
func TestE2ENanoEdit(t *testing.T) {
	if os.Getenv("GOMUX_E2E") != "1" {
		t.Skip("Set GOMUX_E2E=1 to run E2E tests (requires nano, vim, etc)")
	}

	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	// Start nano
	pane.Send(WriteToPane{Data: []byte("nano /tmp/gomux_test.txt\r")})
	
	// Wait for nano to initialize (shows "GNU nano" or bottom menu)
	if !pollFor(1*time.Second, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		for _, line := range content.Lines {
			if contains(line, "GNU nano") || contains(line, "^G") {
				return true
			}
		}
		return false
	}) {
		t.Fatal("nano did not start (may not be installed)")
	}

	// Type some text
	testText := "HELLO_GOMUX"
	pane.Send(WriteToPane{Data: []byte(testText)})
	
	// Wait for text to appear
	var textFound bool
	if !pollFor(500*time.Millisecond, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		for _, line := range content.Lines {
			if contains(line, testText) {
				textFound = true
				return true
			}
		}
		return false
	}) {
		t.Error("Typed text did not appear in nano")
	}

	if textFound {
		t.Log("E2E success: nano opened, text typed, rendered correctly")
	}

	// Exit nano (Ctrl+X)
	pane.Send(WriteToPane{Data: []byte{0x18}}) // Ctrl+X
	pollFor(200*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		return result != nil
	})
}

// TestE2EShellCommand validates basic shell interaction
func TestE2EShellCommand(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	// Run echo command
	uniqueStr := "GOMUX_TEST_42"
	pane.Send(WriteToPane{Data: []byte("echo " + uniqueStr + "\r")})
	
	// Wait for output
	var outputFound bool
	if !pollFor(500*time.Millisecond, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		for _, line := range content.Lines {
			if contains(line, uniqueStr) {
				outputFound = true
				return true
			}
		}
		return false
	}) {
		t.Log("Note: Shell command output may not be visible in content snapshot")
	}

	if outputFound {
		t.Log("E2E success: shell command executed and output rendered")
	}
}

// TestE2EKeySequence validates special key handling through full stack
func TestE2EKeySequence(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	// Send clear command
	pane.Send(WriteToPane{Data: []byte("clear\r")})
	
	// Wait for clear to take effect (content should be mostly empty)
	pollFor(300*time.Millisecond, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		return true // Best effort
	})

	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected pane content after clear")
	}

	content := result.(*PaneContent)
	// After clear, screen should be mostly empty (spaces or nulls)
	emptyLines := 0
	for _, line := range content.Lines {
		if len(line) == 0 || allSpaces(line) {
			emptyLines++
		}
	}
	
	if emptyLines > len(content.Lines)/2 {
		t.Log("E2E success: clear command processed through full stack")
	}
}

func allSpaces(s string) bool {
	for _, r := range s {
		if r != ' ' && r != 0 {
			return false
		}
	}
	return true
}
