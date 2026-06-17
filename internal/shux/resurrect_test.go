package shux_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"shux/internal/persist"
	"shux/internal/protocol"
	"shux/internal/shux"
)

const resurrectMarker = "SHUX_RESURRECT_OK"

func TestResurrection_checkpointRestore(t *testing.T) {
	dir := t.TempDir()
	cfg := shux.DefaultConfig()
	cfg.StateDir = dir
	cfg.Resurrection = true

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	app1, err := shux.NewShuxWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := app1.BootstrapDefaultSession(ctx); err != nil {
		t.Fatal(err)
	}

	journalPath := persist.JournalPath(dir, app1.DefaultWindowID, app1.DefaultPaneID)
	if err := os.MkdirAll(filepath.Dir(journalPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath, []byte(resurrectMarker+"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	layouts := map[string]persist.LayoutSnapshot{
		string(app1.DefaultWindowID): {
			WindowID: string(app1.DefaultWindowID),
			Cols:     80,
			Rows:     24,
			Panes: []persist.LayoutPaneSnapshot{
				{PaneID: string(app1.DefaultPaneID), Col: 0, Row: 0, Cols: 80, Rows: 24},
			},
		},
	}
	m := persist.BuildManifest(
		app1.DefaultSessionID,
		cfg.ShellPath,
		dir,
		[]protocol.WindowID{app1.DefaultWindowID},
		layouts,
	)
	if err := persist.SaveManifest(dir, m); err != nil {
		t.Fatal(err)
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

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		text, ok := app2.PaneScreenText(app2.DefaultSessionID, app2.DefaultWindowID, app2.DefaultPaneID)
		if ok && strings.Contains(text, resurrectMarker) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("restored pane did not replay journal marker")
}
