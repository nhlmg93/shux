package persist

import (
	"os"
	"sync"
)

// Journal appends PTY output bytes to a pane journal file.
type Journal struct {
	path string
	f    *os.File
	mu   sync.Mutex
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
	_, err := j.f.Write(data)
	return err
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
