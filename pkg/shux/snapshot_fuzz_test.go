package shux

import (
	"bytes"
	"encoding/gob"
	"testing"
)

func FuzzDecodeSnapshot(f *testing.F) {
	seed := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  "fuzz",
		ID:           1,
		Shell:        "/bin/sh",
		ActiveWindow: 1,
		WindowOrder:  []uint32{1},
		Windows: []WindowSnapshot{{
			ID:         1,
			ActivePane: 1,
			PaneOrder:  []uint32{1},
			Panes: []PaneSnapshot{{
				ID:    1,
				Shell: "/bin/sh",
				Rows:  24,
				Cols:  80,
			}},
		}},
	}

	var buf bytes.Buffer
	if err := gobEncodeSnapshot(&buf, seed); err != nil {
		f.Fatalf("seed encode failed: %v", err)
	}
	f.Add(buf.Bytes())
	f.Add([]byte("not-a-gob"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		snapshot, err := decodeSnapshot(bytes.NewReader(data))
		if err == nil && snapshot == nil {
			t.Fatal("decodeSnapshot returned nil snapshot without error")
		}
	})
}

func gobEncodeSnapshot(buf *bytes.Buffer, snapshot *SessionSnapshot) error {
	return gob.NewEncoder(buf).Encode(snapshot)
}
