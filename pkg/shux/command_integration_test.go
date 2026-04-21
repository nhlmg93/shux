package shux

import (
	"testing"
)

// TestCommandExecutionViaSession tests that commands can be executed through the session.
func TestCommandExecutionViaSession(t *testing.T) {
	// This is a lightweight test that validates command parsing and conversion
	// without starting a full session.

	tests := []struct {
		name      string
		command   string
		wantQuit  bool
		wantValid bool
	}{
		{"new-window command", "new-window", false, true},
		{"next-window", "next-window", false, true},
		{"previous-window", "previous-window", false, true},
		{"select-window 0", "select-window 0", false, true},
		{"split-window -h", "split-window -h", false, true},
		{"split-window -v", "split-window -v", false, true},
		{"resize-pane -L 5", "resize-pane -L 5", false, true},
		{"resize-pane -Z", "resize-pane -Z", false, true},
		{"kill-pane", "kill-pane", false, true},
		{"swap-pane -U", "swap-pane -U", false, true},
		{"swap-pane -D", "swap-pane -D", false, true},
		{"last-window", "last-window", false, true},
		{"rename-window test", "rename-window test", false, true},
		{"rename-session test", "rename-session test", false, true},
		{"list-sessions", "list-sessions", false, true},
		{"attach-session mysession", "attach-session mysession", false, true},
		{"choose-tree -s", "choose-tree -s", false, true},
		{"choose-tree -w", "choose-tree -w", false, true},
		{"list-keys", "list-keys", false, true},
		{"detach command", "detach", true, true},
		{"quit command", "quit", true, true},
		// Invalid commands
		{"unknown command", "unknown-cmd", false, false},
		{"split-window without flag", "split-window", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseCommand(tt.command)
			if err != nil {
				if tt.wantValid {
					t.Errorf("ParseCommand(%q) failed: %v", tt.command, err)
				}
				return
			}

			msg, ok := cmd.ToActionMsg()
			if ok != tt.wantValid {
				t.Errorf("ToActionMsg() ok = %v, want %v", ok, tt.wantValid)
				return
			}
			if !tt.wantValid {
				return
			}

			// Check if the action indicates quit
			var gotQuit bool
			switch msg.Action {
			case ActionQuit, ActionDetach:
				gotQuit = true
			}

			if gotQuit != tt.wantQuit {
				t.Errorf("command %q quit = %v, want %v", tt.command, gotQuit, tt.wantQuit)
			}
		})
	}
}

// TestCommandRegistryCompleteness ensures all v0.1.0 commands are in the registry.
func TestCommandRegistryCompleteness(t *testing.T) {
	requiredCommands := []string{
		"new-window",
		"next-window",
		"previous-window",
		"select-window",
		"last-window",
		"kill-window",
		"rename-window",
		"split-window",
		"select-pane",
		"kill-pane",
		"resize-pane",
		"swap-pane",
		"rename-session",
		"list-sessions",
		"attach-session",
		"choose-tree",
		"detach",
		"list-keys",
	}

	registry := NewCommandRegistry()

	for _, name := range requiredCommands {
		if _, ok := registry.Get(name); !ok {
			t.Errorf("Required command %q not found in registry", name)
		}
	}
}

// TestCommandParsingEdgeCases tests various edge cases for command parsing.
func TestCommandParsingEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"extra whitespace", "  new-window  ", false},
		{"tabs", "new-window\t", false},
		{"mixed whitespace", "\t  new-window  \t", false},
		{"single quotes", "'rename-window' 'test name'", false},
		{"double quotes", `"rename-window" "test name"`, false},
		{"empty quotes", `""`, true},
		{"quoted with spaces", `rename-window "my test window"`, false},
		{"unclosed quote", `rename-window "test`, false}, // Note: unclosed quotes are tolerated by tokenizer
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCommand(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCommand(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
