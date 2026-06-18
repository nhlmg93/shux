package sim

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"shux/internal/persist"
	"shux/internal/protocol"
	"shux/internal/shux"
	"shux/test/testutil"
)

// TestSim_fourPaneDailyDriverResurrection exercises live PTY journals (not pre-seeded files).
func TestSim_fourPaneDailyDriverResurrection(t *testing.T) {
	dir := t.TempDir()
	cfg := simPolicy(dir, true)

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
		if !bytes.Contains(data, []byte(marker)) {
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
	if app2.DefaultSessionID != sid || app2.DefaultWindowID != wid {
		t.Fatalf("restored ids = %q %q, want %q %q", app2.DefaultSessionID, app2.DefaultWindowID, sid, wid)
	}
	if !app2.WaitLayoutPanes(sid, wid, 4, testutil.TestWaitTimeout) {
		t.Fatal("restored layout missing four panes")
	}
	for pid, marker := range markers {
		if !app2.WaitPaneScreen(sid, wid, pid, marker, testutil.TestWaitTimeout) {
			t.Fatalf("restored pane %s missing marker %q", pid, marker)
		}
	}
}
