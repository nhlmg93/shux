package ui

import (
	"strings"
	"testing"

	"shux/internal/protocol"
)

func TestNewModel_viewContainsPane(t *testing.T) {
	m := NewModel(protocol.SessionID("s-1"), protocol.WindowID("w-1"), protocol.PaneID("p-1"))
	if !strings.Contains(m.View().Content, string(m.PaneID)) {
		t.Fatalf("view should include pane %q; got %q", m.PaneID, m.View().Content)
	}
}
