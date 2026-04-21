package shux

// IPC message types for owner <-> client communication over Unix socket.
// These are gob-serializable DTOs that mirror the in-process message types.

// Client -> Owner messages

type (
	// IPCActionMsg forwards an action from client to owner.
	IPCActionMsg struct {
		Action Action
		Args   []string
		Amount int
	}

	// IPCKeyInput forwards keyboard input from client to owner.
	IPCKeyInput struct {
		Code        rune
		Text        string
		ShiftedCode rune
		BaseCode    rune
		Mods        KeyMods
		IsRepeat    bool
	}

	// IPCMouseInput forwards mouse input from client to owner.
	IPCMouseInput struct {
		Row    int
		Col    int
		Button MouseButton
		Mods   KeyMods
		Action MouseAction
	}

	// IPCWriteToPane forwards data write to active pane.
	IPCWriteToPane struct {
		Data []byte
	}

	// IPCResizeMsg forwards terminal resize.
	IPCResizeMsg struct {
		Rows int
		Cols int
	}

	// IPCCreateWindow requests creation of a new window.
	IPCCreateWindow struct {
		Rows int
		Cols int
	}

	// IPCSubscribeUpdates requests subscription to window updates.
	IPCSubscribeUpdates struct{}

	// IPCUnsubscribeUpdates cancels subscription.
	IPCUnsubscribeUpdates struct{}

	// IPCGetWindowView requests current window view.
	IPCGetWindowView struct{}

	// IPCGetActiveWindow reports whether an active window exists.
	IPCGetActiveWindow struct{}

	// IPCExecuteCommandMsg executes a command.
	IPCExecuteCommandMsg struct {
		Command string
	}

	// IPCDetachSession requests detach (save snapshot, keep owner running).
	IPCDetachSession struct{}

	// IPCKillSession requests full session teardown.
	IPCKillSession struct{}
)

// Owner -> Client messages

type (
	// IPCWindowView delivers rendered window content to client.
	IPCWindowView struct {
		Content   string
		CursorRow int
		CursorCol int
		CursorOn  bool
		Title     string
	}

	// IPCPaneContentUpdated signals content changed and client should refresh.
	IPCPaneContentUpdated struct {
		ID uint32
	}

	// IPCSessionEmpty signals session has no windows (natural termination).
	IPCSessionEmpty struct {
		ID uint32
	}

	// IPCSessionDetached signals detach completed successfully.
	IPCSessionDetached struct{}

	// IPCSessionKilled signals session was killed.
	IPCSessionKilled struct{}

	// IPCActionResult returns action execution result.
	IPCActionResult struct {
		Quit  bool
		Error string // empty if no error
	}

	// IPCCommandResult returns command execution result.
	IPCCommandResult struct {
		Success bool
		Error   string
		Quit    bool
	}
)

// IPCEnvelope wraps any IPC message with a type discriminator for gob encoding.
type IPCEnvelope struct {
	Type string
	Data any
}
