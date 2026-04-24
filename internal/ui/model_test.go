package ui

import (
	"strings"
	"testing"
)

func TestNewModel_viewContainsTitle(t *testing.T) {
	m := NewModel()
	if !strings.Contains(m.View().Content, m.Title) {
		t.Fatalf("view should include title %q; got %q", m.Title, m.View().Content)
	}
}
