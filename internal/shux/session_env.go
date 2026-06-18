package shux

import (
	"sync"

	"shux/internal/protocol"
)

type sessionEnvStore struct {
	mu   sync.Mutex
	data map[protocol.SessionID]map[string]string
}

func newSessionEnvStore() *sessionEnvStore {
	return &sessionEnvStore{data: make(map[protocol.SessionID]map[string]string)}
}

func (s *sessionEnvStore) Set(sessionID protocol.SessionID, key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vars := s.data[sessionID]
	if vars == nil {
		vars = make(map[string]string)
		s.data[sessionID] = vars
	}
	vars[key] = value
}

func (s *sessionEnvStore) Unset(sessionID protocol.SessionID, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if vars := s.data[sessionID]; vars != nil {
		delete(vars, key)
	}
}

func (s *sessionEnvStore) Get(sessionID protocol.SessionID, key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if vars := s.data[sessionID]; vars != nil {
		v, ok := vars[key]
		return v, ok
	}
	return "", false
}

func (s *sessionEnvStore) List(sessionID protocol.SessionID) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.data[sessionID]
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func (s *sessionEnvStore) RemoveSession(sessionID protocol.SessionID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, sessionID)
}
