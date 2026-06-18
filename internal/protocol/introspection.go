package protocol

type WindowInfo struct {
	Index     int       `json:"index"`
	SessionID SessionID `json:"session_id"`
	WindowID  WindowID  `json:"window_id"`
	PaneCount int       `json:"pane_count"`
}

type PaneInfo struct {
	Index       int       `json:"index"`
	SessionID   SessionID `json:"session_id"`
	WindowID    WindowID  `json:"window_id"`
	WindowIndex int       `json:"window_index"`
	PaneID      PaneID    `json:"pane_id"`
	Col         int       `json:"col"`
	Row         int       `json:"row"`
	Cols        int       `json:"cols"`
	Rows        int       `json:"rows"`
}

type ClientInfo struct {
	Index     int       `json:"index"`
	ClientID  ClientID  `json:"client_id"`
	SessionID SessionID `json:"session_id"`
}

type DisplayMessageContext struct {
	SessionID   SessionID `json:"session_id"`
	WindowID    WindowID  `json:"window_id"`
	WindowIndex int       `json:"window_index"`
	PaneID      PaneID    `json:"pane_id"`
	PaneIndex   int       `json:"pane_index"`
}

type DisplayMessageInfo struct {
	Message string `json:"message"`
	DisplayMessageContext
}

type QueryMethod string

const (
	QueryListWindows     QueryMethod = "list-windows"
	QueryListPanes       QueryMethod = "list-panes"
	QueryDisplayMessage  QueryMethod = "display-message"
	QueryCheckpointState QueryMethod = "checkpoint-state"
	QueryHasSession      QueryMethod = "has-session"
	QueryCapturePane     QueryMethod = "capture-pane"
)

type QueryRequest struct {
	Method      QueryMethod `json:"method"`
	Format      string      `json:"format,omitempty"`
	SessionName string      `json:"session,omitempty"`
	Target      string      `json:"target,omitempty"`
}

type StateCheckpointInfo struct {
	Pruned []string `json:"pruned,omitempty"`
}

type QueryResponse struct {
	Windows    []WindowInfo         `json:"windows,omitempty"`
	Panes      []PaneInfo           `json:"panes,omitempty"`
	Display    *DisplayMessageInfo  `json:"display,omitempty"`
	Checkpoint *StateCheckpointInfo `json:"checkpoint,omitempty"`
	Exists     *bool                `json:"exists,omitempty"`
	Capture    *CapturePaneInfo     `json:"capture,omitempty"`
}

type CapturePaneInfo struct {
	SessionID SessionID `json:"session_id"`
	WindowID  WindowID  `json:"window_id"`
	PaneID    PaneID    `json:"pane_id"`
	Text      string    `json:"text"`
}
