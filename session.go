package main

import "sync/atomic"

var sessionIDCounter uint32

// NewSession creates a new session
func NewSession() *Session {
	id := atomic.AddUint32(&sessionIDCounter, 1)
	return &Session{ID: id}
}

// Session represents a collection of windows
type Session struct {
	ID      uint32
	Windows []*Window
	Active  *Window
}

// AddWindow adds a window to the session
func (s *Session) AddWindow(w *Window) {
	s.Windows = append(s.Windows, w)
	if s.Active == nil {
		s.Active = w
	}
}

// SetActiveWindow sets the active window
func (s *Session) SetActiveWindow(w *Window) {
	s.Active = w
}
