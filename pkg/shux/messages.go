package shux

import "github.com/nhlmg93/gotor/actor"

// Message types for actor communication.

// Session messages.
type CreateWindow struct {
	Rows int
	Cols int
}

type WindowEmpty struct{ ID uint32 }
type SwitchWindow struct{ Delta int }
type GetActiveWindow struct{}
type SessionEmpty struct{ ID uint32 }

type SubscribeUpdates struct{ Subscriber *actor.Ref }
type UnsubscribeUpdates struct{ Subscriber *actor.Ref }

// Pane messages (terminal panes within windows).
type CreatePane struct {
	Rows  int
	Cols  int
	Shell string
}

type KillPane struct{}
type PaneExited struct{ ID uint32 }
type SwitchToPane struct{ Index int }
type WriteToPane struct{ Data []byte }
type GetActivePane struct{}
type GetPaneContent struct{}
type GetPaneMode struct{}

// KeyCode values are normalized internal key identifiers for non-printable keys.
const (
	KeyCodeUp rune = 0xE000 + iota
	KeyCodeDown
	KeyCodeRight
	KeyCodeLeft
	KeyCodeHome
	KeyCodeEnd
	KeyCodePageUp
	KeyCodePageDown
	KeyCodeInsert
	KeyCodeDelete
	KeyCodeEnter
	KeyCodeBackspace
	KeyCodeTab
	KeyCodeEscape
	KeyCodeF1
	KeyCodeF2
	KeyCodeF3
	KeyCodeF4
	KeyCodeF5
	KeyCodeF6
	KeyCodeF7
	KeyCodeF8
	KeyCodeF9
	KeyCodeF10
	KeyCodeF11
	KeyCodeF12
)

type KeyMods uint16

const (
	KeyModShift KeyMods = 1 << iota
	KeyModAlt
	KeyModCtrl
	KeyModMeta
	KeyModSuper
)

// KeyInput is a normalized keyboard event sent through the actor hierarchy.
type KeyInput struct {
	Code        rune
	Text        string
	ShiftedCode rune
	BaseCode    rune
	Mods        KeyMods
	IsRepeat    bool
}

// PaneMode contains state information about a pane.
type PaneMode struct {
	InAltScreen  bool
	CursorHidden bool
}

// PaneContentUpdated is sent when active pane content or metadata changes.
type PaneContentUpdated struct {
	ID uint32
}

// ResizeTerm is the specific resize message for Pane actors
// (uses rows/cols like ResizeMsg but kept for explicit pane handling).
type ResizeTerm struct {
	Rows int
	Cols int
}

// ResizeMsg is the common resize message for any Resizable actor.
type ResizeMsg struct {
	Rows int
	Cols int
}
