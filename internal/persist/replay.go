package persist

import (
	"github.com/mitchellh/go-libghostty"
)

// ReplayJournal feeds recorded PTY bytes into a fresh libghostty terminal (Ghostty playback).
func ReplayJournal(term *libghostty.Terminal, path string) error {
	data, err := ReadJournal(path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	data = TailJournalBytes(data, MaxJournalReplayBytes)
	term.VTWrite(data)
	return nil
}
