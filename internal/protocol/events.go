package protocol

import (
	"context"
	"fmt"
)

const (
	MaxPaneScreenCols = 512
	MaxPaneScreenRows = 512
)

type Event any

func ValidateEvent(event Event) error {
	switch e := event.(type) {
	case EventNoop:
		return nil
	case EventRegisterSubscriber:
		if !e.ClientID.Valid() {
			return fmt.Errorf("protocol: EventRegisterSubscriber: invalid ClientID")
		}
		if e.Sink == nil {
			return fmt.Errorf("protocol: EventRegisterSubscriber: nil Sink")
		}
		return nil
	case EventUnregisterSubscriber:
		if !e.ClientID.Valid() {
			return fmt.Errorf("protocol: EventUnregisterSubscriber: invalid ClientID")
		}
		return nil
	case EventSessionCreated:
		return validateSessionTarget("EventSessionCreated", e.SessionID)
	case EventWindowCreated:
		if err := validateWindowTarget("EventWindowCreated", e.SessionID, e.WindowID); err != nil {
			return err
		}
		if !optionalRequestMetaValid(e.ClientID, e.RequestID) {
			return fmt.Errorf("protocol: EventWindowCreated: invalid optional request meta")
		}
		return nil
	case EventSessionWindowsChanged:
		if err := validateSessionTarget("EventSessionWindowsChanged", e.SessionID); err != nil {
			return err
		}
		if e.Revision == 0 {
			return fmt.Errorf("protocol: EventSessionWindowsChanged: invalid Revision")
		}
		for i, wid := range e.Windows {
			if !wid.Valid() {
				return fmt.Errorf("protocol: EventSessionWindowsChanged: Windows[%d]: invalid WindowID", i)
			}
		}
		return nil
	case EventPaneCreated:
		if err := validateSessionTarget("EventPaneCreated", e.SessionID); err != nil {
			return err
		}
		return validateWindowPaneFields("EventPaneCreated", e.WindowID, e.PaneID)
	case EventWindowClosed:
		return validateWindowTarget("EventWindowClosed", e.SessionID, e.WindowID)
	case EventPaneClosed:
		return validateWindowPaneFields("EventPaneClosed", e.WindowID, e.PaneID)
	case EventPaneCloseLastRequested:
		if err := validateRequestMeta("EventPaneCloseLastRequested", e.ClientID, e.RequestID); err != nil {
			return err
		}
		return validatePaneTarget("EventPaneCloseLastRequested", e.SessionID, e.WindowID, e.PaneID)
	case EventWindowLayoutChanged:
		if err := validateWindowTarget("EventWindowLayoutChanged", e.SessionID, e.WindowID); err != nil {
			return err
		}
		if e.Revision == 0 {
			return fmt.Errorf("protocol: EventWindowLayoutChanged: invalid Revision")
		}
		if err := validateEventSize("EventWindowLayoutChanged", e.Cols, e.Rows); err != nil {
			return err
		}
		for i, p := range e.Panes {
			if !p.PaneID.Valid() {
				return fmt.Errorf("protocol: EventWindowLayoutChanged: Panes[%d]: invalid PaneID", i)
			}
			if p.Cols <= 0 || p.Rows <= 0 {
				return fmt.Errorf("protocol: EventWindowLayoutChanged: Panes[%d]: invalid size", i)
			}
		}
		return nil
	case EventPaneScreenChanged:
		if err := validatePaneTarget("EventPaneScreenChanged", e.SessionID, e.WindowID, e.PaneID); err != nil {
			return err
		}
		if e.Revision == 0 {
			return fmt.Errorf("protocol: EventPaneScreenChanged: invalid Revision")
		}
		if err := validateEventSize("EventPaneScreenChanged", e.Cols, e.Rows); err != nil {
			return err
		}
		if e.Cols > MaxPaneScreenCols || e.Rows > MaxPaneScreenRows {
			return fmt.Errorf("protocol: EventPaneScreenChanged: size %dx%d exceeds max %dx%d", e.Cols, e.Rows, MaxPaneScreenCols, MaxPaneScreenRows)
		}
		if err := e.Cursor.Validate(e.Cols, e.Rows); err != nil {
			return fmt.Errorf("protocol: EventPaneScreenChanged: %w", err)
		}
		if len(e.Lines) > e.Rows {
			return fmt.Errorf("protocol: EventPaneScreenChanged: too many lines %d > %d", len(e.Lines), e.Rows)
		}
		for i, line := range e.Lines {
			if len(line.Cells) > e.Cols {
				return fmt.Errorf("protocol: EventPaneScreenChanged: Lines[%d]: too many cells %d > %d", i, len(line.Cells), e.Cols)
			}
		}
		return nil
	case EventPaneSplitCompleted:
		if err := validateRequestMeta("EventPaneSplitCompleted", e.ClientID, e.RequestID); err != nil {
			return err
		}
		if err := validateWindowTarget("EventPaneSplitCompleted", e.SessionID, e.WindowID); err != nil {
			return err
		}
		if !e.TargetPaneID.Valid() {
			return fmt.Errorf("protocol: EventPaneSplitCompleted: invalid TargetPaneID")
		}
		if !e.NewPaneID.Valid() {
			return fmt.Errorf("protocol: EventPaneSplitCompleted: invalid NewPaneID")
		}
		if e.Revision == 0 {
			return fmt.Errorf("protocol: EventPaneSplitCompleted: invalid Revision")
		}
		return nil
	case EventCommandRejected:
		if err := validateRequestMeta("EventCommandRejected", e.ClientID, e.RequestID); err != nil {
			return err
		}
		if err := validateWindowTarget("EventCommandRejected", e.SessionID, e.WindowID); err != nil {
			return err
		}
		if e.Command == "" {
			return fmt.Errorf("protocol: EventCommandRejected: empty Command")
		}
		if e.Reason == "" {
			return fmt.Errorf("protocol: EventCommandRejected: empty Reason")
		}
		return nil
	default:
		return fmt.Errorf("protocol: unknown event type %T", event)
	}
}

func validateRequestMeta(name string, clientID ClientID, requestID RequestID) error {
	if !clientID.Valid() {
		return fmt.Errorf("protocol: %s: invalid ClientID", name)
	}
	if !requestID.Valid() {
		return fmt.Errorf("protocol: %s: invalid RequestID", name)
	}
	return nil
}

func optionalRequestMetaValid(clientID ClientID, requestID RequestID) bool {
	if clientID == "" && requestID == 0 {
		return true
	}
	return clientID.Valid() && requestID.Valid()
}

func validateWindowPaneFields(name string, windowID WindowID, paneID PaneID) error {
	if !windowID.Valid() {
		return fmt.Errorf("protocol: %s: invalid WindowID", name)
	}
	if !paneID.Valid() {
		return fmt.Errorf("protocol: %s: invalid PaneID", name)
	}
	return nil
}

func validateEventSize(name string, cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("protocol: %s: invalid size %dx%d", name, cols, rows)
	}
	return nil
}

// EventSink receives fanout copies of events from the hub (e.g. actor.Ref[Event] in internal/actor).
type EventSink interface {
	DeliverEvent(ctx context.Context, e Event) error
}

// EventRegisterSubscriber registers Sink under ClientID for hub fanout. Duplicate ClientID is an error (hub panics, matching Init).
type EventRegisterSubscriber struct {
	ClientID ClientID
	Sink     EventSink
}

// EventUnregisterSubscriber removes a fanout client registered with EventRegisterSubscriber.
type EventUnregisterSubscriber struct {
	ClientID ClientID
}

type EventNoop struct{}

// EventSessionCreated is emitted after a session actor exists for SessionID.
type EventSessionCreated struct {
	SessionID SessionID
}

// EventWindowCreated is emitted after a window exists under the session. When
// ClientID/RequestID are set, they identify the client command that created it.
type EventWindowCreated struct {
	ClientID  ClientID
	RequestID RequestID
	SessionID SessionID
	WindowID  WindowID
}

// EventSessionWindowsChanged is emitted when a session's window order changes.
// Active window is intentionally client-local and not part of this event.
type EventSessionWindowsChanged struct {
	SessionID SessionID
	Revision  uint64
	Windows   []WindowID
}

// EventPaneCreated is emitted after a pane exists under the window.
type EventPaneCreated struct {
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
}

// EventWindowClosed is emitted after the last pane in a window is closed.
type EventWindowClosed struct {
	SessionID SessionID
	WindowID  WindowID
}

// EventPaneClosed is emitted after a pane is removed from its window.
type EventPaneClosed struct {
	WindowID WindowID
	PaneID   PaneID
}

// EventPaneCloseLastRequested asks the originating client to exit/quit because
// closing the final pane is equivalent to quitting the session.
type EventPaneCloseLastRequested struct {
	ClientID  ClientID
	RequestID RequestID
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
}

// EventLayoutPane is one pane’s placement in window cell space for UI snapshots.
type EventLayoutPane struct {
	PaneID PaneID
	Col    int
	Row    int
	Cols   int
	Rows   int
}

// EventWindowLayoutChanged is emitted when a window’s cell geometry changes
// (e.g. after resize or split). Hub fanout and publishers are wired separately.
type EventWindowLayoutChanged struct {
	SessionID SessionID
	WindowID  WindowID
	Revision  uint64
	Cols      int
	Rows      int
	Panes     []EventLayoutPane
}

// EventPaneScreenColor is a protocol-owned terminal color description.
// Kind is one of "", "palette", "rgb", or "default". Palette colors use Index;
// RGB colors use R/G/B. The protocol package intentionally does not depend on
// libghostty or UI packages.
type EventPaneScreenColor struct {
	Kind  string
	Index uint8
	R     uint8
	G     uint8
	B     uint8
}

// EventPaneScreenCell is a single styled terminal cell/grapheme in a pane
// viewport snapshot. Text should be bounded by terminal cell semantics at the
// producer; validation bounds the number of cells per line.
type EventPaneScreenCell struct {
	Text          string
	Foreground    EventPaneScreenColor
	Background    EventPaneScreenColor
	Bold          bool
	Italic        bool
	Faint         bool
	Blink         bool
	Inverse       bool
	Invisible     bool
	Underline     bool
	Strikethrough bool
	Overline      bool
}

// EventPaneScreenLine carries plain line text for tests/simple consumers plus
// per-cell style metadata for renderers.
type EventPaneScreenLine struct {
	Text  string
	Cells []EventPaneScreenCell
}

// EventPaneScreenCursor is the terminal cursor state in pane-local cell space.
// Col and Row are zero-based and valid only when Visible is true.
type EventPaneScreenCursor struct {
	Visible bool
	Col     int
	Row     int
	Blink   bool
}

func (c EventPaneScreenCursor) Validate(cols, rows int) error {
	if !c.Visible {
		return nil
	}
	if c.Col < 0 || c.Row < 0 || c.Col >= cols || c.Row >= rows {
		return fmt.Errorf("invalid cursor %d,%d for size %dx%d", c.Col, c.Row, cols, rows)
	}
	return nil
}

func (c EventPaneScreenCursor) WithVisible(visible bool) EventPaneScreenCursor {
	c.Visible = visible
	return c
}

func NewEventPaneScreenCursor(col, row int, blink bool) EventPaneScreenCursor {
	return EventPaneScreenCursor{Visible: true, Col: col, Row: row, Blink: blink}
}

// EventPaneScreenChanged is emitted with the latest bounded viewport snapshot
// for a pane after terminal output or resize.
type EventPaneScreenChanged struct {
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
	Revision  uint64
	Cols      int
	Rows      int
	Lines     []EventPaneScreenLine
	Cursor    EventPaneScreenCursor
}

// EventPaneSplitCompleted correlates an accepted pane split back to the client
// that requested it. Geometry is still published through EventWindowLayoutChanged.
type EventPaneSplitCompleted struct {
	ClientID     ClientID
	RequestID    RequestID
	SessionID    SessionID
	WindowID     WindowID
	TargetPaneID PaneID
	NewPaneID    PaneID
	Revision     uint64
}

// EventCommandRejected reports a bounded command failure without crashing actors.
type EventCommandRejected struct {
	ClientID  ClientID
	RequestID RequestID
	SessionID SessionID
	WindowID  WindowID
	Command   string
	Reason    string
}
