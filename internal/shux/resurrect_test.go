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
	"shux/test/testutil"
)

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
	cfg := testutil.ResurrectionConfig(dir, "/bin/true")

	m := persist.BuildManifest(
		protocol.SessionID("s-1"),
		cfg.ShellPath,
		dir,
		[]protocol.WindowID{"w-1"},
		map[string]persist.LayoutSnapshot{"w-1": fourPaneLayout},
		nil,
		nil,
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
	if !app.WaitLayoutPanes(app.DefaultSessionID, app.DefaultWindowID, 4, testutil.TestWaitTimeout) {
		t.Fatal("restored window missing four panes")
	}
}

func TestRestoreFromManifest_twoWindows(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/true")

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
		nil,
		nil,
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

func TestRestoreFromManifest_zoomedLayout(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/true")

	m := persist.BuildManifest(
		protocol.SessionID("s-1"),
		cfg.ShellPath,
		dir,
		[]protocol.WindowID{"w-1"},
		map[string]persist.LayoutSnapshot{
			"w-1": {
				WindowID:     "w-1",
				Cols:         80,
				Rows:         24,
				ZoomedPaneID: "p-2",
				Panes: []persist.LayoutPaneSnapshot{
					{PaneID: "p-2", Col: 0, Row: 0, Cols: 80, Rows: 24},
				},
				SavedPanes: []persist.LayoutPaneSnapshot{
					{PaneID: "p-1", Col: 0, Row: 0, Cols: 40, Rows: 24},
					{PaneID: "p-2", Col: 40, Row: 0, Cols: 40, Rows: 24},
				},
			},
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
	if !app.WaitLayoutPanes(app.DefaultSessionID, app.DefaultWindowID, 1, testutil.TestWaitTimeout) {
		t.Fatal("zoomed restore missing visible pane")
	}
	if !app.WaitLayoutZoomed(app.DefaultSessionID, app.DefaultWindowID, "p-2", testutil.TestWaitTimeout) {
		t.Fatal("restored layout missing zoom state for p-2")
	}
}

func TestResurrection_journalReplayOnRestore(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/true")
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
		nil,
		nil,
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
	if !app.WaitPaneScreen(app.DefaultSessionID, app.DefaultWindowID, "p-1", marker, testutil.TestWaitTimeout) {
		t.Fatalf("restored pane missing journal marker %q", marker)
	}
}

func TestResurrection_eventDrivenCheckpoint(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/true")

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
	var splitReq protocol.RequestID
	testutil.SendSplit(t, ctx, ref, &splitReq, "resurrect-test", sid, wid, "p-1", protocol.SplitVertical)

	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Fatalf("expected manifest after layout change: %v", err)
	}
}

func TestResurrection_liveJournalReplayRoundtrip(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/true")
	cfg.ShellPath = "/bin/sh"
	marker := "SHUX_LIVE_MARKER"

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
	sid, wid, pid := app1.DefaultSessionID, app1.DefaultWindowID, app1.DefaultPaneID
	testutil.SendPaste(t, ctx, ref, sid, wid, pid, marker+"\n")
	if !app1.WaitPaneScreen(sid, wid, pid, marker, testutil.TestWaitTimeout) {
		t.Fatal("live pane missing marker before checkpoint")
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
	if !app2.WaitPaneScreen(app2.DefaultSessionID, app2.DefaultWindowID, pid, marker, testutil.TestWaitTimeout) {
		t.Fatal("restored pane missing live journal marker")
	}
}

func TestResurrection_checkpointLayoutRoundtrip(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/true")

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
	var splitReq protocol.RequestID
	testutil.SendSplit(t, ctx, ref, &splitReq, "resurrect-test", sid, wid, "p-1", protocol.SplitVertical)
	testutil.SendSplit(t, ctx, ref, &splitReq, "resurrect-test", sid, wid, "p-1", protocol.SplitHorizontal)
	testutil.SendSplit(t, ctx, ref, &splitReq, "resurrect-test", sid, wid, "p-2", protocol.SplitHorizontal)
	if !app1.WaitLayoutPanes(sid, wid, 4, testutil.TestWaitTimeout) {
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
	if !app2.WaitLayoutPanes(app2.DefaultSessionID, app2.DefaultWindowID, 4, testutil.TestWaitTimeout) {
		t.Fatal("restored layout missing four panes")
	}
}

func TestResurrection_checkpointPersistsResizedPaneGeometry(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/true")

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
	var splitReq protocol.RequestID
	testutil.SendSplit(t, ctx, ref, &splitReq, "resurrect-test", sid, wid, "p-1", protocol.SplitVertical)
	if !app1.WaitLayoutPanes(sid, wid, 2, testutil.TestWaitTimeout) {
		t.Fatal("live layout never reached two panes")
	}
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneResizeDelta{
		Meta:         protocol.CommandMeta{ClientID: "resurrect-test", RequestID: splitReq + 1},
		SessionID:    sid,
		WindowID:     wid,
		TargetPaneID: "p-1",
		Edge:         protocol.PaneResizeEdgeRight,
		Delta:        6,
	})
	time.Sleep(200 * time.Millisecond)
	if err := app1.Close(); err != nil {
		t.Fatal(err)
	}

	manifest, ok, err := persist.LoadManifest(dir)
	if err != nil || !ok {
		t.Fatalf("load manifest: ok=%v err=%v", ok, err)
	}
	if len(manifest.Sessions) == 0 {
		t.Fatal("manifest missing sessions")
	}
	layout := manifest.Sessions[0].Layouts["w-1"]
	if len(layout.Panes) != 2 {
		t.Fatalf("manifest panes = %d, want 2", len(layout.Panes))
	}
	var leftCols int
	for _, pane := range layout.Panes {
		if pane.PaneID == "p-1" {
			leftCols = pane.Cols
		}
	}
	if leftCols <= 40 {
		t.Fatalf("resized pane width = %d, want > 40", leftCols)
	}

	app2, err := shux.NewShuxWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app2.Close()
	if err := app2.BootstrapDefaultSession(ctx); err != nil {
		t.Fatal(err)
	}
	if !app2.WaitLayoutPanes(app2.DefaultSessionID, app2.DefaultWindowID, 2, testutil.TestWaitTimeout) {
		t.Fatal("restored layout missing two panes")
	}
}
func TestResurrection_movePaneBreakJoinRoundtrip(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/sh")
	token := "SHUX_MOVE_TOKEN_13"

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
	sid := app1.DefaultSessionID
	wid1 := app1.DefaultWindowID
	var splitReq protocol.RequestID
	testutil.SendSplit(t, ctx, ref, &splitReq, "move-pane-test", sid, wid1, app1.DefaultPaneID, protocol.SplitVertical)
	if !app1.WaitLayoutPanes(sid, wid1, 2, testutil.TestWaitTimeout) {
		t.Fatal("expected split window with two panes")
	}

	movedPaneID := protocol.PaneID("p-2")
	testutil.SendPaste(t, ctx, ref, sid, wid1, movedPaneID, "export SHUX_MOVE_TOKEN_13=ok\n")
	if !app1.WaitPaneScreen(sid, wid1, movedPaneID, token, testutil.TestWaitTimeout) {
		t.Fatal("source pane missing move marker before break")
	}

	testutil.SendMove(t, ctx, ref, sid, wid1, "", movedPaneID)
	if !app1.WaitWindowCount(sid, 2, testutil.TestWaitTimeout) {
		t.Fatalf("window count after break = %d, want 2", app1.WindowCount(sid))
	}
	var wid2 protocol.WindowID
	for _, wid := range app1.WindowIDs(sid) {
		if wid != wid1 {
			wid2 = wid
			break
		}
	}
	if !wid2.Valid() {
		t.Fatal("break-pane did not create a second window")
	}
	testutil.SendPaste(t, ctx, ref, sid, wid2, movedPaneID, "printf '%s\\n' \"$SHUX_MOVE_TOKEN_13\"\n")
	if !app1.WaitPaneScreen(sid, wid2, movedPaneID, token, testutil.TestWaitTimeout) {
		t.Fatal("moved pane in break window lost PTY environment")
	}

	testutil.SendMove(t, ctx, ref, sid, wid2, wid1, movedPaneID)
	if !app1.WaitWindowCount(sid, 1, testutil.TestWaitTimeout) {
		t.Fatalf("window count after join = %d, want 1", app1.WindowCount(sid))
	}
	if !app1.WaitLayoutPanes(sid, wid1, 2, testutil.TestWaitTimeout) {
		t.Fatal("joined target window missing moved pane")
	}
	testutil.SendPaste(t, ctx, ref, sid, wid1, movedPaneID, "printf '%s\\n' \"$SHUX_MOVE_TOKEN_13\"\n")
	if !app1.WaitPaneScreen(sid, wid1, movedPaneID, token, testutil.TestWaitTimeout) {
		t.Fatal("joined pane lost PTY environment")
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
	if !app2.WaitWindowCount(app2.DefaultSessionID, 1, testutil.TestWaitTimeout) {
		t.Fatalf("restored window count = %d, want 1", app2.WindowCount(app2.DefaultSessionID))
	}
	if !app2.WaitLayoutPanes(app2.DefaultSessionID, app2.DefaultWindowID, 2, testutil.TestWaitTimeout) {
		t.Fatal("restored layout missing moved pane")
	}
	if !app2.WaitPaneScreen(app2.DefaultSessionID, app2.DefaultWindowID, movedPaneID, token, testutil.TestWaitTimeout) {
		t.Fatal("restored moved pane missing journal marker")
	}
}
