package sim

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"shux/internal/persist"
	"shux/internal/protocol"
	"shux/internal/shux"
)

// TestSim_fourPaneDailyDriverResurrection is the AGENTS.md user story at sim scope:
// four panes running distinct commands, checkpoint on shutdown, new daemon restores
// layout and replays live PTY journals (not pre-seeded files).
func TestSim_fourPaneDailyDriverResurrection(t *testing.T) {
	dir := t.TempDir()
	cfg := simResurrectionConfig(dir)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	app1, err := shux.NewShuxWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := app1.BootstrapDefaultSession(ctx); err != nil {
		t.Fatal(err)
	}

	sid, wid := buildFourPanes(t, ctx, app1)
	markers := map[protocol.PaneID]string{
		"p-1": "SHUX_DOC_OK",
		"p-2": "SHUX_NODE_OK",
		"p-3": "SHUX_NANO_OK",
		"p-4": "SHUX_SHELL_OK",
	}
	runPaneCommands(t, ctx, app1, sid, wid, []paneCommand{
		{Pane: "p-1", Cmd: "printf SHUX_DOC_OK", Want: markers["p-1"]},
		{Pane: "p-2", Cmd: `node -e "console.log('SHUX_NODE_OK')"`, Want: markers["p-2"]},
		{Pane: "p-3", Cmd: "printf SHUX_NANO_OK", Want: markers["p-3"]},
		{Pane: "p-4", Cmd: "printf SHUX_SHELL_OK", Want: markers["p-4"]},
	})

	for pid, marker := range markers {
		data, err := os.ReadFile(persist.JournalPath(dir, 1, pid))
		if err != nil {
			t.Fatalf("journal %s: %v", pid, err)
		}
		if !strings.Contains(string(data), marker) {
			t.Fatalf("journal %s missing live PTY marker %q: %q", pid, marker, data)
		}
	}

	if err := app1.Close(); err != nil {
		t.Fatal(err)
	}

	app2, err := shux.NewShuxWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app2.Close()
	if err := app2.BootstrapDefaultSession(ctx); err != nil {
		t.Fatal(err)
	}
	if !app2.WaitLayoutPanes(app2.DefaultSessionID, app2.DefaultWindowID, 4, 500*time.Millisecond) {
		t.Fatal("restored layout missing four panes")
	}
	assertPaneMarkers(t, app2, app2.DefaultSessionID, app2.DefaultWindowID, markers)
}
