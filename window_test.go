package main

import (
	"os/exec"
	"testing"
)

func TestNewWindow(t *testing.T) {
	w := NewWindow()
	if w.ID == 0 {
		t.Error("Expected window ID to be non-zero")
	}
	if w.Panes != nil {
		t.Error("Expected new window to have no panes")
	}
}

func TestAddPane(t *testing.T) {
	w := NewWindow()
	pane, _ := NewPane(exec.Command("/bin/true"))
	defer pane.Close()

	w.AddPane(pane)

	if len(w.Panes) != 1 {
		t.Errorf("Expected 1 pane, got %d", len(w.Panes))
	}
	if w.Active != pane {
		t.Error("Expected pane to be auto-set as active")
	}
}