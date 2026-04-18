package main

import "sync/atomic"

var windowIDCounter uint32

// NewWindow creates a new window
func NewWindow() *Window {
	id := atomic.AddUint32(&windowIDCounter, 1)
	return &Window{ID: id}
}

// Window represents a collection of panes
type Window struct {
	ID     uint32
	Panes  []*Pane
	Active *Pane
}

// AddPane adds a pane to the window
func (w *Window) AddPane(p *Pane) {
	w.Panes = append(w.Panes, p)
	if w.Active == nil {
		w.Active = p
	}
}

// SetActivePane sets the active pane
func (w *Window) SetActivePane(p *Pane) {
	w.Active = p
}
