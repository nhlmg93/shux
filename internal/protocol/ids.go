package protocol

type ClientID string
type SessionID string
type WindowID string
type PaneID string

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
