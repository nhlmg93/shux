package pane_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellh/go-libghostty"
	"shux/internal/persist"
)

func TestReplayJournal_rendersRecordedBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pane.journal")
	marker := "SHUX_REPLAY_MARKER"
	if err := os.WriteFile(path, []byte(marker+"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	term, err := libghostty.NewTerminal(libghostty.WithSize(80, 24))
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()

	rs, err := libghostty.NewRenderState()
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()

	if err := persist.ReplayJournal(term, path); err != nil {
		t.Fatal(err)
	}
	if err := rs.Update(term); err != nil {
		t.Fatal(err)
	}

	rowIter, err := libghostty.NewRenderStateRowIterator()
	if err != nil {
		t.Fatal(err)
	}
	defer rowIter.Close()
	cells, err := libghostty.NewRenderStateRowCells()
	if err != nil {
		t.Fatal(err)
	}
	defer cells.Close()
	if err := rs.RowIterator(rowIter); err != nil {
		t.Fatal(err)
	}
	if !rowIter.Next() {
		t.Fatal("expected at least one screen row")
	}
	if err := rowIter.Cells(cells); err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for cells.Next() {
		gr, err := cells.Graphemes()
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range gr {
			text.WriteRune(rune(r))
		}
	}
	if !strings.Contains(text.String(), marker) {
		t.Fatalf("screen missing marker %q: %q", marker, text.String())
	}
}
