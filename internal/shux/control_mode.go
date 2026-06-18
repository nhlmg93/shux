package shux

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"shux/internal/protocol"
)

const (
	controlEventBuffer        = 256
	controlInputLineMaxBytes  = 4096
	controlOutputTextMaxBytes = 2048
	controlCaptureMaxBytes    = 8192
	controlCommandTimeout     = time.Second
)

type controlSession struct {
	app        *Shux
	clientID   protocol.ClientID
	sessionID  protocol.SessionID
	activePane protocol.PaneID
	activeWin  protocol.WindowID
	requestID  protocol.RequestID

	subscribeLayout bool
	subscribeOutput bool

	events chan protocol.Event
	writer *bufio.Writer
}

func (a *Shux) RunControlMode(ctx context.Context, clientID protocol.ClientID, in io.Reader, out io.Writer) error {
	if a == nil {
		return fmt.Errorf("shux: nil app")
	}
	if !clientID.Valid() {
		return fmt.Errorf("shux: invalid control client id")
	}
	if in == nil || out == nil {
		return fmt.Errorf("shux: control mode requires stdin/stdout")
	}
	if !a.supervisor.Valid() || !a.hub.Valid() {
		return fmt.Errorf("shux: control mode unavailable before bootstrap")
	}

	s := &controlSession{
		app:        a,
		clientID:   clientID,
		sessionID:  a.DefaultSessionID,
		activeWin:  a.DefaultWindowID,
		activePane: a.DefaultPaneID,
		events:     make(chan protocol.Event, controlEventBuffer),
		writer:     bufio.NewWriter(out),
	}
	s.seedFromCache()
	if err := a.hub.Send(ctx, protocol.EventRegisterSubscriber{
		ClientID: clientID,
		Sink:     protocol.EventChanAdapter(s.events),
	}); err != nil {
		return fmt.Errorf("shux: control register subscriber: %w", err)
	}
	defer a.hub.Send(context.Background(), protocol.EventUnregisterSubscriber{ClientID: clientID})

	if err := s.writeLine("ready session=%s window=%s pane=%s", s.sessionID, s.activeWin, s.activePane); err != nil {
		return err
	}

	lines, readErr := readControlLines(ctx, in)
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-readErr:
			if err == nil || err == io.EOF {
				return nil
			}
			return err
		case line, ok := <-lines:
			if !ok {
				lines = nil
				continue
			}
			if err := s.handleLine(ctx, line); err != nil {
				_ = s.writeLine("error %s", sanitizeMessage(err.Error()))
			}
		case event := <-s.events:
			if err := s.handleEvent(event); err != nil {
				return err
			}
		}
	}
}

func readControlLines(ctx context.Context, in io.Reader) (<-chan string, <-chan error) {
	lines := make(chan string, 32)
	errc := make(chan error, 1)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 256), controlInputLineMaxBytes)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			select {
			case <-ctx.Done():
				errc <- nil
				return
			case lines <- line:
			}
		}
		if err := scanner.Err(); err != nil {
			errc <- err
			return
		}
		errc <- io.EOF
	}()
	return lines, errc
}

func (s *controlSession) handleLine(ctx context.Context, line string) error {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	switch fields[0] {
	case "subscribe":
		return s.handleSubscribe(fields[1:])
	case "new-window":
		return s.handleNewWindow(ctx)
	case "split":
		return s.handleSplit(ctx, fields[1:])
	case "select-pane":
		return s.handleSelectPane(fields[1:])
	case "capture-pane":
		return s.handleCapturePane(fields[1:])
	default:
		return fmt.Errorf("unknown command %q", fields[0])
	}
}

func (s *controlSession) handleSubscribe(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subscribe requires one or more topics")
	}
	for _, arg := range args {
		switch arg {
		case "all":
			s.subscribeLayout = true
			s.subscribeOutput = true
		case "layout", "layout-change":
			s.subscribeLayout = true
		case "pane-output", "output":
			s.subscribeOutput = true
		default:
			return fmt.Errorf("unknown subscribe topic %q", arg)
		}
	}
	return s.writeLine("ok subscribe layout=%t output=%t", s.subscribeLayout, s.subscribeOutput)
}

func (s *controlSession) handleNewWindow(ctx context.Context) error {
	req := s.nextRequest()
	cols, rows := s.currentWindowSize()
	cmd := protocol.CommandCreateWindow{
		Meta:      protocol.CommandMeta{ClientID: s.clientID, RequestID: req},
		SessionID: s.sessionID,
		Cols:      cols,
		Rows:      rows,
		AutoPane:  true,
	}
	if err := s.app.supervisor.Send(ctx, cmd); err != nil {
		return err
	}
	created, err := s.waitForWindowCreated(ctx, req)
	if err != nil {
		return err
	}
	s.activeWin = created.WindowID
	if paneID, ok := s.firstPaneInWindow(s.activeWin); ok {
		s.activePane = paneID
	}
	return s.writeLine("ok new-window window=%s pane=%s", s.activeWin, s.activePane)
}

func (s *controlSession) handleSplit(ctx context.Context, args []string) error {
	dir := protocol.SplitVertical
	target := s.activePane
	for _, arg := range args {
		switch arg {
		case "vertical", "-v":
			dir = protocol.SplitVertical
		case "horizontal", "-h":
			dir = protocol.SplitHorizontal
		default:
			target = protocol.PaneID(arg)
		}
	}
	if !target.Valid() {
		return fmt.Errorf("split target pane is not set")
	}
	windowID, ok := s.windowForPane(target)
	if !ok {
		return fmt.Errorf("unknown pane %q", target)
	}
	req := s.nextRequest()
	cmd := protocol.CommandPaneSplit{
		Meta:         protocol.CommandMeta{ClientID: s.clientID, RequestID: req},
		SessionID:    s.sessionID,
		WindowID:     windowID,
		TargetPaneID: target,
		Direction:    dir,
	}
	if err := s.app.supervisor.Send(ctx, cmd); err != nil {
		return err
	}
	done, err := s.waitForSplitCompleted(ctx, req)
	if err != nil {
		return err
	}
	s.activeWin = done.WindowID
	s.activePane = done.NewPaneID
	return s.writeLine("ok split window=%s pane=%s", s.activeWin, s.activePane)
}

func (s *controlSession) handleSelectPane(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("select-pane requires exactly one pane id")
	}
	paneID := protocol.PaneID(args[0])
	if !paneID.Valid() {
		return fmt.Errorf("invalid pane id")
	}
	windowID, ok := s.windowForPane(paneID)
	if !ok {
		return fmt.Errorf("unknown pane %q", paneID)
	}
	s.activeWin = windowID
	s.activePane = paneID
	return s.writeLine("ok select-pane window=%s pane=%s", s.activeWin, s.activePane)
}

func (s *controlSession) handleCapturePane(args []string) error {
	paneID := s.activePane
	if len(args) == 1 {
		paneID = protocol.PaneID(args[0])
	} else if len(args) > 1 {
		return fmt.Errorf("capture-pane accepts at most one pane id")
	}
	if !paneID.Valid() {
		return fmt.Errorf("capture-pane target pane is not set")
	}
	windowID, ok := s.windowForPane(paneID)
	if !ok {
		return fmt.Errorf("unknown pane %q", paneID)
	}
	screen, ok := s.findScreen(windowID, paneID)
	if !ok {
		return fmt.Errorf("pane %q has no cached screen", paneID)
	}
	text := quoteText(screenText(screen, controlCaptureMaxBytes))
	return s.writeLine("ok capture-pane window=%s pane=%s text=%s", windowID, paneID, text)
}

func (s *controlSession) handleEvent(event protocol.Event) error {
	switch e := event.(type) {
	case protocol.EventSessionWindowsChanged:
		if e.SessionID == s.sessionID && len(e.Windows) > 0 && !slices.Contains(e.Windows, s.activeWin) {
			s.activeWin = e.Windows[0]
		}
	case protocol.EventWindowClosed:
		if e.SessionID == s.sessionID && e.WindowID == s.activeWin {
			if windows := s.app.cache.WindowIDs(s.sessionID); len(windows) > 0 {
				s.activeWin = windows[0]
			}
		}
	case protocol.EventWindowLayoutChanged:
		if e.SessionID != s.sessionID {
			return nil
		}
		if e.WindowID == s.activeWin && !s.layoutHasPane(e, s.activePane) && len(e.Panes) > 0 {
			s.activePane = e.Panes[0].PaneID
		}
		if s.subscribeLayout {
			return s.writeLayoutEvent(e)
		}
	case protocol.EventPaneScreenChanged:
		if e.SessionID != s.sessionID || !s.subscribeOutput {
			return nil
		}
		text := quoteText(screenText(e, controlOutputTextMaxBytes))
		return s.writeLine("%%output session=%s window=%s pane=%s revision=%d text=%s", e.SessionID, e.WindowID, e.PaneID, e.Revision, text)
	}
	return nil
}

func (s *controlSession) writeLayoutEvent(e protocol.EventWindowLayoutChanged) error {
	var panes []string
	for _, pane := range e.Panes {
		panes = append(panes, fmt.Sprintf("%s@%d,%d,%d,%d", pane.PaneID, pane.Col, pane.Row, pane.Cols, pane.Rows))
	}
	return s.writeLine("%%layout session=%s window=%s revision=%d panes=%s", e.SessionID, e.WindowID, e.Revision, strings.Join(panes, ";"))
}

func (s *controlSession) writeLine(pattern string, args ...any) error {
	if _, err := fmt.Fprintf(s.writer, pattern+"\n", args...); err != nil {
		return err
	}
	return s.writer.Flush()
}

func (s *controlSession) waitForWindowCreated(ctx context.Context, requestID protocol.RequestID) (protocol.EventWindowCreated, error) {
	deadline := time.NewTimer(controlCommandTimeout)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return protocol.EventWindowCreated{}, ctx.Err()
		case <-deadline.C:
			return protocol.EventWindowCreated{}, fmt.Errorf("new-window timed out")
		case event := <-s.events:
			if err := s.handleEvent(event); err != nil {
				return protocol.EventWindowCreated{}, err
			}
			switch e := event.(type) {
			case protocol.EventWindowCreated:
				if e.ClientID == s.clientID && e.RequestID == requestID {
					return e, nil
				}
			case protocol.EventCommandRejected:
				if e.ClientID == s.clientID && e.RequestID == requestID {
					return protocol.EventWindowCreated{}, fmt.Errorf("%s: %s", e.Command, e.Reason)
				}
			}
		}
	}
}

func (s *controlSession) waitForSplitCompleted(ctx context.Context, requestID protocol.RequestID) (protocol.EventPaneSplitCompleted, error) {
	deadline := time.NewTimer(controlCommandTimeout)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return protocol.EventPaneSplitCompleted{}, ctx.Err()
		case <-deadline.C:
			return protocol.EventPaneSplitCompleted{}, fmt.Errorf("split timed out")
		case event := <-s.events:
			if err := s.handleEvent(event); err != nil {
				return protocol.EventPaneSplitCompleted{}, err
			}
			switch e := event.(type) {
			case protocol.EventPaneSplitCompleted:
				if e.ClientID == s.clientID && e.RequestID == requestID {
					return e, nil
				}
			case protocol.EventCommandRejected:
				if e.ClientID == s.clientID && e.RequestID == requestID {
					return protocol.EventPaneSplitCompleted{}, fmt.Errorf("%s: %s", e.Command, e.Reason)
				}
			}
		}
	}
}

func (s *controlSession) layoutHasPane(layout protocol.EventWindowLayoutChanged, paneID protocol.PaneID) bool {
	for _, pane := range layout.Panes {
		if pane.PaneID == paneID {
			return true
		}
	}
	return false
}

func (s *controlSession) currentWindowSize() (uint16, uint16) {
	layout, ok := s.app.cache.LayoutSnapshot(s.sessionID, s.activeWin)
	if !ok || layout.Cols <= 0 || layout.Rows <= 0 || layout.Cols > 0xFFFF || layout.Rows > 0xFFFF {
		return 0, 0
	}
	return uint16(layout.Cols), uint16(layout.Rows)
}

func (s *controlSession) firstPaneInWindow(windowID protocol.WindowID) (protocol.PaneID, bool) {
	layout, ok := s.app.cache.LayoutSnapshot(s.sessionID, windowID)
	if !ok || len(layout.Panes) == 0 {
		return "", false
	}
	return layout.Panes[0].PaneID, true
}

func (s *controlSession) windowForPane(paneID protocol.PaneID) (protocol.WindowID, bool) {
	for _, windowID := range s.app.cache.WindowIDs(s.sessionID) {
		layout, ok := s.app.cache.LayoutSnapshot(s.sessionID, windowID)
		if !ok {
			continue
		}
		for _, pane := range layout.Panes {
			if pane.PaneID == paneID {
				return windowID, true
			}
		}
	}
	return "", false
}

func (s *controlSession) findScreen(windowID protocol.WindowID, paneID protocol.PaneID) (protocol.EventPaneScreenChanged, bool) {
	for _, screen := range s.app.cache.ScreenSnapshots(s.sessionID, windowID) {
		if screen.PaneID == paneID {
			return screen, true
		}
	}
	return protocol.EventPaneScreenChanged{}, false
}

func (s *controlSession) seedFromCache() {
	windows := s.app.cache.WindowIDs(s.sessionID)
	if len(windows) > 0 {
		s.activeWin = windows[0]
	}
	if pane, ok := s.firstPaneInWindow(s.activeWin); ok {
		s.activePane = pane
	}
}

func (s *controlSession) nextRequest() protocol.RequestID {
	s.requestID++
	return s.requestID
}

func screenText(screen protocol.EventPaneScreenChanged, maxBytes int) string {
	var b strings.Builder
	for i, line := range screen.Lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line.Text)
		if b.Len() >= maxBytes {
			break
		}
	}
	text := b.String()
	if len(text) > maxBytes {
		return text[:maxBytes]
	}
	return text
}

func quoteText(text string) string {
	return strconv.QuoteToASCII(text)
}

func sanitizeMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "request failed"
	}
	return strings.ReplaceAll(msg, "\n", " ")
}
