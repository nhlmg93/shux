package shux

import "shux/internal/cfg"

type Config = cfg.Config

const (
	DefaultShellPath    = cfg.DefaultShellPath
	BashShellPath       = cfg.BashShellPath
	DefaultBindAddr     = cfg.DefaultBindAddr
	DefaultMapLeader    = cfg.DefaultMapLeader
	DefaultScrollback   = cfg.DefaultScrollback
	DefaultJournalMaxMB = cfg.DefaultJournalMaxMB
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
	ActionClosePane       = cfg.ActionClosePane
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
	ActionListKeymaps     = cfg.ActionListKeymaps
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
	EventWindowCreated       = cfg.EventWindowCreated
	EventWindowClosed        = cfg.EventWindowClosed
	EventWindowLayoutChanged = cfg.EventWindowLayoutChanged
)

var NewAutocmdRegistry = cfg.NewAutocmdRegistry

var (
	DefaultConfig = cfg.DefaultConfig
	BashConfig    = cfg.BashConfig
)
