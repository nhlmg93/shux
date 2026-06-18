package persist

import (
	"os"
	"sync"
)

// Journal appends PTY output bytes to a pane journal file.
type Journal struct {
	path     string
	f        *os.File
	maxBytes uint64
	mu       sync.Mutex
}

func (j *Journal) Path() string {
	if j == nil {
		return ""
	}
	return j.path
}

func (j *Journal) Append(data []byte) error {
	if j == nil || len(data) == 0 {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, err := j.f.Write(data); err != nil {
		return err
	}
	return j.enforceCapLocked()
}

func (j *Journal) enforceCapLocked() error {
	if j.maxBytes == 0 {
		return nil
	}
	info, err := j.f.Stat()
	if err != nil {
		return err
	}
	if uint64(info.Size()) <= j.maxBytes {
		return nil
	}
	if err := j.f.Close(); err != nil {
		return err
	}
	raw, err := os.ReadFile(j.path)
	if err != nil {
		return err
	}
	if uint64(len(raw)) > j.maxBytes {
		raw = raw[len(raw)-int(j.maxBytes):]
	}
	if err := os.WriteFile(j.path, raw, 0o600); err != nil {
		return err
	}
	f, err := os.OpenFile(j.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	j.f = f
	return nil
}

func (j *Journal) Close() error {
	if j == nil || j.f == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	err := j.f.Close()
	j.f = nil
	return err
}

// ReadJournal returns all bytes recorded in a journal file.
func ReadJournal(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}
