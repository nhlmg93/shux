package protocol

type ClientID string
type SessionID string
type WindowID string
type PaneID string
type RequestID uint64

func (id ClientID) Valid() bool {
	return id != ""
}

func (id SessionID) Valid() bool {
	return id != ""
}

func (id WindowID) Valid() bool {
	return id != ""
}

func (id PaneID) Valid() bool {
	return id != ""
}

func (id RequestID) Valid() bool {
	return id != 0
}
