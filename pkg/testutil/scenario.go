package testutil

import (
	"fmt"
	"testing"
	"time"

	"shux/pkg/shux"
)

// ScenarioStep represents a single step in a deterministic test scenario.
type ScenarioStep struct {
	Name    string
	Action  func(*shux.SessionRef) error
	Verify  func(*shux.SessionRef) error
	Timeout time.Duration
}

// ScenarioRunner executes a sequence of scenario steps deterministically.
type ScenarioRunner struct {
	Steps   []ScenarioStep
	Session *shux.SessionRef
	Super   *TestSupervisor
}

// NewScenarioRunner creates a new scenario runner.
func NewScenarioRunner(session *shux.SessionRef, super *TestSupervisor) *ScenarioRunner {
	return &ScenarioRunner{
		Steps:   make([]ScenarioStep, 0),
		Session: session,
		Super:   super,
	}
}

// AddStep adds a step to the scenario.
func (r *ScenarioRunner) AddStep(step ScenarioStep) {
	if step.Timeout == 0 {
		step.Timeout = 1 * time.Second
	}
	r.Steps = append(r.Steps, step)
}

// Run executes all steps in sequence, failing on first error.
func (r *ScenarioRunner) Run(t *testing.T) {
	t.Helper()

	for i, step := range r.Steps {
		t.Logf("[SCENARIO] Step %d: %s", i+1, step.Name)

		if step.Action != nil {
			if err := step.Action(r.Session); err != nil {
				t.Fatalf("Step %d (%s) action failed: %v", i+1, step.Name, err)
			}
		}

		if step.Verify != nil {
			done := make(chan error, 1)
			go func() {
				done <- step.Verify(r.Session)
			}()

			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("Step %d (%s) verify failed: %v", i+1, step.Name, err)
				}
			case <-time.After(step.Timeout):
				t.Fatalf("Step %d (%s) verify timeout", i+1, step.Name)
			}
		}
	}
}

// Common scenario building blocks

// CreateWindowStep returns a step that creates a window.
func CreateWindowStep(rows, cols int) ScenarioStep {
	return ScenarioStep{
		Name: fmt.Sprintf("CreateWindow(%dx%d)", rows, cols),
		Action: func(s *shux.SessionRef) error {
			s.Send(shux.CreateWindow{Rows: rows, Cols: cols})
			return nil
		},
		Verify: func(s *shux.SessionRef) error {
			result := <-s.Ask(shux.GetActiveWindow{})
			if result == nil {
				return fmt.Errorf("no active window after creation")
			}
			return nil
		},
		Timeout: 500 * time.Millisecond,
	}
}

// CreatePaneStep returns a step that creates a pane in the active window.
func CreatePaneStep(rows, cols int, shell string) ScenarioStep {
	return ScenarioStep{
		Name: fmt.Sprintf("CreatePane(%dx%d, %s)", rows, cols, shell),
		Action: func(s *shux.SessionRef) error {
			winResult := <-s.Ask(shux.GetActiveWindow{})
			if winResult == nil {
				return fmt.Errorf("no active window")
			}
			win := winResult.(*shux.WindowRef)
			win.Send(shux.CreatePane{Rows: rows, Cols: cols, Shell: shell})
			return nil
		},
		Verify: func(s *shux.SessionRef) error {
			result := <-s.Ask(shux.GetActivePane{})
			if result == nil {
				return fmt.Errorf("no active pane after creation")
			}
			return nil
		},
		Timeout: 500 * time.Millisecond,
	}
}

// SplitPaneStep returns a step that splits the active pane.
func SplitPaneStep(dir shux.SplitDir) ScenarioStep {
	name := "SplitH"
	if dir == shux.SplitV {
		name = "SplitV"
	}
	return ScenarioStep{
		Name: fmt.Sprintf("SplitPane(%s)", name),
		Action: func(s *shux.SessionRef) error {
			winResult := <-s.Ask(shux.GetActiveWindow{})
			if winResult == nil {
				return fmt.Errorf("no active window")
			}
			win := winResult.(*shux.WindowRef)
			win.Send(shux.Split{Dir: dir})
			return nil
		},
		Verify: func(s *shux.SessionRef) error {
			winResult := <-s.Ask(shux.GetActiveWindow{})
			if winResult == nil {
				return fmt.Errorf("no active window")
			}
			winData := <-winResult.(*shux.WindowRef).Ask(shux.GetWindowSnapshotData{})
			if data, ok := winData.(shux.WindowSnapshot); ok {
				if len(data.PaneOrder) < 2 {
					return fmt.Errorf("expected at least 2 panes after split, got %d", len(data.PaneOrder))
				}
			}
			return nil
		},
		Timeout: 500 * time.Millisecond,
	}
}

// SwitchWindowStep returns a step that switches windows by delta.
func SwitchWindowStep(delta int) ScenarioStep {
	return ScenarioStep{
		Name: fmt.Sprintf("SwitchWindow(%d)", delta),
		Action: func(s *shux.SessionRef) error {
			s.Send(shux.SwitchWindow{Delta: delta})
			return nil
		},
		Timeout: 100 * time.Millisecond,
	}
}

// KillActivePaneStep returns a step that kills the active pane.
func KillActivePaneStep() ScenarioStep {
	return ScenarioStep{
		Name: "KillActivePane",
		Action: func(s *shux.SessionRef) error {
			paneResult := <-s.Ask(shux.GetActivePane{})
			if paneResult == nil {
				return fmt.Errorf("no active pane to kill")
			}
			pane := paneResult.(*shux.PaneRef)
			pane.Send(shux.KillPane{})
			return nil
		},
		Timeout: 500 * time.Millisecond,
	}
}

// DetachStep returns a step that detaches the session.
func DetachStep() ScenarioStep {
	return ScenarioStep{
		Name: "DetachSession",
		Action: func(s *shux.SessionRef) error {
			<-s.Ask(shux.DetachSession{})
			return nil
		},
		Timeout: 2 * time.Second,
	}
}

// SnapshotRestoreScenario builds a complete snapshot/restore test scenario.
func SnapshotRestoreScenario(sessionName string) []ScenarioStep {
	return []ScenarioStep{
		{
			Name: "CreateInitialWindow",
			Action: func(s *shux.SessionRef) error {
				s.Send(shux.CreateWindow{Rows: 24, Cols: 80})
				return nil
			},
			Verify: func(s *shux.SessionRef) error {
				result := <-s.Ask(shux.GetActiveWindow{})
				if result == nil {
					return fmt.Errorf("no window after creation")
				}
				return nil
			},
			Timeout: 500 * time.Millisecond,
		},
		{
			Name: "SplitPane",
			Action: func(s *shux.SessionRef) error {
				winResult := <-s.Ask(shux.GetActiveWindow{})
				if winResult == nil {
					return fmt.Errorf("no active window")
				}
				win := winResult.(*shux.WindowRef)
				win.Send(shux.Split{Dir: shux.SplitV})
				return nil
			},
			Verify: func(s *shux.SessionRef) error {
				winResult := <-s.Ask(shux.GetActiveWindow{})
				winData := <-winResult.(*shux.WindowRef).Ask(shux.GetWindowSnapshotData{})
				if data, ok := winData.(shux.WindowSnapshot); ok {
					if len(data.PaneOrder) != 2 {
						return fmt.Errorf("expected 2 panes, got %d", len(data.PaneOrder))
					}
				}
				return nil
			},
			Timeout: 500 * time.Millisecond,
		},
		{
			Name: "BuildSnapshot",
			Action: func(s *shux.SessionRef) error {
				// Snapshot is built during detach, just verify state is valid
				return nil
			},
			Verify: func(s *shux.SessionRef) error {
				result := <-s.Ask(shux.GetFullSessionSnapshot{})
				if result == nil {
					return fmt.Errorf("failed to build snapshot")
				}
				snapshot := result.(*shux.SessionSnapshot)
				if err := shux.ValidateSnapshot(snapshot); err != nil {
					return fmt.Errorf("snapshot validation failed: %w", err)
				}
				return nil
			},
			Timeout: 500 * time.Millisecond,
		},
	}
}

// FourPaneWorkflowScenario builds the 4-pane workflow scenario from AGENTS.md.
func FourPaneWorkflowScenario() []ScenarioStep {
	return []ScenarioStep{
		{
			Name: "CreateTopLeftPane",
			Action: func(s *shux.SessionRef) error {
				s.Send(shux.CreateWindow{Rows: 48, Cols: 160})
				return nil
			},
			Verify: func(s *shux.SessionRef) error {
				result := <-s.Ask(shux.GetActiveWindow{})
				if result == nil {
					return fmt.Errorf("no window created")
				}
				return nil
			},
			Timeout: 500 * time.Millisecond,
		},
		{
			Name: "SplitVerticalForRightPane",
			Action: func(s *shux.SessionRef) error {
				winResult := <-s.Ask(shux.GetActiveWindow{})
				win := winResult.(*shux.WindowRef)
				win.Send(shux.Split{Dir: shux.SplitV})
				return nil
			},
			Verify: func(s *shux.SessionRef) error {
				winResult := <-s.Ask(shux.GetActiveWindow{})
				winData := <-winResult.(*shux.WindowRef).Ask(shux.GetWindowSnapshotData{})
				if data, ok := winData.(shux.WindowSnapshot); ok {
					if len(data.PaneOrder) != 2 {
						return fmt.Errorf("expected 2 panes after vertical split, got %d", len(data.PaneOrder))
					}
				}
				return nil
			},
			Timeout: 500 * time.Millisecond,
		},
		{
			Name: "SplitLeftPaneHorizontal",
			Action: func(s *shux.SessionRef) error {
				winResult := <-s.Ask(shux.GetActiveWindow{})
				win := winResult.(*shux.WindowRef)
				win.Send(shux.SwitchToPane{Index: 0})
				time.Sleep(20 * time.Millisecond)
				win.Send(shux.Split{Dir: shux.SplitH})
				return nil
			},
			Verify: func(s *shux.SessionRef) error {
				winResult := <-s.Ask(shux.GetActiveWindow{})
				winData := <-winResult.(*shux.WindowRef).Ask(shux.GetWindowSnapshotData{})
				if data, ok := winData.(shux.WindowSnapshot); ok {
					if len(data.PaneOrder) != 3 {
						return fmt.Errorf("expected 3 panes, got %d", len(data.PaneOrder))
					}
				}
				return nil
			},
			Timeout: 500 * time.Millisecond,
		},
		{
			Name: "SplitRightPaneHorizontal",
			Action: func(s *shux.SessionRef) error {
				winResult := <-s.Ask(shux.GetActiveWindow{})
				win := winResult.(*shux.WindowRef)
				win.Send(shux.SwitchToPane{Index: 2}) // Right pane
				time.Sleep(20 * time.Millisecond)
				win.Send(shux.Split{Dir: shux.SplitH})
				return nil
			},
			Verify: func(s *shux.SessionRef) error {
				winResult := <-s.Ask(shux.GetActiveWindow{})
				winData := <-winResult.(*shux.WindowRef).Ask(shux.GetWindowSnapshotData{})
				if data, ok := winData.(shux.WindowSnapshot); ok {
					if len(data.PaneOrder) != 4 {
						return fmt.Errorf("expected 4 panes (2x2 layout), got %d", len(data.PaneOrder))
					}
				}
				return nil
			},
			Timeout: 500 * time.Millisecond,
		},
	}
}

// StressStep is a step for stress testing with configurable chaos parameters.
type StressStep struct {
	ScenarioStep
	// Chaos parameters
	FailProbability float64 // 0.0-1.0 chance of simulated failure
	DelayMin        time.Duration
	DelayMax        time.Duration
}
