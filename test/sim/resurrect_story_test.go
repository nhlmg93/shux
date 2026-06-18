package sim

import (
	"context"
	"testing"
	"time"

	"shux/internal/persist"
	"shux/internal/protocol"
	"shux/internal/shux"
)

func TestResurrectionStory_preseededFourPaneLayout(t *testing.T) {
	dir := t.TempDir()
	cfg := shux.DefaultConfig()
	cfg.ShellPath = "/bin/true"
	cfg.StateDir = dir
	cfg.Resurrection = true
	cfg.JournalReplayDelay = 0

	layout := persist.LayoutSnapshot{
		WindowID: "w-1",
		Cols:     80,
		Rows:     24,
		Panes: []persist.LayoutPaneSnapshot{
			{PaneID: "p-1", Col: 0, Row: 0, Cols: 40, Rows: 12},
			{PaneID: "p-2", Col: 40, Row: 0, Cols: 40, Rows: 12},
			{PaneID: "p-3", Col: 0, Row: 12, Cols: 40, Rows: 12},
			{PaneID: "p-4", Col: 40, Row: 12, Cols: 40, Rows: 12},
		},
	}
	m := persist.BuildManifest(
		protocol.SessionID("s-1"),
		cfg.ShellPath,
		dir,
		[]protocol.WindowID{"w-1"},
		map[string]persist.LayoutSnapshot{"w-1": layout},
	)
	if err := persist.SaveManifest(dir, m); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	app, err := shux.NewShuxWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.RestoreFromCheckpoint(ctx); err != nil {
		t.Fatal(err)
	}
	if !app.WaitLayoutPanes(app.DefaultSessionID, app.DefaultWindowID, 4, 500*time.Millisecond) {
		t.Fatal("story restore: expected four-pane layout")
	}
}
