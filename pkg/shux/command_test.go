package shux

import (
	"strings"
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantArgs []string
		wantErr  bool
	}{
		{
			name:     "simple command",
			input:    "new-window",
			wantName: "new-window",
			wantArgs: nil,
			wantErr:  false,
		},
		{
			name:     "command with args",
			input:    "select-window 5",
			wantName: "select-window",
			wantArgs: []string{"5"},
			wantErr:  false,
		},
		{
			name:     "command with flag",
			input:    "split-window -h",
			wantName: "split-window",
			wantArgs: []string{"-h"},
			wantErr:  false,
		},
		{
			name:     "command with multiple args",
			input:    "resize-pane -L 10",
			wantName: "resize-pane",
			wantArgs: []string{"-L", "10"},
			wantErr:  false,
		},
		{
			name:     "quoted argument",
			input:    `rename-window "my window"`,
			wantName: "rename-window",
			wantArgs: []string{"my window"},
			wantErr:  false,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseCommand(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCommand(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if cmd.Name != tt.wantName {
				t.Errorf("ParseCommand(%q) name = %q, want %q", tt.input, cmd.Name, tt.wantName)
			}
			if len(cmd.Args) != len(tt.wantArgs) {
				t.Errorf("ParseCommand(%q) args = %v, want %v", tt.input, cmd.Args, tt.wantArgs)
				return
			}
			for i, arg := range cmd.Args {
				if arg != tt.wantArgs[i] {
					t.Errorf("ParseCommand(%q) arg[%d] = %q, want %q", tt.input, i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestCommandToActionMsg(t *testing.T) {
	tests := []struct {
		name       string
		cmd        Command
		wantAction Action
		wantAmount int
		wantArgs   []string
		wantOk     bool
	}{
		{
			name:       "new-window",
			cmd:        Command{Name: "new-window"},
			wantAction: ActionNewWindow,
			wantOk:     true,
		},
		{
			name:       "next-window",
			cmd:        Command{Name: "next-window"},
			wantAction: ActionNextWindow,
			wantOk:     true,
		},
		{
			name:       "previous-window",
			cmd:        Command{Name: "previous-window"},
			wantAction: ActionPrevWindow,
			wantOk:     true,
		},
		{
			name:       "prev-window alias",
			cmd:        Command{Name: "prev-window"},
			wantAction: ActionPrevWindow,
			wantOk:     true,
		},
		{
			name:       "select-window",
			cmd:        Command{Name: "select-window", Args: []string{"3"}},
			wantAction: ActionSelectWindow3,
			wantOk:     true,
		},
		{
			name:       "last-window",
			cmd:        Command{Name: "last-window"},
			wantAction: ActionLastWindow,
			wantOk:     true,
		},
		{
			name:       "kill-window",
			cmd:        Command{Name: "kill-window"},
			wantAction: ActionKillWindow,
			wantOk:     true,
		},
		{
			name:       "rename-window",
			cmd:        Command{Name: "rename-window", Args: []string{"mywin"}},
			wantAction: ActionRenameWindow,
			wantArgs:   []string{"mywin"},
			wantOk:     true,
		},
		{
			name:       "split-window -h",
			cmd:        Command{Name: "split-window", Args: []string{"-h"}},
			wantAction: ActionSplitVertical,
			wantOk:     true,
		},
		{
			name:       "split-window -v",
			cmd:        Command{Name: "split-window", Args: []string{"-v"}},
			wantAction: ActionSplitHorizontal,
			wantOk:     true,
		},
		{
			name:       "select-pane -L",
			cmd:        Command{Name: "select-pane", Args: []string{"-L"}},
			wantAction: ActionSelectPaneLeft,
			wantOk:     true,
		},
		{
			name:       "select-pane -R",
			cmd:        Command{Name: "select-pane", Args: []string{"-R"}},
			wantAction: ActionSelectPaneRight,
			wantOk:     true,
		},
		{
			name:       "select-pane -U",
			cmd:        Command{Name: "select-pane", Args: []string{"-U"}},
			wantAction: ActionSelectPaneUp,
			wantOk:     true,
		},
		{
			name:       "select-pane -D",
			cmd:        Command{Name: "select-pane", Args: []string{"-D"}},
			wantAction: ActionSelectPaneDown,
			wantOk:     true,
		},
		{
			name:       "kill-pane",
			cmd:        Command{Name: "kill-pane"},
			wantAction: ActionKillPane,
			wantOk:     true,
		},
		{
			name:       "resize-pane -L",
			cmd:        Command{Name: "resize-pane", Args: []string{"-L"}},
			wantAction: ActionResizePaneLeft,
			wantAmount: 1,
			wantOk:     true,
		},
		{
			name:       "resize-pane -R 10",
			cmd:        Command{Name: "resize-pane", Args: []string{"-R", "10"}},
			wantAction: ActionResizePaneRight,
			wantAmount: 10,
			wantOk:     true,
		},
		{
			name:       "resize-pane -Z",
			cmd:        Command{Name: "resize-pane", Args: []string{"-Z"}},
			wantAction: ActionZoomPane,
			wantOk:     true,
		},
		{
			name:       "swap-pane -U",
			cmd:        Command{Name: "swap-pane", Args: []string{"-U"}},
			wantAction: ActionSwapPaneUp,
			wantOk:     true,
		},
		{
			name:       "swap-pane -D",
			cmd:        Command{Name: "swap-pane", Args: []string{"-D"}},
			wantAction: ActionSwapPaneDown,
			wantOk:     true,
		},
		{
			name:       "rename-session",
			cmd:        Command{Name: "rename-session", Args: []string{"mysession"}},
			wantAction: ActionRenameSession,
			wantArgs:   []string{"mysession"},
			wantOk:     true,
		},
		{
			name:       "list-sessions",
			cmd:        Command{Name: "list-sessions"},
			wantAction: ActionListSessions,
			wantOk:     true,
		},
		{
			name:       "attach-session",
			cmd:        Command{Name: "attach-session", Args: []string{"mysession"}},
			wantAction: ActionAttachSession,
			wantArgs:   []string{"mysession"},
			wantOk:     true,
		},
		{
			name:       "choose-tree -s",
			cmd:        Command{Name: "choose-tree", Args: []string{"-s"}},
			wantAction: ActionChooseTreeSessions,
			wantOk:     true,
		},
		{
			name:       "choose-tree -w",
			cmd:        Command{Name: "choose-tree", Args: []string{"-w"}},
			wantAction: ActionChooseTreeWindows,
			wantOk:     true,
		},
		{
			name:       "detach",
			cmd:        Command{Name: "detach"},
			wantAction: ActionDetach,
			wantOk:     true,
		},
		{
			name:       "detach-client alias",
			cmd:        Command{Name: "detach-client"},
			wantAction: ActionDetach,
			wantOk:     true,
		},
		{
			name:       "list-keys",
			cmd:        Command{Name: "list-keys"},
			wantAction: ActionShowHelp,
			wantOk:     true,
		},
		{
			name:       "quit",
			cmd:        Command{Name: "quit"},
			wantAction: ActionQuit,
			wantOk:     true,
		},
		// Error cases
		{
			name:   "unknown command",
			cmd:    Command{Name: "unknown-cmd"},
			wantOk: false,
		},
		{
			name:   "select-window without index",
			cmd:    Command{Name: "select-window"},
			wantOk: false,
		},
		{
			name:   "select-window invalid index",
			cmd:    Command{Name: "select-window", Args: []string{"abc"}},
			wantOk: false,
		},
		{
			name:   "resize-pane invalid direction",
			cmd:    Command{Name: "resize-pane", Args: []string{"-X"}},
			wantOk: false,
		},
		{
			name:   "select-pane without direction",
			cmd:    Command{Name: "select-pane"},
			wantOk: false,
		},
		{
			name:   "select-pane invalid direction",
			cmd:    Command{Name: "select-pane", Args: []string{"-X"}},
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, ok := tt.cmd.ToActionMsg()
			if ok != tt.wantOk {
				t.Errorf("ToActionMsg() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if !tt.wantOk {
				return
			}
			if msg.Action != tt.wantAction {
				t.Errorf("ToActionMsg() action = %q, want %q", msg.Action, tt.wantAction)
			}
			if msg.Amount != tt.wantAmount {
				t.Errorf("ToActionMsg() amount = %d, want %d", msg.Amount, tt.wantAmount)
			}
			if len(msg.Args) != len(tt.wantArgs) {
				t.Errorf("ToActionMsg() args = %v, want %v", msg.Args, tt.wantArgs)
				return
			}
			for i, arg := range msg.Args {
				if arg != tt.wantArgs[i] {
					t.Errorf("ToActionMsg() arg[%d] = %q, want %q", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestCommandRegistry(t *testing.T) {
	registry := NewCommandRegistry()

	// Test that all v0.1.0 commands are registered
	v1Commands := []string{
		"new-window", "next-window", "previous-window", "select-window",
		"last-window", "kill-window", "kill-session", "rename-window", "split-window",
		"select-pane", "kill-pane", "resize-pane", "swap-pane",
		"rename-session", "list-sessions", "attach-session", "choose-tree",
		"detach", "list-keys", "quit",
	}

	for _, name := range v1Commands {
		info, ok := registry.Get(name)
		if !ok {
			t.Errorf("Command %q not found in registry", name)
			continue
		}
		if info.Name != name {
			t.Errorf("Command name = %q, want %q", info.Name, name)
		}
		if info.Description == "" {
			t.Errorf("Command %q has no description", name)
		}
	}

	// Test List returns all commands
	all := registry.List()
	if len(all) != len(v1Commands) {
		t.Errorf("registry.List() returned %d commands, want %d", len(all), len(v1Commands))
	}

	// Test FormatHelp doesn't panic and contains expected content
	help := registry.FormatHelp()
	if help == "" {
		t.Error("FormatHelp() returned empty string")
	}
	if !strings.Contains(help, "new-window") {
		t.Error("FormatHelp() doesn't contain 'new-window'")
	}
}

func TestValidCommands(t *testing.T) {
	commands := ValidCommands()
	if len(commands) == 0 {
		t.Error("ValidCommands() returned empty slice")
	}

	// Check that all commands are unique
	seen := make(map[string]bool)
	for _, name := range commands {
		if seen[name] {
			t.Errorf("Duplicate command name: %q", name)
		}
		seen[name] = true
	}
}

func TestFullCommandParsing(t *testing.T) {
	// Test that ParseCommand + ToActionMsg round-trip works for all valid commands
	registry := NewCommandRegistry()

	for _, info := range registry.List() {
		// Skip commands that require complex arguments for now
		if len(info.Args) > 0 && info.Args[0].Required {
			continue
		}

		cmd, err := ParseCommand(info.Name)
		if err != nil {
			t.Errorf("ParseCommand(%q) failed: %v", info.Name, err)
			continue
		}

		_, ok := cmd.ToActionMsg()
		if !ok {
			t.Errorf("ToActionMsg() failed for %q", info.Name)
		}
	}
}
