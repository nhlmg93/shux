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
	QueryListWindows    QueryMethod = "list-windows"
	QueryListPanes      QueryMethod = "list-panes"
	QueryDisplayMessage             = "display-message"
)

type QueryRequest struct {
	Method QueryMethod `json:"method"`
	Format string      `json:"format,omitempty"`
}

type QueryResponse struct {
	Windows []WindowInfo        `json:"windows,omitempty"`
	Panes   []PaneInfo          `json:"panes,omitempty"`
	Display *DisplayMessageInfo `json:"display,omitempty"`
}
