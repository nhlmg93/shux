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

func FuzzDecodeSnapshotMalformed(f *testing.F) {
	// Seed with valid snapshot
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
	validData := buf.Bytes()

	// Add valid snapshot as seed
	f.Add(validData)

	// Add truncated valid snapshot
	for i := 1; i < 50 && i < len(validData); i++ {
		f.Add(validData[:len(validData)-i])
	}

	// Add malformed gob headers
	f.Add([]byte{0x07, 0x00, 0x00})                         // Invalid gob header
	f.Add([]byte{0x0a, 0xff, 0xff, 0xff, 0xff})             // Oversized length
	f.Add([]byte("gob:"))                                   // Partial magic
	f.Add(append([]byte{0x07, 0x00}, []byte("garbage")...)) // Wrong version

	// Add random bytes that might trigger edge cases
	f.Add([]byte{0x00})                               // Single null
	f.Add(make([]byte, 1024))                         // All zeros
	f.Add(bytes.Repeat([]byte{0xff}, 1024))           // All 0xff
	f.Add([]byte("\x07\x00\x00\x00\x00\x00\x00\x00")) // Go version with bad data

	f.Fuzz(func(t *testing.T, data []byte) {
		// Test 1: decodeSnapshot must not panic
		snapshot, err := decodeSnapshot(bytes.NewReader(data))

		// If we got a snapshot, it should either be nil with error or valid with nil error
		if err == nil && snapshot == nil {
			t.Fatal("decodeSnapshot returned nil snapshot without error")
		}

		// Test 2: ValidateSnapshot must also not panic
		if snapshot != nil {
			_ = ValidateSnapshot(snapshot) // Can return error, must not panic
		}
	})
}

func FuzzValidateSnapshot(f *testing.F) {
	// Seed corpus with various valid snapshots
	validSeed := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  "valid",
		ID:           1,
		Shell:        "/bin/sh",
		ActiveWindow: 1,
		WindowOrder:  []uint32{1},
		Windows: []WindowSnapshot{{
			ID:         1,
			ActivePane: 1,
			PaneOrder:  []uint32{1, 2},
			Panes: []PaneSnapshot{
				{ID: 1, Shell: "/bin/sh", Rows: 24, Cols: 80},
				{ID: 2, Shell: "/bin/bash", Rows: 24, Cols: 80},
			},
			Layout: &SplitTreeSnapshot{
				Dir:    SplitV,
				Ratio:  0.5,
				First:  &SplitTreeSnapshot{PaneID: 1},
				Second: &SplitTreeSnapshot{PaneID: 2},
			},
		}},
	}
	var buf bytes.Buffer
	if err := gobEncodeSnapshot(&buf, validSeed); err != nil {
		f.Fatalf("seed encode failed: %v", err)
	}
	f.Add(buf.Bytes())

	// Seed with snapshot that has invalid active window
	invalidActive := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  "invalid-active",
		ID:           1,
		Shell:        "/bin/sh",
		ActiveWindow: 999, // Does not exist
		WindowOrder:  []uint32{1},
		Windows: []WindowSnapshot{{
			ID:         1,
			ActivePane: 1,
			PaneOrder:  []uint32{1},
			Panes: []PaneSnapshot{{
				ID: 1, Shell: "/bin/sh", Rows: 24, Cols: 80,
			}},
		}},
	}
	buf.Reset()
	if err := gobEncodeSnapshot(&buf, invalidActive); err != nil {
		f.Fatalf("seed encode failed: %v", err)
	}
	f.Add(buf.Bytes())

	// Seed with mismatched paneOrder/panes
	mismatched := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  "mismatched",
		ID:           1,
		Shell:        "/bin/sh",
		ActiveWindow: 1,
		WindowOrder:  []uint32{1},
		Windows: []WindowSnapshot{{
			ID:         1,
			ActivePane: 1,
			PaneOrder:  []uint32{1, 2}, // Claims 2 panes
			Panes: []PaneSnapshot{{ // Only 1 pane
				ID: 1, Shell: "/bin/sh", Rows: 24, Cols: 80,
			}},
		}},
	}
	buf.Reset()
	if err := gobEncodeSnapshot(&buf, mismatched); err != nil {
		f.Fatalf("seed encode failed: %v", err)
	}
	f.Add(buf.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		snapshot, err := decodeSnapshot(bytes.NewReader(data))
		if err != nil {
			// Expected for invalid data
			return
		}
		if snapshot == nil {
			return
		}

		// ValidateSnapshot must not panic
		validationErr := ValidateSnapshot(snapshot)

		// If validation passes, snapshot should be structurally sound
		if validationErr == nil {
			// Basic sanity checks
			if snapshot.Version != SnapshotVersion {
				t.Fatalf("validated snapshot has wrong version: %d", snapshot.Version)
			}
			if len(snapshot.WindowOrder) != len(snapshot.Windows) {
				t.Fatalf("validated snapshot has mismatched window counts: order=%d windows=%d",
					len(snapshot.WindowOrder), len(snapshot.Windows))
			}
		}
	})
}

func gobEncodeSnapshot(buf *bytes.Buffer, snapshot *SessionSnapshot) error {
	return gob.NewEncoder(buf).Encode(snapshot)
}
