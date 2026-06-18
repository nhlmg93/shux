package ui

import "shux/internal/protocol"

// ClientUIAction triggers in-UI overlays on an attached client.
type ClientUIAction string

const (
	ClientUIActionChooseTree     ClientUIAction = "choose-tree"
	ClientUIActionCommandPrompt  ClientUIAction = "command-prompt"
	ClientUIActionSwitchSession  ClientUIAction = "switch-session"
)

// ClientUITreeMode controls choose-tree initial expansion.
type ClientUITreeMode int

const (
	ClientUITreeDefault ClientUITreeMode = iota
	ClientUITreeSessionsCollapsed
	ClientUITreeWindowsCollapsed
)

// ClientUIMsg is delivered to an attached Bubble Tea client from detached CLI.
type ClientUIMsg struct {
	Action    ClientUIAction
	SessionID protocol.SessionID
	TreeMode  ClientUITreeMode
}
