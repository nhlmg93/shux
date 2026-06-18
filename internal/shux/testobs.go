package shux

import (
	"context"
	"fmt"
	"strings"
	"time"

	"shux/internal/actor"
	"shux/internal/persist"
	"shux/internal/protocol"
)

// TestSupervisor exposes the live supervisor ref for integration-style tests.
func (a *Shux) TestSupervisor() actor.Ref[protocol.Command] {
	return a.supervisor
}

// WaitPaneScreen waits until a pane snapshot contains needle or times out.
func (a *Shux) WaitPaneScreen(sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID, needle string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if text, ok := a.PaneScreenText(sessionID, windowID, paneID); ok && strings.Contains(text, needle) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// RestoreFromCheckpoint replays a saved manifest for integration tests.
func (a *Shux) RestoreFromCheckpoint(ctx context.Context) error {
	if err := a.Start(ctx); err != nil {
		return err
	}
	m, ok, err := persist.LoadManifest(a.Config.StateDir)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("shux: no manifest in %q", a.Config.StateDir)
	}
	return a.restoreFromManifest(ctx, m)
}

// WaitLayoutPanes waits until a window layout has at least minPanes entries.
func (a *Shux) WaitLayoutPanes(sessionID protocol.SessionID, windowID protocol.WindowID, minPanes int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if layout, ok := a.cache.LayoutSnapshot(sessionID, windowID); ok && len(layout.Panes) >= minPanes {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// PaneScreenText returns the cached screen text for a pane. It is intended for
// integration tests observing resurrection replay.
func (a *Shux) PaneScreenText(sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID) (string, bool) {
	if a.cache == nil {
		return "", false
	}
	for _, screen := range a.cache.ScreenSnapshots(sessionID, windowID) {
		if screen.PaneID != paneID {
			continue
		}
		var b strings.Builder
		for _, line := range screen.Lines {
			b.WriteString(line.Text)
			b.WriteByte('\n')
		}
		return b.String(), true
	}
	return "", false
}
