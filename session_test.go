package main

import (
	"testing"
)

func TestNewSession(t *testing.T) {
	s := NewSession()
	if s.ID == 0 {
		t.Error("Expected session ID to be non-zero")
	}
	if s.Windows != nil {
		t.Error("Expected new session to have no windows")
	}
}

func TestAddWindow(t *testing.T) {
	s := NewSession()
	w := NewWindow()

	s.AddWindow(w)

	if len(s.Windows) != 1 {
		t.Errorf("Expected 1 window, got %d", len(s.Windows))
	}
	if s.Active != w {
		t.Error("Expected window to be auto-set as active")
	}
}

func TestSetActiveWindow(t *testing.T) {
	s := NewSession()
	w1 := NewWindow()
	w2 := NewWindow()

	s.AddWindow(w1)
	s.AddWindow(w2)
	s.SetActiveWindow(w2)

	if s.Active != w2 {
		t.Error("Expected active window to be w2")
	}
}
