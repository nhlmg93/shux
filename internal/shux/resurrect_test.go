package shux_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"shux/internal/persist"
	"shux/internal/protocol"
	"shux/internal/shux"
)

func fastResurrectionConfig(t *testing.T, dir string) shux.Config {
	t.Helper()
	cfg := shux.DefaultConfig()
	cfg.ShellPath = "/bin/true"
	cfg.StateDir = dir
	cfg.Resurrection = true
	cfg.JournalReplayDelay = 0
	return cfg
}

var fourPaneLayout = persist.LayoutSnapshot{
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

func TestRestoreFromManifest_fourPaneLayout(t *testing.T) {
	dir := t.TempDir()
	cfg := fastResurrectionConfig(t, dir)

	m := persist.BuildManifest(
		protocol.SessionID("s-1"),
		cfg.ShellPath,
		dir,
		[]protocol.WindowID{"w-1"},
		map[string]persist.LayoutSnapshot{"w-1": fourPaneLayout},
	)
	if err := persist.SaveManifest(dir, m); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
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
		t.Fatal("restored window missing four panes")
	}
}

func TestRestoreFromManifest_twoWindows(t *testing.T) {
	dir := t.TempDir()
	cfg := fastResurrectionConfig(t, dir)

	singlePane := func(wid string) persist.LayoutSnapshot {
		return persist.LayoutSnapshot{
			WindowID: wid,
			Cols:     80,
			Rows:     24,
			Panes:    []persist.LayoutPaneSnapshot{{PaneID: "p-1", Col: 0, Row: 0, Cols: 80, Rows: 24}},
		}
	}
	m := persist.BuildManifest(
		protocol.SessionID("s-1"),
		cfg.ShellPath,
		dir,
		[]protocol.WindowID{"w-1", "w-2"},
		map[string]persist.LayoutSnapshot{
			"w-1": singlePane("w-1"),
			"w-2": singlePane("w-2"),
		},
	)
	if err := persist.SaveManifest(dir, m); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	app, err := shux.NewShuxWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.RestoreFromCheckpoint(ctx); err != nil {
		t.Fatal(err)
	}
	if app.WindowCount(app.DefaultSessionID) != 2 {
		t.Fatalf("window count = %d, want 2", app.WindowCount(app.DefaultSessionID))
	}
}

func TestResurrection_journalReplayOnRestore(t *testing.T) {
	dir := t.TempDir()
	cfg := fastResurrectionConfig(t, dir)
	marker := "SHUX_L2_MARKER"

	layout := persist.LayoutSnapshot{
		WindowID: "w-1",
		Cols:     80,
		Rows:     24,
		Panes:    []persist.LayoutPaneSnapshot{{PaneID: "p-1", Col: 0, Row: 0, Cols: 80, Rows: 24}},
	}
	j, err := persist.OpenJournal(dir, 1, "p-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Append([]byte(marker + "\r\n")); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
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

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	app, err := shux.NewShuxWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.RestoreFromCheckpoint(ctx); err != nil {
		t.Fatal(err)
	}
	if !app.WaitPaneScreen(app.DefaultSessionID, app.DefaultWindowID, "p-1", marker, 500*time.Millisecond) {
		t.Fatalf("restored pane missing journal marker %q", marker)
	}
}

func TestResurrection_eventDrivenCheckpoint(t *testing.T) {
	dir := t.TempDir()
	cfg := fastResurrectionConfig(t, dir)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	app, err := shux.NewShuxWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.BootstrapDefaultSession(ctx); err != nil {
		t.Fatal(err)
	}

	ref := app.TestSupervisor()
	sid, wid := app.DefaultSessionID, app.DefaultWindowID
	splitPane(t, ctx, ref, sid, wid, "p-1", protocol.SplitVertical)

	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Fatalf("expected manifest after layout change: %v", err)
	}
}

func TestResurrection_checkpointLayoutRoundtrip(t *testing.T) {
	dir := t.TempDir()
	cfg := fastResurrectionConfig(t, dir)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	app1, err := shux.NewShuxWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := app1.BootstrapDefaultSession(ctx); err != nil {
		t.Fatal(err)
	}

	ref := app1.TestSupervisor()
	sid, wid := app1.DefaultSessionID, app1.DefaultWindowID
	splitPane(t, ctx, ref, sid, wid, "p-1", protocol.SplitVertical)
	splitPane(t, ctx, ref, sid, wid, "p-1", protocol.SplitHorizontal)
	splitPane(t, ctx, ref, sid, wid, "p-2", protocol.SplitHorizontal)
	if !app1.WaitLayoutPanes(sid, wid, 4, 500*time.Millisecond) {
		t.Fatal("live layout never reached four panes")
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
}

var testSplitReq protocol.RequestID

func splitPane(t *testing.T, ctx context.Context, ref interface {
	Send(context.Context, protocol.Command) error
}, sid protocol.SessionID, wid protocol.WindowID, target protocol.PaneID, dir protocol.SplitDirection) {
	t.Helper()
	testSplitReq++
	if err := ref.Send(ctx, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: "resurrect-test", RequestID: testSplitReq},
		SessionID:    sid,
		WindowID:     wid,
		TargetPaneID: target,
		Direction:    dir,
	}); err != nil {
		t.Fatal(err)
	}
}
