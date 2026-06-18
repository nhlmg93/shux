package shux

import (
	"sync"
)

const defaultBufferName = ""

type bufferStore struct {
	mu      sync.Mutex
	entries map[string][]byte
	order   []string
}

func newBufferStore() *bufferStore {
	return &bufferStore{entries: make(map[string][]byte)}
}

func (b *bufferStore) Set(name string, data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.entries[name]; !ok {
		b.order = append(b.order, name)
	}
	b.entries[name] = append([]byte(nil), data...)
}

func (b *bufferStore) Get(name string) ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, ok := b.entries[name]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), data...), true
}

func (b *bufferStore) Delete(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, name)
	for i, n := range b.order {
		if n == name {
			b.order = append(b.order[:i], b.order[i+1:]...)
			break
		}
	}
}

func (b *bufferStore) List() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.order...)
}
