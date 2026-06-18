package persist

import "bytes"

// MaxJournalReplayBytes caps how much journal data is fed into libghostty on
// replay. Full multi-megabyte replays race with live PTY output and can
// corrupt the VT; the tail preserves recent scrollback.
const MaxJournalReplayBytes = 512 * 1024

// TailJournalBytes returns the last maxBytes of data, trimmed forward to the
// first newline so replay does not start mid-escape-sequence.
func TailJournalBytes(data []byte, maxBytes int) []byte {
	if maxBytes <= 0 || len(data) <= maxBytes {
		return data
	}
	tail := data[len(data)-maxBytes:]
	if idx := bytes.IndexByte(tail, '\n'); idx >= 0 && idx < len(tail)-1 {
		return tail[idx+1:]
	}
	return tail
}
