package shux

import "shux/internal/cfg"

type Config = cfg.Config

const (
	DefaultShellPath              = cfg.DefaultShellPath
	BashShellPath                 = cfg.BashShellPath
	DefaultBindAddr               = cfg.DefaultBindAddr
	DefaultMapLeader              = cfg.DefaultMapLeader
	DefaultScrollback             = cfg.DefaultScrollback
	DefaultMaxSessions            = cfg.DefaultMaxSessions
	DefaultJournalMaxMB           = cfg.DefaultJournalMaxMB
	DefaultPaneQuickSelectTimeout = cfg.DefaultPaneQuickSelectTimeout
)

type BuiltinKeyAction = cfg.BuiltinKeyAction

const (
	ActionDetach          = cfg.ActionDetach
	ActionQuit            = cfg.ActionQuit
	ActionSplitLR         = cfg.ActionSplitLR
	ActionSplitTB         = cfg.ActionSplitTB
	ActionResizePaneLeft  = cfg.ActionResizePaneLeft
	ActionResizePaneDown  = cfg.ActionResizePaneDown
	ActionResizePaneUp    = cfg.ActionResizePaneUp
	ActionResizePaneRight = cfg.ActionResizePaneRight
	ActionNextPane        = cfg.ActionNextPane
	ActionFocusPaneLeft   = cfg.ActionFocusPaneLeft
	ActionFocusPaneRight  = cfg.ActionFocusPaneRight
	ActionFocusPaneUp     = cfg.ActionFocusPaneUp
	ActionFocusPaneDown   = cfg.ActionFocusPaneDown
	ActionDisplayPanes    = cfg.ActionDisplayPanes
	ActionClosePane       = cfg.ActionClosePane
	ActionTogglePaneZoom  = cfg.ActionTogglePaneZoom
	ActionNewWindow       = cfg.ActionNewWindow
	ActionNextWindow      = cfg.ActionNextWindow
	ActionPreviousWindow  = cfg.ActionPreviousWindow
	ActionSelectWindow1   = cfg.ActionSelectWindow1
	ActionSelectWindow2   = cfg.ActionSelectWindow2
	ActionSelectWindow3   = cfg.ActionSelectWindow3
	ActionSelectWindow4   = cfg.ActionSelectWindow4
	ActionSelectWindow5   = cfg.ActionSelectWindow5
	ActionSelectWindow6   = cfg.ActionSelectWindow6
	ActionSelectWindow7   = cfg.ActionSelectWindow7
	ActionSelectWindow8   = cfg.ActionSelectWindow8
	ActionSelectWindow9   = cfg.ActionSelectWindow9
	ActionSelectWindow10  = cfg.ActionSelectWindow10
	ActionToggleSyncPanes = cfg.ActionToggleSyncPanes
	ActionListKeymaps     = cfg.ActionListKeymaps
	ActionRenameWindow    = cfg.ActionRenameWindow
	ActionRenamePane      = cfg.ActionRenamePane
	ActionCopyModeToggle  = cfg.ActionCopyModeToggle
	ActionPasteRegister   = cfg.ActionPasteRegister

	ActionCopyLeft          = cfg.ActionCopyLeft
	ActionCopyDown          = cfg.ActionCopyDown
	ActionCopyUp            = cfg.ActionCopyUp
	ActionCopyRight         = cfg.ActionCopyRight
	ActionCopyWordForward   = cfg.ActionCopyWordForward
	ActionCopyWordBackward  = cfg.ActionCopyWordBackward
	ActionCopyTop           = cfg.ActionCopyTop
	ActionCopyBottom        = cfg.ActionCopyBottom
	ActionCopyPageUp        = cfg.ActionCopyPageUp
	ActionCopyPageDown      = cfg.ActionCopyPageDown
	ActionCopySelectStart   = cfg.ActionCopySelectStart
	ActionCopyYankSelection = cfg.ActionCopyYankSelection
	ActionCopyCancel        = cfg.ActionCopyCancel
)

type KeymapBinding = cfg.KeymapBinding
type Keymaps = cfg.Keymaps

var (
	NewKeymaps      = cfg.NewKeymaps
	DefaultKeymaps  = cfg.DefaultKeymaps
	ExpandLeaderKey = cfg.ExpandLeaderKey
)

type AutocmdEvent = cfg.AutocmdEvent
type AutocmdCallback = cfg.AutocmdCallback
type AutocmdRegistry = cfg.AutocmdRegistry

const (
	EventDaemonStarted       = cfg.EventDaemonStarted
	EventClientAttached      = cfg.EventClientAttached
	EventClientDetached      = cfg.EventClientDetached
	EventPaneCreated         = cfg.EventPaneCreated
	EventPaneClosed          = cfg.EventPaneClosed
	EventPaneRenamed         = cfg.EventPaneRenamed
	EventWindowCreated       = cfg.EventWindowCreated
	EventWindowRenamed       = cfg.EventWindowRenamed
	EventWindowClosed        = cfg.EventWindowClosed
	EventWindowLayoutChanged = cfg.EventWindowLayoutChanged
)

var NewAutocmdRegistry = cfg.NewAutocmdRegistry

var (
	DefaultConfig = cfg.DefaultConfig
	BashConfig    = cfg.BashConfig
)
