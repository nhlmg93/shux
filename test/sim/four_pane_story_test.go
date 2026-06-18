package sim

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"shux/internal/protocol"
	"shux/internal/shux"
)

func TestTestBed_hasUserStoryTools(t *testing.T) {
	if os.Getenv("SHUX_SIM_DOCKER") == "" {
		t.Skip("user story tool check runs in Docker sim only")
	}
	for _, tool := range []string{"less", "nano", "node"} {
		if path, err := exec.LookPath(tool); err != nil {
			t.Fatalf("expected %q in sim test bed, not found: %v", tool, err)
		} else if path == "" {
			t.Fatalf("expected %q in sim test bed", tool)
		}
	}
}

func TestResurrectionStory_fourPaneLiveRecycle(t *testing.T) {
	dir := t.TempDir()
	cfg := shux.DefaultConfig()
	cfg.ShellPath = "/bin/sh"
	cfg.StateDir = dir
	cfg.Resurrection = true
	cfg.JournalReplayDelay = 0

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
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
	splitStoryPane(t, ctx, ref, sid, wid, "p-1", protocol.SplitVertical)
	splitStoryPane(t, ctx, ref, sid, wid, "p-1", protocol.SplitHorizontal)
	splitStoryPane(t, ctx, ref, sid, wid, "p-2", protocol.SplitHorizontal)
	if !app1.WaitLayoutPanes(sid, wid, 4, 500*time.Millisecond) {
		t.Fatal("story: expected four-pane layout")
	}

	markers := map[protocol.PaneID]string{
		"p-1": "STORY_P1",
		"p-2": "STORY_P2",
		"p-3": "STORY_P3",
		"p-4": "STORY_P4",
	}
	for pid, marker := range markers {
		pasteStoryPane(t, ctx, ref, sid, wid, pid, marker+"\n")
		if !app1.WaitPaneScreen(sid, wid, pid, marker, 500*time.Millisecond) {
			t.Fatalf("pane %s missing live marker %q", pid, marker)
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
		t.Fatal("story recycle: expected four-pane layout")
	}
	for pid, marker := range markers {
		if !app2.WaitPaneScreen(app2.DefaultSessionID, app2.DefaultWindowID, pid, marker, 500*time.Millisecond) {
			t.Fatalf("restored pane %s missing marker %q", pid, marker)
		}
	}
}

var storySplitReq protocol.RequestID

func splitStoryPane(t *testing.T, ctx context.Context, ref interface {
	Send(context.Context, protocol.Command) error
}, sid protocol.SessionID, wid protocol.WindowID, target protocol.PaneID, dir protocol.SplitDirection) {
	t.Helper()
	storySplitReq++
	if err := ref.Send(ctx, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: "story-test", RequestID: storySplitReq},
		SessionID:    sid,
		WindowID:     wid,
		TargetPaneID: target,
		Direction:    dir,
	}); err != nil {
		t.Fatal(err)
	}
}

func pasteStoryPane(t *testing.T, ctx context.Context, ref interface {
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
