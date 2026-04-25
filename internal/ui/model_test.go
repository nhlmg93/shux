package ui

import (
	"strings"
	"testing"

	"shux/internal/protocol"
)

func TestNewModel_viewContainsTitle(t *testing.T) {
	m := NewModel(protocol.SessionID("s-1"), protocol.WindowID("w-1"), protocol.PaneID("p-1"))
	if !strings.Contains(m.View().Content, m.Title) {
		t.Fatalf("view should include title %q; got %q", m.Title, m.View().Content)
	}
}
