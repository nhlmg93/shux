package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"shux/internal/client"
	"shux/internal/daemon"
	"shux/internal/persist"
	"shux/internal/protocol"
	"shux/internal/shux"
	"shux/test/testutil"
)

func TestDaemon_gracefulRestartReplacesBackend(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/true")

	layout := persist.LayoutSnapshot{
		WindowID: "w-1",
		Cols:     80,
		Rows:     24,
		Panes:    []persist.LayoutPaneSnapshot{{PaneID: "p-1", Col: 0, Row: 0, Cols: 80, Rows: 24}},
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

	addr, stop := startTestDaemonWithConfig(t, cfg)
	defer stop()

	if err := client.Restart(t.Context(), addr); err != nil {
		t.Fatal(err)
	}
	if err := client.WaitReady(t.Context(), addr, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	attachAndDetach(t, addr)
}

func TestResurrectionCheckpoint_persistsWindowAndPaneNames(t *testing.T) {
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
	sid, wid, pid := app.DefaultSessionID, app.DefaultWindowID, app.DefaultPaneID
	testutil.MustSend(t, ctx, ref, protocol.CommandWindowRename{
		SessionID: sid,
		WindowID:  wid,
		Name:      "workspace",
	})
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneRename{
		SessionID: sid,
		WindowID:  wid,
		PaneID:    pid,
		Name:      "shell",
	})

	deadline := time.Now().Add(testutil.TestWaitTimeout)
	for time.Now().Before(deadline) {
		m, ok, err := persist.LoadManifest(dir)
		if err == nil && ok && len(m.Sessions) > 0 &&
			m.Sessions[0].WindowNames[string(wid)] == "workspace" &&
			m.Sessions[0].PaneNames[persist.PaneNameMapKey(wid, pid)] == "shell" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("checkpoint manifest missing renamed window/pane names")
}

func TestResurrection_windowAndPaneNamesSurviveRestart(t *testing.T) {
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
	sid, wid, pid := app1.DefaultSessionID, app1.DefaultWindowID, app1.DefaultPaneID
	testutil.MustSend(t, ctx, ref, protocol.CommandWindowRename{SessionID: sid, WindowID: wid, Name: "editor"})
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneRename{SessionID: sid, WindowID: wid, PaneID: pid, Name: "logs"})
	time.Sleep(200 * time.Millisecond)
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
	if got, _ := app2.WindowName(app2.DefaultSessionID, app2.DefaultWindowID); got != "editor" {
		t.Fatalf("window name = %q, want %q", got, "editor")
	}
	if got, _ := app2.PaneName(app2.DefaultSessionID, app2.DefaultWindowID, app2.DefaultPaneID); got != "logs" {
		t.Fatalf("pane name = %q, want %q", got, "logs")
	}
}

func startTestDaemonWithConfig(t *testing.T, cfg shux.Config) (string, func()) {
	t.Helper()
	addr := freeLoopbackAddr(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- daemon.RunWithConfig(ctx, addr, cfg) }()
	if err := client.WaitReady(t.Context(), addr, 2*time.Second); err != nil {
		cancel()
		t.Fatal(err)
	}
	return addr, func() {
		cancel()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("daemon stopped with error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("daemon did not stop")
		}
	}
}
