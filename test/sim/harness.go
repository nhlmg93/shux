package sim

import (
	"context"
	"testing"
	"time"

	"shux/internal/protocol"
	"shux/internal/shux"
)

func simResurrectionConfig(dir string) shux.Config {
	cfg := shux.DefaultConfig()
	cfg.ShellPath = "/bin/sh"
	cfg.StateDir = dir
	cfg.Resurrection = true
	cfg.JournalReplayDelay = 0
	return cfg
}

var simCmdReq protocol.RequestID

func sendSplit(t *testing.T, ctx context.Context, ref interface {
	Send(context.Context, protocol.Command) error
}, sid protocol.SessionID, wid protocol.WindowID, target protocol.PaneID, dir protocol.SplitDirection) {
	t.Helper()
	simCmdReq++
	if err := ref.Send(ctx, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: "sim-test", RequestID: simCmdReq},
		SessionID:    sid,
		WindowID:     wid,
		TargetPaneID: target,
		Direction:    dir,
	}); err != nil {
		t.Fatal(err)
	}
}

func sendPaste(t *testing.T, ctx context.Context, ref interface {
	Send(context.Context, protocol.Command) error
}, sid protocol.SessionID, wid protocol.WindowID, pid protocol.PaneID, data string) {
	t.Helper()
	if err := ref.Send(ctx, protocol.CommandPanePaste{
		SessionID: sid,
		WindowID:  wid,
		PaneID:    pid,
		Data:      []byte(data),
	}); err != nil {
		t.Fatal(err)
	}
}

func buildFourPanes(t *testing.T, ctx context.Context, app *shux.Shux) (protocol.SessionID, protocol.WindowID) {
	t.Helper()
	ref := app.TestSupervisor()
	sid, wid := app.DefaultSessionID, app.DefaultWindowID
	sendSplit(t, ctx, ref, sid, wid, "p-1", protocol.SplitVertical)
	sendSplit(t, ctx, ref, sid, wid, "p-1", protocol.SplitHorizontal)
	sendSplit(t, ctx, ref, sid, wid, "p-2", protocol.SplitHorizontal)
	if !app.WaitLayoutPanes(sid, wid, 4, 500*time.Millisecond) {
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
		sendPaste(t, ctx, ref, sid, wid, c.Pane, c.Cmd+"\n")
		if !app.WaitPaneScreen(sid, wid, c.Pane, c.Want, 500*time.Millisecond) {
			t.Fatalf("pane %s missing %q after %q", c.Pane, c.Want, c.Cmd)
		}
	}
}

func assertPaneMarkers(t *testing.T, app *shux.Shux, sid protocol.SessionID, wid protocol.WindowID, markers map[protocol.PaneID]string) {
	t.Helper()
	for pid, marker := range markers {
		if !app.WaitPaneScreen(sid, wid, pid, marker, 500*time.Millisecond) {
			t.Fatalf("pane %s missing marker %q", pid, marker)
		}
	}
}
