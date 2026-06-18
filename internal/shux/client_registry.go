package shux

import (
	"sync"

	"shux/internal/protocol"
)

type clientInfo struct {
	SessionID protocol.SessionID
}

type clientRegistry struct {
	mu      sync.Mutex
	entries map[protocol.ClientID]clientInfo
}

func newClientRegistry() *clientRegistry {
	return &clientRegistry{entries: make(map[protocol.ClientID]clientInfo)}
}

func (r *clientRegistry) Register(id protocol.ClientID, sessionID protocol.SessionID) {
	if !id.Valid() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[id] = clientInfo{SessionID: sessionID}
}

func (r *clientRegistry) Unregister(id protocol.ClientID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, id)
}

func (r *clientRegistry) SetSession(id protocol.ClientID, sessionID protocol.SessionID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if info, ok := r.entries[id]; ok {
		info.SessionID = sessionID
		r.entries[id] = info
	}
}

func (r *clientRegistry) List() []protocol.ClientInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]protocol.ClientInfo, 0, len(r.entries))
	i := 1
	for id, info := range r.entries {
		out = append(out, protocol.ClientInfo{
			Index:     i,
			ClientID:  id,
			SessionID: info.SessionID,
		})
		i++
	}
	return out
}

func (r *clientRegistry) First() (protocol.ClientID, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id := range r.entries {
		return id, true
	}
	return "", false
}
