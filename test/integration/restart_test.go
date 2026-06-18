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
)

func TestDaemon_gracefulRestartReplacesBackend(t *testing.T) {
	dir := t.TempDir()
	cfg := shux.DefaultConfig()
	cfg.StateDir = dir
	cfg.Resurrection = true
	cfg.ShellPath = "/bin/true"
	cfg.JournalReplayDelay = 0

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
