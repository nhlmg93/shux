package protocol

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	MaxPaneInputTextBytes = 4096
	MaxPanePasteBytes     = 65536
	MaxEntityNameBytes    = 256
)

type Command any

func ValidateCommand(cmd Command) error {
	switch c := cmd.(type) {
	case CommandNoop:
		return nil
	case CommandCreateSession:
		if c.Name != "" && !ValidSessionName(c.Name) {
			return fmt.Errorf("protocol: CommandCreateSession: invalid Name")
		}
		return nil
	case CommandListSessions:
		if c.Reply == nil {
			return fmt.Errorf("protocol: CommandListSessions: nil Reply")
		}
		return nil
	case CommandCreateWindow:
		if err := validateSessionTarget("CommandCreateWindow", c.SessionID); err != nil {
			return err
		}
		if !c.Meta.Empty() && !c.Meta.Valid() {
			return fmt.Errorf("protocol: CommandCreateWindow: invalid Meta")
		}
		if c.Cols != 0 || c.Rows != 0 {
			return validateSize("CommandCreateWindow", c.Cols, c.Rows)
		}
		return nil
	case CommandCreatePane:
		return validateWindowTarget("CommandCreatePane", c.SessionID, c.WindowID)
	case CommandPaneInit:
		return validateSize("CommandPaneInit", c.Cols, c.Rows)
	case CommandPaneResize:
		if err := validatePaneTarget("CommandPaneResize", c.SessionID, c.WindowID, c.PaneID); err != nil {
			return err
		}
		return validateSize("CommandPaneResize", c.Cols, c.Rows)
	case CommandWindowResize:
		if err := validateWindowTarget("CommandWindowResize", c.SessionID, c.WindowID); err != nil {
			return err
		}
		return validateSize("CommandWindowResize", c.Cols, c.Rows)
	case CommandWindowRename:
		if err := validateWindowTarget("CommandWindowRename", c.SessionID, c.WindowID); err != nil {
			return err
		}
		return validateName("CommandWindowRename", c.Name)
	case CommandWindowToggleSyncPanes:
		return validateWindowTarget("CommandWindowToggleSyncPanes", c.SessionID, c.WindowID)
	case CommandWindowClosed:
		return validateWindowTarget("CommandWindowClosed", c.SessionID, c.WindowID)
	case CommandPaneKey:
		if err := validatePaneTarget("CommandPaneKey", c.SessionID, c.WindowID, c.PaneID); err != nil {
			return err
		}
		if !c.Action.Valid() {
			return fmt.Errorf("protocol: CommandPaneKey: invalid Action %d", int(c.Action))
		}
		if !c.Modifiers.Valid() {
			return fmt.Errorf("protocol: CommandPaneKey: invalid Modifiers %d", uint16(c.Modifiers))
		}
		if len(c.Text) > MaxPaneInputTextBytes {
			return fmt.Errorf("protocol: CommandPaneKey: text too large")
		}
		return nil
	case CommandPaneMouse:
		if err := validatePaneTarget("CommandPaneMouse", c.SessionID, c.WindowID, c.PaneID); err != nil {
			return err
		}
		if !c.Action.Valid() {
			return fmt.Errorf("protocol: CommandPaneMouse: invalid Action %d", int(c.Action))
		}
		if !c.Button.Valid() {
			return fmt.Errorf("protocol: CommandPaneMouse: invalid Button %d", int(c.Button))
		}
		if !c.Modifiers.Valid() {
			return fmt.Errorf("protocol: CommandPaneMouse: invalid Modifiers %d", uint16(c.Modifiers))
		}
		if c.CellCol < 0 || c.CellRow < 0 {
			return fmt.Errorf("protocol: CommandPaneMouse: negative coordinate")
		}
		return nil
	case CommandPaneClose:
		return validatePaneTarget("CommandPaneClose", c.SessionID, c.WindowID, c.PaneID)
	case CommandPanePaste:
		if err := validatePaneTarget("CommandPanePaste", c.SessionID, c.WindowID, c.PaneID); err != nil {
			return err
		}
		if len(c.Data) > MaxPanePasteBytes {
			return fmt.Errorf("protocol: CommandPanePaste: data too large")
		}
		return nil
	case CommandPaneScroll:
		if err := validatePaneTarget("CommandPaneScroll", c.SessionID, c.WindowID, c.PaneID); err != nil {
			return err
		}
		if c.Delta == 0 {
			return fmt.Errorf("protocol: CommandPaneScroll: zero delta")
		}
		if c.Delta < -1000 || c.Delta > 1000 {
			return fmt.Errorf("protocol: CommandPaneScroll: delta out of range")
		}
		return nil
	case CommandPaneRename:
		if err := validatePaneTarget("CommandPaneRename", c.SessionID, c.WindowID, c.PaneID); err != nil {
			return err
		}
		return validateName("CommandPaneRename", c.Name)
	case CommandPaneSplit:
		if !c.Meta.Valid() {
			return fmt.Errorf("protocol: CommandPaneSplit: invalid Meta")
		}
		if err := validateWindowTarget("CommandPaneSplit", c.SessionID, c.WindowID); err != nil {
			return err
		}
		if !c.TargetPaneID.Valid() {
			return fmt.Errorf("protocol: CommandPaneSplit: invalid TargetPaneID")
		}
		if !c.Direction.Valid() {
			return fmt.Errorf("protocol: CommandPaneSplit: invalid Direction %d", int(c.Direction))
		}
		return nil
	case CommandPaneFocus:
		if !c.Meta.Valid() {
			return fmt.Errorf("protocol: CommandPaneFocus: invalid Meta")
		}
		if err := validateWindowTarget("CommandPaneFocus", c.SessionID, c.WindowID); err != nil {
			return err
		}
		directionSet := c.Direction.Valid()
		targetSet := c.TargetPaneID.Valid()
		if directionSet == targetSet {
			return fmt.Errorf("protocol: CommandPaneFocus: must set exactly one of Direction or TargetPaneID")
		}
		if directionSet && !c.CurrentPaneID.Valid() {
			return fmt.Errorf("protocol: CommandPaneFocus: invalid CurrentPaneID")
		}
		return nil
	case CommandPaneResizeDelta:
		if !c.Meta.Empty() && !c.Meta.Valid() {
			return fmt.Errorf("protocol: CommandPaneResizeDelta: invalid Meta")
		}
		if err := validateWindowTarget("CommandPaneResizeDelta", c.SessionID, c.WindowID); err != nil {
			return err
		}
		if !c.TargetPaneID.Valid() {
			return fmt.Errorf("protocol: CommandPaneResizeDelta: invalid TargetPaneID")
		}
		if !c.Edge.Valid() {
			return fmt.Errorf("protocol: CommandPaneResizeDelta: invalid Edge %d", int(c.Edge))
		}
		if c.Delta == 0 {
			return fmt.Errorf("protocol: CommandPaneResizeDelta: zero Delta")
		}
		if c.Delta < -1000 || c.Delta > 1000 {
			return fmt.Errorf("protocol: CommandPaneResizeDelta: delta out of range")
		}
		return nil
	case CommandWindowSelectLayout:
		if !c.Meta.Valid() {
			return fmt.Errorf("protocol: CommandWindowSelectLayout: invalid Meta")
		}
		if err := validateWindowTarget("CommandWindowSelectLayout", c.SessionID, c.WindowID); err != nil {
			return err
		}
		if !c.ActivePaneID.Valid() {
			return fmt.Errorf("protocol: CommandWindowSelectLayout: invalid ActivePaneID")
		}
		if !c.Preset.Valid() {
			return fmt.Errorf("protocol: CommandWindowSelectLayout: invalid Preset %q", string(c.Preset))
		}
		return nil
	case CommandPaneSwap:
		if !c.Meta.Valid() {
			return fmt.Errorf("protocol: CommandPaneSwap: invalid Meta")
		}
		if err := validateWindowTarget("CommandPaneSwap", c.SessionID, c.WindowID); err != nil {
			return err
		}
		if !c.PaneID.Valid() {
			return fmt.Errorf("protocol: CommandPaneSwap: invalid PaneID")
		}
		if !c.Direction.Valid() {
			return fmt.Errorf("protocol: CommandPaneSwap: invalid Direction %d", int(c.Direction))
		}
		return nil
	case CommandPaneZoomToggle:
		return validatePaneTarget("CommandPaneZoomToggle", c.SessionID, c.WindowID, c.PaneID)
	case CommandPaneMove:
		if err := validateWindowTarget("CommandPaneMove", c.SessionID, c.SourceWindowID); err != nil {
			return err
		}
		if !c.PaneID.Valid() {
			return fmt.Errorf("protocol: CommandPaneMove: invalid PaneID")
		}
		if c.TargetWindowID != "" && !c.TargetWindowID.Valid() {
			return fmt.Errorf("protocol: CommandPaneMove: invalid TargetWindowID")
		}
		return nil
	default:
		return fmt.Errorf("protocol: unknown command type %T", cmd)
	}
}

type CommandMeta struct {
	ClientID  ClientID
	RequestID RequestID
}

func (m CommandMeta) Valid() bool {
	return m.ClientID.Valid() && m.RequestID.Valid()
}

func (m CommandMeta) Empty() bool {
	return m.ClientID == "" && m.RequestID == 0
}

func validateSessionTarget(name string, sessionID SessionID) error {
	if !sessionID.Valid() {
		return fmt.Errorf("protocol: %s: invalid SessionID", name)
	}
	return nil
}

func validateWindowTarget(name string, sessionID SessionID, windowID WindowID) error {
	if err := validateSessionTarget(name, sessionID); err != nil {
		return err
	}
	if !windowID.Valid() {
		return fmt.Errorf("protocol: %s: invalid WindowID", name)
	}
	return nil
}

func validatePaneTarget(name string, sessionID SessionID, windowID WindowID, paneID PaneID) error {
	if err := validateWindowTarget(name, sessionID, windowID); err != nil {
		return err
	}
	if !paneID.Valid() {
		return fmt.Errorf("protocol: %s: invalid PaneID", name)
	}
	return nil
}

func validateSize(name string, cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return fmt.Errorf("protocol: %s: invalid size %dx%d", name, cols, rows)
	}
	return nil
}

func validateName(name, value string) error {
	if len(value) > MaxEntityNameBytes {
		return fmt.Errorf("protocol: %s: name too large", name)
	}
	return nil
}

// RouteSessionID returns the SessionID a command should be forwarded to.
// Reports false for commands that the supervisor handles directly (CommandNoop,
// CommandCreateSession, CommandListSessions) or for unknown types.
func RouteSessionID(cmd Command) (SessionID, bool) {
	switch c := cmd.(type) {
	case CommandCreateWindow:
		return c.SessionID, true
	case CommandWindowClosed:
		return c.SessionID, true
	case CommandCreatePane:
		return c.SessionID, true
	case CommandWindowResize:
		return c.SessionID, true
	case CommandWindowRename:
		return c.SessionID, true
	case CommandWindowToggleSyncPanes:
		return c.SessionID, true
	case CommandPaneSplit:
		return c.SessionID, true
	case CommandPaneFocus:
		return c.SessionID, true
	case CommandPaneResizeDelta:
		return c.SessionID, true
	case CommandWindowSelectLayout:
		return c.SessionID, true
	case CommandPaneSwap:
		return c.SessionID, true
	case CommandPaneZoomToggle:
		return c.SessionID, true
	case CommandPaneMove:
		return c.SessionID, true
	case CommandPaneClose:
		return c.SessionID, true
	case CommandPaneResize:
		return c.SessionID, true
	case CommandPaneKey:
		return c.SessionID, true
	case CommandPaneMouse:
		return c.SessionID, true
	case CommandPanePaste:
		return c.SessionID, true
	case CommandPaneScroll:
		return c.SessionID, true
	case CommandPaneRename:
		return c.SessionID, true
	}
	return "", false
}

// RoutePaneID returns the PaneID for commands that the window forwards directly
// to a pane actor without further bookkeeping. Reports false for commands the
// window handles itself (CommandPaneSplit, CommandPaneZoomToggle, CommandPaneClose, CommandCreatePane).
func RoutePaneID(cmd Command) (PaneID, bool) {
	switch c := cmd.(type) {
	case CommandPaneResize:
		return c.PaneID, true
	case CommandPaneKey:
		return c.PaneID, true
	case CommandPaneMouse:
		return c.PaneID, true
	case CommandPanePaste:
		return c.PaneID, true
	case CommandPaneScroll:
		return c.PaneID, true
	}
	return "", false
}

// RouteWindowID returns the WindowID a command should be forwarded to.
// Reports false for commands that the session handles directly
// (CommandCreateWindow, CommandWindowClosed) or for unknown types.
func RouteWindowID(cmd Command) (WindowID, bool) {
	switch c := cmd.(type) {
	case CommandCreatePane:
		return c.WindowID, true
	case CommandWindowResize:
		return c.WindowID, true
	case CommandWindowToggleSyncPanes:
		return c.WindowID, true
	case CommandPaneSplit:
		return c.WindowID, true
	case CommandPaneFocus:
		return c.WindowID, true
	case CommandPaneResizeDelta:
		return c.WindowID, true
	case CommandWindowSelectLayout:
		return c.WindowID, true
	case CommandPaneSwap:
		return c.WindowID, true
	case CommandPaneZoomToggle:
		return c.WindowID, true
	case CommandPaneClose:
		return c.WindowID, true
	case CommandPaneResize:
		return c.WindowID, true
	case CommandPaneKey:
		return c.WindowID, true
	case CommandPaneMouse:
		return c.WindowID, true
	case CommandPanePaste:
		return c.WindowID, true
	case CommandPaneScroll:
		return c.WindowID, true
	case CommandPaneRename:
		return c.WindowID, true
	case CommandWindowRename:
		return c.WindowID, true
	}
	return "", false
}

type CommandNoop struct{}

// CommandCreateSession is handled by the supervisor; it spawns a new session actor.
type CommandCreateSession struct {
	// Name is optional. Empty requests the supervisor-assigned default name.
	Name string
	// Reply is optional. When set, supervisor reports the create result.
	Reply chan<- CommandCreateSessionResult
}

type CommandCreateSessionResult struct {
	Session SessionDescriptor
	Err     error
}

type SessionDescriptor struct {
	Name      string
	SessionID SessionID
}

type CommandListSessions struct {
	Reply chan<- []SessionDescriptor
}

func ValidSessionName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	if strings.TrimSpace(name) != name {
		return false
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			continue
		}
		switch r {
		case '-', '_', '.':
			continue
		default:
			return false
		}
	}
	return true
}

// CommandCreateWindow is handled by the session actor. When AutoPane is true
// it also creates the initial pane (tmux new-window behavior). When Meta is set,
// EventWindowCreated echoes it so only the originating client switches windows.
type CommandCreateWindow struct {
	Meta      CommandMeta
	SessionID SessionID
	Cols      uint16
	Rows      uint16
	AutoPane  bool
}

// CommandCreatePane is handled by the window actor; it creates a new pane.
// When sent via the supervisor, SessionID selects the session; the session then
// routes to WindowID. Zero SessionID/WindowID are invalid for routed sends.
type CommandCreatePane struct {
	SessionID SessionID
	WindowID  WindowID
}

// CommandPaneInit creates the pane's libghostty terminal at the given cell size.
// Cols and Rows must be non-zero; as uint16 they cannot be negative. The pane
// actor also panics on double init or if NewTerminal fails (see pane package).
type CommandPaneInit struct {
	Cols uint16
	Rows uint16
}

// SplitDirection is how a pane is divided for a 2-way split.
type SplitDirection int

const (
	// SplitVertical places the new pane to the right of the current pane.
	SplitVertical SplitDirection = iota
	// SplitHorizontal places the new pane below the current pane.
	SplitHorizontal
)

// Valid reports whether d is a known split axis. Unknown numeric values fail
// validation in ValidateCommand for CommandPaneSplit.
func (d SplitDirection) Valid() bool {
	return d == SplitVertical || d == SplitHorizontal
}

// LayoutPreset is a named window layout transform.
type LayoutPreset string

const (
	LayoutPresetEvenHorizontal LayoutPreset = "even-horizontal"
	LayoutPresetEvenVertical   LayoutPreset = "even-vertical"
	LayoutPresetMainHorizontal LayoutPreset = "main-horizontal"
)

func (p LayoutPreset) Valid() bool {
	switch p {
	case LayoutPresetEvenHorizontal, LayoutPresetEvenVertical, LayoutPresetMainHorizontal:
		return true
	default:
		return false
	}
}

// PaneDirection points to an adjacent pane.
type PaneDirection int

const (
	PaneDirectionLeft PaneDirection = iota
	PaneDirectionRight
	PaneDirectionUp
	PaneDirectionDown
)

func (d PaneDirection) Valid() bool {
	return d == PaneDirectionLeft || d == PaneDirectionRight || d == PaneDirectionUp || d == PaneDirectionDown
}

// PaneFocusDirection is the directional target for pane navigation.
type PaneFocusDirection int

const (
	PaneFocusLeft PaneFocusDirection = iota + 1
	PaneFocusRight
	PaneFocusUp
	PaneFocusDown
)

func (d PaneFocusDirection) Valid() bool {
	return d == PaneFocusLeft || d == PaneFocusRight || d == PaneFocusUp || d == PaneFocusDown
}

// CommandPaneResize updates a pane’s libghostty size after the terminal has
// been initialized. Cols and rows must be non-zero. SessionID, WindowID, and
// PaneID select the target pane when routed from the supervisor.
type CommandPaneResize struct {
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
	Cols      uint16
	Rows      uint16
}

// CommandWindowResize reports a new total window size in cells. Handled by
// the window actor to recompute layout and issue pane resizes. SessionID and
// WindowID select the window when routed from the supervisor.
type CommandWindowResize struct {
	SessionID SessionID
	WindowID  WindowID
	Cols      uint16
	Rows      uint16
}

// CommandWindowRename sets a user-visible window name.
type CommandWindowRename struct {
	SessionID SessionID
	WindowID  WindowID
	Name      string
}

// CommandWindowToggleSyncPanes flips the window-local synchronize-panes mode.
// When enabled, CommandPaneKey input is broadcast to every pane in the window.
type CommandWindowToggleSyncPanes struct {
	SessionID SessionID
	WindowID  WindowID
}

// CommandWindowClosed notifies the session that a window is gone and should be
// removed from session-level window bookkeeping.
type CommandWindowClosed struct {
	SessionID SessionID
	WindowID  WindowID
}

// KeyAction is a semantic keyboard event phase.
type KeyAction uint8

const (
	KeyActionPress KeyAction = iota
	KeyActionRelease
	KeyActionRepeat
)

func (a KeyAction) Valid() bool {
	return a == KeyActionPress || a == KeyActionRelease || a == KeyActionRepeat
}

// InputModifiers is a bitset of keyboard/mouse modifiers.
type InputModifiers uint16

const (
	ModifierShift InputModifiers = 1 << iota
	ModifierAlt
	ModifierCtrl
	ModifierMeta
)

func (m InputModifiers) Valid() bool {
	const known = ModifierShift | ModifierAlt | ModifierCtrl | ModifierMeta
	return m&^known == 0
}

// CommandPaneKey carries protocol-owned semantic key input to a pane actor.
// Key is a normalized key name/code supplied by the UI mapping layer; Text is
// bounded UTF-8 associated/printable text when available.
type CommandPaneKey struct {
	SessionID   SessionID
	WindowID    WindowID
	PaneID      PaneID
	Action      KeyAction
	Key         string
	Text        string
	Modifiers   InputModifiers
	BaseKey     string
	ShiftedKey  string
	PhysicalKey string
}

// MouseAction is a semantic mouse event phase.
type MouseAction uint8

const (
	MouseActionPress MouseAction = iota
	MouseActionRelease
	MouseActionMotion
	MouseActionWheel
)

func (a MouseAction) Valid() bool {
	return a == MouseActionPress || a == MouseActionRelease || a == MouseActionMotion || a == MouseActionWheel
}

// MouseButton identifies a mouse button or wheel axis.
type MouseButton uint8

const (
	MouseButtonNone MouseButton = iota
	MouseButtonLeft
	MouseButtonMiddle
	MouseButtonRight
	MouseButtonWheelUp
	MouseButtonWheelDown
	MouseButtonWheelLeft
	MouseButtonWheelRight
)

func (b MouseButton) Valid() bool {
	return b <= MouseButtonWheelRight
}

// CommandPaneMouse carries pane-local cell coordinates to the pane actor.
type CommandPaneMouse struct {
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
	Action    MouseAction
	Button    MouseButton
	Modifiers InputModifiers
	CellCol   int
	CellRow   int
}

// CommandPaneClose closes/kills one pane and removes it from the window layout.
type CommandPaneClose struct {
	Meta      CommandMeta
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
}

// CommandPanePaste carries bounded raw paste bytes to a pane actor.
type CommandPanePaste struct {
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
	Data      []byte
}

// CommandPaneScroll scrolls the libghostty viewport within scrollback history.
// Delta is in rows; negative scrolls up, positive scrolls down.
type CommandPaneScroll struct {
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
	Delta     int
}

// CommandPaneRename sets a user-visible pane name.
type CommandPaneRename struct {
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
	Name      string
}

// CommandPaneSplit requests splitting an explicit target pane. Client-originated
// commands carry metadata so results can be correlated without shared focus.
type CommandPaneSplit struct {
	Meta         CommandMeta
	SessionID    SessionID
	WindowID     WindowID
	TargetPaneID PaneID
	Direction    SplitDirection
}

// CommandPaneFocus requests client-scoped pane focus resolution inside a window.
// Set exactly one of Direction (with CurrentPaneID) or TargetPaneID.
type CommandPaneFocus struct {
	Meta          CommandMeta
	SessionID     SessionID
	WindowID      WindowID
	CurrentPaneID PaneID
	Direction     PaneFocusDirection
	TargetPaneID  PaneID
}

// PaneResizeEdge identifies which pane edge should move for resize delta.
type PaneResizeEdge uint8

const (
	PaneResizeEdgeLeft PaneResizeEdge = iota
	PaneResizeEdgeRight
	PaneResizeEdgeUp
	PaneResizeEdgeDown
)

func (e PaneResizeEdge) Valid() bool {
	return e == PaneResizeEdgeLeft ||
		e == PaneResizeEdgeRight ||
		e == PaneResizeEdgeUp ||
		e == PaneResizeEdgeDown
}

// CommandPaneResizeDelta applies a delta resize to the target pane by moving
// one of its edges. Positive Delta grows toward the selected edge.
type CommandPaneResizeDelta struct {
	Meta         CommandMeta
	SessionID    SessionID
	WindowID     WindowID
	TargetPaneID PaneID
	Edge         PaneResizeEdge
	Delta        int
}

// CommandWindowSelectLayout applies a named layout preset across current panes.
type CommandWindowSelectLayout struct {
	Meta         CommandMeta
	SessionID    SessionID
	WindowID     WindowID
	ActivePaneID PaneID
	Preset       LayoutPreset
}

// CommandPaneSwap swaps a pane with its adjacent neighbor in Direction.
type CommandPaneSwap struct {
	Meta      CommandMeta
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
	Direction PaneDirection
}

// CommandPaneZoomToggle toggles window-local zoom for PaneID.
type CommandPaneZoomToggle struct {
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
}

// CommandPaneMove moves an existing pane between windows without replacing the
// pane actor/PTY. If TargetWindowID is empty, the session breaks PaneID into a
// new window. If TargetWindowID is set, the session joins PaneID into that
// existing window.
type CommandPaneMove struct {
	SessionID      SessionID
	SourceWindowID WindowID
	TargetWindowID WindowID
	PaneID         PaneID
}
