package sim

import (
	"context"
	"testing"

	"shux/internal/cfg"
	"shux/internal/protocol"
	"shux/internal/shux"
	"shux/test/testutil"
)

func simPolicy(stateDir string, resurrection bool) cfg.Config {
	c := cfg.DefaultConfig()
	c.ShellPath = "/bin/sh"
	c.StateDir = stateDir
	c.JournalReplayDelay = 0
	c.Resurrection = resurrection
	return c
}

func buildFourPanes(t *testing.T, ctx context.Context, app *shux.Shux) (protocol.SessionID, protocol.WindowID) {
	t.Helper()
	ref := app.TestSupervisor()
	sid, wid := app.DefaultSessionID, app.DefaultWindowID
	var req protocol.RequestID
	testutil.SendSplit(t, ctx, ref, &req, "sim-test", sid, wid, "p-1", protocol.SplitVertical)
	testutil.SendSplit(t, ctx, ref, &req, "sim-test", sid, wid, "p-1", protocol.SplitHorizontal)
	testutil.SendSplit(t, ctx, ref, &req, "sim-test", sid, wid, "p-2", protocol.SplitHorizontal)
	if !app.WaitLayoutPanes(sid, wid, 4, testutil.TestWaitTimeout) {
		t.Fatal("expected four-pane layout")
	}
	return sid, wid
}

type paneCommand struct {
	Pane protocol.PaneID
	Cmd  string
	Want string
}

func runPaneCommands(t *testing.T, ctx context.Context, app *shux.Shux, sid protocol.SessionID, wid protocol.WindowID, cmds []paneCommand) {
	t.Helper()
	ref := app.TestSupervisor()
	for _, c := range cmds {
		testutil.SendPaste(t, ctx, ref, sid, wid, c.Pane, c.Cmd+"\n")
		if !app.WaitPaneScreen(sid, wid, c.Pane, c.Want, testutil.TestWaitTimeout) {
			t.Fatalf("pane %s missing %q after %q", c.Pane, c.Want, c.Cmd)
		}
	}
}
