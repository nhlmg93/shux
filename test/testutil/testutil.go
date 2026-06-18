package testutil

import (
	"context"
	"testing"
	"time"

	"shux/internal/protocol"
	"shux/internal/shux"
)

const TestWaitTimeout = 500 * time.Millisecond

type commandSender interface {
	Send(context.Context, protocol.Command) error
}

func MustSend(t testing.TB, ctx context.Context, ref commandSender, cmd protocol.Command) {
	t.Helper()
	if err := ref.Send(ctx, cmd); err != nil {
		t.Fatalf("send %T: %v", cmd, err)
	}
}

func SendSplit(t testing.TB, ctx context.Context, ref commandSender, req *protocol.RequestID, clientID protocol.ClientID, sid protocol.SessionID, wid protocol.WindowID, target protocol.PaneID, dir protocol.SplitDirection) {
	t.Helper()
	*req++
	MustSend(t, ctx, ref, protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: clientID, RequestID: *req},
		SessionID:    sid,
		WindowID:     wid,
		TargetPaneID: target,
		Direction:    dir,
	})
}

func SendPaste(t testing.TB, ctx context.Context, ref commandSender, sid protocol.SessionID, wid protocol.WindowID, pid protocol.PaneID, data string) {
	t.Helper()
	MustSend(t, ctx, ref, protocol.CommandPanePaste{
		SessionID: sid,
		WindowID:  wid,
		PaneID:    pid,
		Data:      []byte(data),
	})
}

func SendMove(t testing.TB, ctx context.Context, ref commandSender, sid protocol.SessionID, source protocol.WindowID, target protocol.WindowID, pid protocol.PaneID) {
	t.Helper()
	MustSend(t, ctx, ref, protocol.CommandPaneMove{
		SessionID:      sid,
		SourceWindowID: source,
		TargetWindowID: target,
		PaneID:         pid,
	})
}

func ResurrectionConfig(dir, shellPath string) shux.Config {
	cfg := shux.DefaultConfig()
	cfg.StateDir = dir
	cfg.Resurrection = true
	cfg.ShellPath = shellPath
	cfg.JournalReplayDelay = 0
	return cfg
}
