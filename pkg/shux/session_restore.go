package shux

func (s *Session) restoreFromSnapshot() {
	s.logger.Infof("restore: session=%s id=%d windows=%d activeWindow=%d", s.name, s.id, len(s.snapshot.Windows), s.snapshot.ActiveWindow)

	windowsByID, maxWindowID := indexWindowSnapshots(s.snapshot.Windows)
	for _, winID := range s.snapshot.WindowOrder {
		winSnap, ok := windowsByID[winID]
		if !ok {
			continue
		}
		s.restoreWindow(winSnap)
	}

	s.windowID = maxWindowID
	s.restoreActiveWindow(s.snapshot.ActiveWindow)
	s.assertInvariants()

	s.logger.Infof("restore: session=%s id=%d complete windows=%d activeWindow=%d nextWindowID=%d", s.name, s.id, len(s.windows), s.active, s.windowID)
}

func indexWindowSnapshots(windows []WindowSnapshot) (map[uint32]WindowSnapshot, uint32) {
	windowsByID := make(map[uint32]WindowSnapshot, len(windows))
	var maxWindowID uint32
	for _, winSnap := range windows {
		windowsByID[winSnap.ID] = winSnap
		if winSnap.ID > maxWindowID {
			maxWindowID = winSnap.ID
		}
	}
	return windowsByID, maxWindowID
}

func (s *Session) restoreWindow(winSnap WindowSnapshot) {
	s.logger.Infof("restore: session=%s window=%d panes=%d activePane=%d", s.name, winSnap.ID, len(winSnap.PaneOrder), winSnap.ActivePane)

	windowRef := StartWindow(winSnap.ID, s.ref, s.logger)
	s.windows[winSnap.ID] = windowRef
	s.windowOrder.Add(winSnap.ID)
	s.restoreWindowPanes(windowRef, winSnap)

	if winSnap.Layout != nil {
		windowRef.Send(RestoreWindowLayout{Root: winSnap.Layout, ActivePane: winSnap.ActivePane})
		return
	}
	if activeIdx := OrderedIDList(winSnap.PaneOrder).IndexOf(winSnap.ActivePane); activeIdx > 0 {
		windowRef.Send(SwitchToPane{Index: activeIdx})
	}
}

func (s *Session) restoreWindowPanes(windowRef *WindowRef, winSnap WindowSnapshot) {
	panesByID := make(map[uint32]PaneSnapshot, len(winSnap.Panes))
	for _, paneSnap := range winSnap.Panes {
		panesByID[paneSnap.ID] = paneSnap
	}

	for _, paneID := range winSnap.PaneOrder {
		paneSnap, ok := panesByID[paneID]
		if !ok {
			continue
		}
		s.logger.Infof("restore: session=%s window=%d pane=%d shell=%s cwd=%s rows=%d cols=%d", s.name, winSnap.ID, paneSnap.ID, paneSnap.Shell, paneSnap.CWD, paneSnap.Rows, paneSnap.Cols)
		windowRef.Send(CreatePane{
			ID:    paneSnap.ID,
			Rows:  paneSnap.Rows,
			Cols:  paneSnap.Cols,
			Shell: paneSnap.Shell,
			CWD:   paneSnap.CWD,
		})
	}
}

func (s *Session) restoreActiveWindow(activeWindow uint32) {
	if activeWindow != 0 {
		if _, ok := s.windows[activeWindow]; ok {
			s.active = activeWindow
			return
		}
	}
	if firstWindow, ok := s.windowOrder.First(); ok {
		s.active = firstWindow
	}
}
