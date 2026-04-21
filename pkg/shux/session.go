package shux

import (
	"fmt"
	"os"
)

// SessionRef is a reference to a session loop. Methods are promoted from loopRef.
type SessionRef struct {
	*loopRef
	shell       string
	sessionName string
	ownerMode   bool // true if running as owner process (saves on detach, doesn't stop)
}

// GetShell returns the shell used by the session.
func (r *SessionRef) GetShell() string {
	if r == nil {
		return DefaultShell
	}
	return r.shell
}

// GetSessionName returns the name of the session.
func (r *SessionRef) GetSessionName() string {
	if r == nil {
		return DefaultSessionName
	}
	return r.sessionName
}

// SetOwnerMode marks the session as running in owner mode.
// In owner mode, detach saves the session but doesn't stop it.
func (r *SessionRef) SetOwnerMode() {
	if r != nil {
		r.ownerMode = true
	}
}

type Session struct {
	ref         *SessionRef
	logger      ShuxLogger
	notify      func(any)
	id          uint32
	name        string
	windows     map[uint32]*WindowRef
	windowOrder OrderedIDList
	active      uint32
	lastActive  uint32
	windowID    uint32
	subscribers map[chan any]struct{}
	shell       string
	snapshot    *SessionSnapshot
	lastRows    int // last known terminal dimensions for window creation
	lastCols    int
}

func NewSession(id uint32, logger ShuxLogger) *Session {
	return NewNamedSessionWithShell(id, DefaultSessionName, DefaultShell, logger)
}

func NewSessionWithShell(id uint32, shell string, logger ShuxLogger) *Session {
	return NewNamedSessionWithShell(id, DefaultSessionName, shell, logger)
}

func NewNamedSessionWithShell(id uint32, name, shell string, logger ShuxLogger) *Session {
	name = normalizeSessionName(name)
	return &Session{
		id:          id,
		name:        name,
		logger:      logger,
		windows:     make(map[uint32]*WindowRef),
		subscribers: make(map[chan any]struct{}),
		shell:       normalizeShell(shell),
	}
}

func StartSession(id uint32, notify func(any), logger ShuxLogger) *SessionRef {
	return StartNamedSessionWithShell(id, DefaultSessionName, DefaultShell, notify, logger)
}

func StartSessionWithShell(id uint32, shell string, notify func(any), logger ShuxLogger) *SessionRef {
	return StartNamedSessionWithShell(id, DefaultSessionName, shell, notify, logger)
}

func StartNamedSessionWithShell(id uint32, name, shell string, notify func(any), logger ShuxLogger) *SessionRef {
	s := NewNamedSessionWithShell(id, name, shell, logger)
	s.notify = notify
	return startSessionLoop(s)
}

func StartSessionFromSnapshot(snapshot *SessionSnapshot, notify func(any), logger ShuxLogger) *SessionRef {
	name := normalizeSessionName(snapshot.SessionName)
	s := &Session{
		id:          snapshot.ID,
		name:        name,
		logger:      logger,
		notify:      notify,
		windows:     make(map[uint32]*WindowRef),
		subscribers: make(map[chan any]struct{}),
		shell:       normalizeShell(snapshot.Shell),
		snapshot:    snapshot,
	}
	return startSessionLoop(s)
}

func startSessionLoop(s *Session) *SessionRef {
	ref := &SessionRef{
		loopRef:     newLoopRef(32),
		shell:       s.shell,
		sessionName: s.name,
	}
	s.ref = ref
	go s.run()
	return ref
}

func (s *Session) run() {
	var reason error
	defer func() {
		if r := recover(); r != nil {
			reason = fmt.Errorf("panic: %v\n%s", r, recoverWithContext("session", s.id, len(s.windows), int(s.active)))
		}
		s.terminate(reason)
		close(s.ref.done)
	}()

	s.logger.Infof("session: init id=%d name=%s shell=%s restore=%t", s.id, s.name, s.shell, s.snapshot != nil)
	if s.snapshot != nil {
		s.restoreFromSnapshot()
		s.snapshot = nil
	}

	for {
		select {
		case <-s.ref.stop:
			return
		case msg := <-s.ref.inbox:
			s.receive(msg)
		}
	}
}

func (s *Session) terminate(reason error) {
	for _, win := range s.windows {
		if win != nil {
			win.Shutdown()
		}
	}
	if reason != nil {
		s.logger.Errorf("session: crash id=%d name=%s reason=%v", s.id, s.name, reason)
		return
	}
	s.logger.Infof("session: terminate id=%d name=%s", s.id, s.name)
}

// assertInvariants validates internal state consistency.
// Panics on invariant violation (tiger style - fail fast on bugs).
func (s *Session) assertInvariants() {
	// Map/order sync: len(windowOrder) == len(windows)
	if len(s.windowOrder) != len(s.windows) {
		panic(fmt.Sprintf("session %d: windowOrder length (%d) != windows count (%d)", s.id, len(s.windowOrder), len(s.windows)))
	}

	// windowOrder entries must all exist in windows map
	for _, winID := range s.windowOrder {
		if _, ok := s.windows[winID]; !ok {
			panic(fmt.Sprintf("session %d: windowOrder contains missing window %d", s.id, winID))
		}
	}

	// If windows exist, activeWindow must be non-zero and valid
	if len(s.windowOrder) > 0 {
		if s.active == 0 {
			panic(fmt.Sprintf("session %d: activeWindow=0 but %d windows exist", s.id, len(s.windowOrder)))
		}
		if _, ok := s.windows[s.active]; !ok {
			panic(fmt.Sprintf("session %d: activeWindow %d not in windows map", s.id, s.active))
		}
	}
}

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

func (s *Session) receive(msg any) {
	switch m := msg.(type) {
	case CreateWindow:
		s.createWindow(m.Rows, m.Cols)
	case SwitchWindow:
		s.switchWindow(m.Delta)
	case Split:
		s.forwardToActiveWindow(Split{Dir: m.Dir})
	case NavigatePane:
		s.forwardToActiveWindow(NavigatePane{Dir: m.Dir})
	case ResizePane:
		s.forwardToActiveWindow(ResizePane{Dir: m.Dir, Amount: m.Amount})
	case ActionMsg:
		_ = s.dispatchAction(m)
	case WindowEmpty:
		s.handleWindowEmpty(m.ID)
	case PaneContentUpdated:
		s.forwardUpdate(m)
	case ResizeMsg:
		s.resizeActiveWindow(m.Rows, m.Cols)
	case SubscribeUpdates:
		if m.Subscriber != nil {
			s.subscribers[m.Subscriber] = struct{}{}
		}
	case UnsubscribeUpdates:
		if m.Subscriber != nil {
			delete(s.subscribers, m.Subscriber)
		}
	case WriteToPane, KeyInput:
		s.forwardToActivePane(m)
	case MouseInput:
		s.forwardToActiveWindow(m)
	case DetachSession:
		if err := s.handleDetach(); err != nil {
			s.logger.Warnf("detach: session=%s id=%d failed err=%v", s.name, s.id, err)
		}
	case ExecuteCommandMsg:
		s.handleExecuteCommand(m)
	case askEnvelope:
		s.handleAsk(m)
	}
}

// dispatchAction handles session-scoped and window-scoped actions.
func (s *Session) dispatchAction(msg ActionMsg) ActionResult {
	switch msg.Action {
	// Session-scoped actions
	case ActionQuit:
		return ActionResult{Quit: true}
	case ActionNewWindow:
		// Use last known dimensions, or defaults if never resized
		rows, cols := s.lastRows, s.lastCols
		if rows <= 0 || cols <= 0 {
			rows, cols = 24, 80
		}
		s.createWindow(rows, cols)
	case ActionNextWindow:
		s.switchWindow(1)
	case ActionPrevWindow:
		s.switchWindow(-1)
	case ActionLastWindow:
		if s.lastActive != 0 {
			if _, ok := s.windows[s.lastActive]; ok {
				s.setActiveWindow(s.lastActive)
			}
		}
	case ActionDetach:
		if err := s.handleDetach(); err != nil {
			s.logger.Warnf("detach: session=%s id=%d failed err=%v", s.name, s.id, err)
			return ActionResult{Err: err}
		}
		return ActionResult{Quit: true}
	case ActionRenameSession:
		// TODO: implement session renaming with prompt
		s.logger.Infof("session: rename requested (not yet implemented)")

	// Window-scoped actions - forward to active window
	case ActionSplitHorizontal:
		s.forwardToActiveWindow(Split{Dir: SplitH})
	case ActionSplitVertical:
		s.forwardToActiveWindow(Split{Dir: SplitV})
	case ActionSelectPaneLeft:
		s.forwardToActiveWindow(NavigatePane{Dir: PaneNavLeft})
	case ActionSelectPaneDown:
		s.forwardToActiveWindow(NavigatePane{Dir: PaneNavDown})
	case ActionSelectPaneUp:
		s.forwardToActiveWindow(NavigatePane{Dir: PaneNavUp})
	case ActionSelectPaneRight:
		s.forwardToActiveWindow(NavigatePane{Dir: PaneNavRight})
	case ActionResizePaneLeft:
		s.forwardToActiveWindow(ResizePane{Dir: PaneNavLeft, Amount: msg.Amount})
	case ActionResizePaneDown:
		s.forwardToActiveWindow(ResizePane{Dir: PaneNavDown, Amount: msg.Amount})
	case ActionResizePaneUp:
		s.forwardToActiveWindow(ResizePane{Dir: PaneNavUp, Amount: msg.Amount})
	case ActionResizePaneRight:
		s.forwardToActiveWindow(ResizePane{Dir: PaneNavRight, Amount: msg.Amount})
	case ActionSelectWindow0, ActionSelectWindow1, ActionSelectWindow2, ActionSelectWindow3,
		ActionSelectWindow4, ActionSelectWindow5, ActionSelectWindow6, ActionSelectWindow7,
		ActionSelectWindow8, ActionSelectWindow9:
		idx := int(msg.Action[len("select_window_")] - '0')
		if idx >= 0 && idx < len(s.windowOrder) {
			s.setActiveWindow(s.windowOrder[idx])
		}
	case ActionKillWindow:
		s.killActiveWindow()
	case ActionKillSession:
		return s.killSession()

	// Pane-scoped actions - forward to active window for dispatch
	case ActionKillPane, ActionZoomPane, ActionSwapPaneUp, ActionSwapPaneDown,
		ActionRenameWindow:
		s.forwardToActiveWindow(msg)

	// Session management actions (deferred for now)
	case ActionListSessions:
		s.logger.Infof("session: list-sessions requested (not yet implemented)")
	case ActionAttachSession:
		if len(msg.Args) > 0 {
			s.logger.Infof("session: attach-session %q requested (not yet implemented)", msg.Args[0])
		} else {
			s.logger.Infof("session: attach-session requested without name (not yet implemented)")
		}

	// Interactive/prompting actions (deferred for now)
	case ActionCommandPrompt, ActionChooseTreeSessions, ActionChooseTreeWindows, ActionShowHelp:
		s.logger.Infof("session: action %q requested (not yet implemented)", msg.Action)

	default:
		s.logger.Warnf("session: unknown action %q", msg.Action)
	}
	return ActionResult{}
}

// handleExecuteCommand parses and executes a command string.
func (s *Session) handleExecuteCommand(msg ExecuteCommandMsg) {
	cmd, err := ParseCommand(msg.Command)
	if err != nil {
		s.logger.Warnf("command: parse error: %v", err)
		return
	}

	actionMsg, ok := cmd.ToActionMsg()
	if !ok {
		s.logger.Warnf("command: unknown command %q", cmd.Name)
		return
	}

	result := s.dispatchAction(actionMsg)
	if result.Err != nil {
		s.logger.Warnf("command: execution error: %v", result.Err)
		return
	}
	if result.Quit {
		s.logger.Infof("command: %q triggered quit", msg.Command)
	}
}

func (s *Session) handleAsk(envelope askEnvelope) {
	switch m := envelope.msg.(type) {
	case GetActiveWindow:
		if win := s.activeWindow(); win != nil {
			envelope.reply <- win
			return
		}
		envelope.reply <- nil
	case GetActivePane:
		if win := s.activeWindow(); win != nil {
			result, _ := askValue(win, GetActivePane{})
			envelope.reply <- result
			return
		}
		envelope.reply <- nil
	case GetPaneContent:
		if win := s.activeWindow(); win != nil {
			result, _ := askValue(win, envelope.msg)
			envelope.reply <- result
			return
		}
		envelope.reply <- nil
	case GetWindowView:
		if win := s.activeWindow(); win != nil {
			result, _ := askValue(win, envelope.msg)
			envelope.reply <- result
			return
		}
		envelope.reply <- nil
	case GetSessionSnapshotData:
		data := SessionSnapshotData{
			ID:           s.id,
			SessionName:  s.name,
			Shell:        s.shell,
			ActiveWindow: s.active,
			WindowOrder:  s.windowOrder.Clone(),
		}
		envelope.reply <- data
	case GetFullSessionSnapshot:
		envelope.reply <- s.buildSnapshot()
	case DetachSession:
		envelope.reply <- s.handleDetach()
	case ActionMsg:
		envelope.reply <- s.dispatchAction(m)
	case ExecuteCommandMsg:
		envelope.reply <- s.executeCommandWithResult(m)
	default:
		envelope.reply <- nil
	}
}

// executeCommandWithResult parses and executes a command, returning a CommandResult.
func (s *Session) executeCommandWithResult(msg ExecuteCommandMsg) CommandResult {
	cmd, err := ParseCommand(msg.Command)
	if err != nil {
		return CommandResult{Success: false, Error: err.Error()}
	}

	actionMsg, ok := cmd.ToActionMsg()
	if !ok {
		return CommandResult{Success: false, Error: fmt.Sprintf("unknown command: %s", cmd.Name)}
	}

	result := s.dispatchAction(actionMsg)
	if result.Err != nil {
		return CommandResult{Success: false, Error: result.Err.Error()}
	}
	return CommandResult{Success: true, Quit: result.Quit}
}

func (s *Session) handleDetach() error {
	s.logger.Infof("detach: session=%s id=%d requested windows=%d ownerMode=%t", s.name, s.id, len(s.windowOrder), s.ref.ownerMode)
	snapshot := s.buildSnapshot()

	// In owner mode, mark snapshot as live
	if s.ref.ownerMode {
		if err := MarkSnapshotLive(snapshot, os.Getpid(), SessionSocketPath(s.name)); err != nil {
			s.logger.Warnf("detach: session=%s failed to mark live: %v", s.name, err)
		}
	}

	path := SessionSnapshotPath(s.name)
	s.logger.Infof("detach: session=%s id=%d snapshot-built windows=%d activeWindow=%d path=%s", s.name, s.id, len(snapshot.Windows), snapshot.ActiveWindow, path)
	if err := SaveSnapshot(path, snapshot, s.logger); err != nil {
		return fmt.Errorf("save snapshot %q: %w", path, err)
	}

	// In owner mode, just save - don't stop windows or the session
	if s.ref.ownerMode {
		s.logger.Infof("detach: session=%s id=%d owner saved and staying alive", s.name, s.id)
		return nil
	}

	// Non-owner mode: traditional detach behavior (save and stop)
	for _, win := range s.windows {
		win.Stop()
	}

	s.notifyEvent(SessionEmpty{ID: s.id})
	s.logger.Infof("detach: session=%s id=%d stopping loop", s.name, s.id)
	s.ref.Stop()
	return nil
}

func (s *Session) buildSnapshot() *SessionSnapshot {
	snapshot := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  s.name,
		ID:           s.id,
		Shell:        s.shell,
		ActiveWindow: s.active,
		WindowOrder:  s.windowOrder.Clone(),
		Windows:      make([]WindowSnapshot, 0, len(s.windowOrder)),
	}

	for _, winID := range s.windowOrder {
		win := s.windows[winID]
		if win == nil {
			continue
		}

		result, _ := askValue(win, GetWindowSnapshotData{})
		winData, ok := result.(WindowSnapshot)
		if !ok {
			continue
		}

		snapshot.Windows = append(snapshot.Windows, winData)
	}

	return snapshot
}

func (s *Session) activeWindow() *WindowRef {
	if s.active == 0 {
		return nil
	}
	return s.windows[s.active]
}

func (s *Session) setActiveWindow(id uint32) bool {
	if id == 0 || id == s.active {
		return false
	}
	if _, ok := s.windows[id]; !ok {
		panic(fmt.Sprintf("session %d: setActiveWindow targets missing window %d", s.id, id))
	}
	prev := s.active
	s.active = id
	if prev != 0 {
		s.lastActive = prev
	}
	s.forwardUpdate(PaneContentUpdated{})
	s.assertInvariants()
	return true
}

func (s *Session) killActiveWindow() {
	if s.active == 0 {
		return
	}
	id := s.active
	win := s.windows[id]
	if win == nil {
		return
	}
	win.Shutdown()
	s.handleWindowEmpty(id)
}

// killSession kills all windows and stops the session.
// This is the "hard quit" that doesn't save state.
func (s *Session) killSession() ActionResult {
	s.logger.Infof("kill-session: session=%s id=%d windows=%d", s.name, s.id, len(s.windowOrder))

	// Kill all windows
	for _, win := range s.windows {
		if win != nil {
			win.Shutdown()
		}
	}

	// Clear all window state
	s.windows = make(map[uint32]*WindowRef)
	s.windowOrder = OrderedIDList{}
	s.active = 0
	s.lastActive = 0

	// Mark session empty
	s.notifyEvent(SessionEmpty{ID: s.id})

	// Stop the session loop
	s.logger.Infof("kill-session: session=%s id=%d stopping loop", s.name, s.id)
	s.ref.Stop()

	return ActionResult{Quit: true}
}

func (s *Session) resizeActiveWindow(rows, cols int) {
	s.logger.Infof("session: id=%d name=%s resize-active-window rows=%d cols=%d", s.id, s.name, rows, cols)
	s.lastRows, s.lastCols = rows, cols
	if win := s.activeWindow(); win != nil {
		win.Send(ResizeMsg{Rows: rows, Cols: cols})
	}
}

func (s *Session) createWindow(rows, cols int) {
	s.windowID++
	s.logger.Infof("session: id=%d name=%s create-window window=%d rows=%d cols=%d shell=%s", s.id, s.name, s.windowID, rows, cols, s.shell)
	ref := StartWindow(s.windowID, s.ref, s.logger)
	s.windows[s.windowID] = ref
	s.windowOrder.Add(s.windowID)
	ref.Send(CreatePane{Rows: rows, Cols: cols, Shell: s.shell})
	if s.active == 0 {
		s.active = s.windowID
	}
	s.assertInvariants()
}

func (s *Session) switchWindow(delta int) {
	if len(s.windowOrder) == 0 || delta == 0 {
		return
	}

	currentIdx := s.windowOrder.IndexOf(s.active)
	if currentIdx < 0 {
		currentIdx = 0
	}

	newIdx := (currentIdx + delta) % len(s.windowOrder)
	if newIdx < 0 {
		newIdx += len(s.windowOrder)
	}
	newActive := s.windowOrder[newIdx]
	oldActive := s.active
	if !s.setActiveWindow(newActive) {
		return
	}
	s.logger.Infof("session: id=%d name=%s switch-window from=%d to=%d delta=%d", s.id, s.name, oldActive, newActive, delta)
}

func (s *Session) handleWindowEmpty(id uint32) {
	// Verify window exists before removal (invariant check)
	if _, ok := s.windows[id]; !ok {
		panic(fmt.Sprintf("session %d: handleWindowEmpty called for missing window %d", s.id, id))
	}

	currentIdx := s.windowOrder.IndexOf(s.active)
	delete(s.windows, id)
	s.windowOrder.Remove(id)
	if s.lastActive == id {
		s.lastActive = 0
	}

	if len(s.windowOrder) == 0 {
		s.active = 0
		s.lastActive = 0
		empty := SessionEmpty{ID: s.id}
		s.notifyEvent(empty)
		s.assertInvariants()
		return
	}

	if s.active != id {
		s.assertInvariants()
		return
	}

	if currentIdx >= len(s.windowOrder) {
		currentIdx = len(s.windowOrder) - 1
	}
	if currentIdx < 0 {
		currentIdx = 0
	}
	s.active = s.windowOrder[currentIdx]
	s.forwardUpdate(PaneContentUpdated{})
	s.assertInvariants()
}

func (s *Session) forwardToActivePane(msg any) {
	if win := s.activeWindow(); win != nil {
		win.Send(msg)
	}
}

func (s *Session) forwardToActiveWindow(msg any) {
	if win := s.activeWindow(); win != nil {
		win.Send(msg)
	}
}

func (s *Session) forwardUpdate(msg PaneContentUpdated) {
	s.notifyEvent(msg)
}

func (s *Session) notifyEvent(msg any) {
	if s.notify != nil {
		s.notify(msg)
	}
	s.notifySubscribers(msg)
}

func (s *Session) notifySubscribers(msg any) {
	for subscriber := range s.subscribers {
		select {
		case subscriber <- msg:
		default:
		}
	}
}
