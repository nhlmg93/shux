package persist_test

import (
	"bytes"
	"strings"
	"testing"

	"shux/internal/persist"
)

func TestTailJournalBytes_keepsSmallPayload(t *testing.T) {
	data := []byte("hello\nworld\n")
	got := persist.TailJournalBytes(data, persist.MaxJournalReplayBytes)
	if !bytes.Equal(got, data) {
		t.Fatalf("got %q, want %q", got, data)
	}
}

func TestTailJournalBytes_trimsAtNewlineBoundary(t *testing.T) {
	data := []byte("line1\nline2\nline3\n")
	got := persist.TailJournalBytes(data, 12)
	if string(got) != "line3\n" {
		t.Fatalf("got %q", got)
	}
}

func TestTailJournalBytes_largeJournalUsesTailOnly(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 10000; i++ {
		b.WriteString("row\n")
	}
	data := []byte(b.String())
	got := persist.TailJournalBytes(data, 128)
	if len(got) > 128 {
		t.Fatalf("tail len = %d, want <= 128", len(got))
	}
	if !strings.Contains(string(got), "row") {
		t.Fatalf("tail missing expected content: %q", got)
	}
}
