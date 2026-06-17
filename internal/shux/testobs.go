package shux

import (
	"strings"

	"shux/internal/protocol"
)

// PaneScreenText returns joined viewport line text for a pane snapshot in the state cache.
// It is intended for integration tests observing resurrection replay.
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
