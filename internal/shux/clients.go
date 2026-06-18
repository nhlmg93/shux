package shux

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"
	"shux/internal/protocol"
)

type clientEntry struct {
	program   *tea.Program
	sessionID protocol.SessionID
}

func (a *Shux) registerClient(id protocol.ClientID, sessionID protocol.SessionID, p *tea.Program) {
	if !id.Valid() {
		return
	}
	a.clientsMu.Lock()
	defer a.clientsMu.Unlock()
	a.clients[id] = clientEntry{program: p, sessionID: sessionID}
}

func (a *Shux) unregisterClient(id protocol.ClientID) {
	a.clientsMu.Lock()
	defer a.clientsMu.Unlock()
	delete(a.clients, id)
}

func (a *Shux) setClientSession(id protocol.ClientID, sessionID protocol.SessionID) {
	a.clientsMu.Lock()
	defer a.clientsMu.Unlock()
	if entry, ok := a.clients[id]; ok {
		entry.sessionID = sessionID
		a.clients[id] = entry
	}
}

func (a *Shux) listClients() []protocol.ClientInfo {
	a.clientsMu.Lock()
	defer a.clientsMu.Unlock()
	ids := make([]protocol.ClientID, 0, len(a.clients))
	for id := range a.clients {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]protocol.ClientInfo, 0, len(ids))
	for i, id := range ids {
		out = append(out, protocol.ClientInfo{
			Index:     i + 1,
			ClientID:  id,
			SessionID: a.clients[id].sessionID,
		})
	}
	return out
}

func (a *Shux) resolveClientID(clientSpec string) (protocol.ClientID, error) {
	a.clientsMu.Lock()
	defer a.clientsMu.Unlock()
	if clientSpec != "" {
		id := protocol.ClientID(clientSpec)
		if _, ok := a.clients[id]; !ok {
			return "", fmt.Errorf("shux: unknown client %q", clientSpec)
		}
		return id, nil
	}
	if len(a.clients) == 0 {
		return "", fmt.Errorf("shux: no attached clients")
	}
	if len(a.clients) > 1 {
		return "", fmt.Errorf("shux: multiple clients attached; use -c CLIENT")
	}
	for id := range a.clients {
		return id, nil
	}
	return "", fmt.Errorf("shux: no attached clients")
}

func (a *Shux) clientProgram(id protocol.ClientID) *tea.Program {
	a.clientsMu.Lock()
	defer a.clientsMu.Unlock()
	return a.clients[id].program
}
