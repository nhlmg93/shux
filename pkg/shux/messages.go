package shux

// Message types for loop communication.

// Session messages.
type CreateWindow struct {
	Rows int
	Cols int
}

type (
	WindowEmpty     struct{ ID uint32 }
	SwitchWindow    struct{ Delta int }
	GetActiveWindow struct{}
	SessionEmpty    struct{ ID uint32 }
)

// Split directions for pane splitting.
type SplitDir int

const (
	SplitH SplitDir = iota // horizontal divider: panes stacked top/bottom
	SplitV                 // vertical divider: panes side by side
)

type Split struct {
	Dir SplitDir
}

type PaneNavDir int

const (
	PaneNavLeft PaneNavDir = iota
	PaneNavDown
	PaneNavUp
	PaneNavRight
)

type NavigatePane struct {
	Dir PaneNavDir
}

type (
	SubscribeUpdates   struct{ Subscriber chan any }
	UnsubscribeUpdates struct{ Subscriber chan any }
)

// Pane messages (terminal panes within windows).
type CreatePane struct {
	ID    uint32
	Rows  int
	Cols  int
	Shell string
	CWD   string
}

type RestoreWindowLayout struct {
	Root       *SplitTreeSnapshot
	ActivePane uint32
}

type (
	KillPane       struct{}
	PaneExited     struct{ ID uint32 }
	SwitchToPane   struct{ Index int }
	WriteToPane    struct{ Data []byte }
	GetActivePane  struct{}
	GetPaneContent struct{}
	GetPaneMode    struct{}
	GetPaneShell   struct{}
)

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

// KeyInput is a normalized keyboard event sent through the session/window/pane loops.
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

// Snapshot data gathering requests.
type GetSessionSnapshotData struct{}

// SessionSnapshotData is returned by session in response to GetSessionSnapshotData.
// Contains session-level info; window data is gathered via separate requests.
type SessionSnapshotData struct {
	ID           uint32
	Shell        string
	ActiveWindow uint32
	WindowOrder  []uint32
}

type GetWindowSnapshotData struct{}

// WindowSnapshotData is returned by window in response to GetWindowSnapshotData.
type WindowSnapshotData struct {
	ID         uint32
	ActivePane uint32
	PaneOrder  []uint32
}

// GetWindowView requests the rendered view of the active window.
type GetWindowView struct{}

type WindowView struct {
	Content   string
	CursorRow int
	CursorCol int
	CursorOn  bool
	Title     string
}

type GetPaneSnapshotData struct{}

// PaneSnapshotData is returned by pane in response to GetPaneSnapshotData.
type PaneSnapshotData struct {
	ID          uint32
	Shell       string
	Rows        int
	Cols        int
	CWD         string
	WindowTitle string
}

// DetachSession triggers session save and shutdown.
type DetachSession struct{}

// PaneContentUpdated is sent when active pane content or metadata changes.
type PaneContentUpdated struct {
	ID uint32
}

// ResizeTerm is the pane-specific resize message.
// It uses rows/cols like ResizeMsg but stays explicit at the pane boundary.
type ResizeTerm struct {
	Rows int
	Cols int
}

// ResizeMsg is the common resize message for any Resizable component.
type ResizeMsg struct {
	Rows int
	Cols int
}
