package sim

import (
	"testing"

	"github.com/mitchellh/go-libghostty"
)

// TestTestBed_LibghosttyVT checks that the sim test bed (CGO, Ghostty lib-vt on PKG_CONFIG_PATH)
// is wired correctly. It does not cover full shux behavior; see test/e2e.
func TestTestBed_LibghosttyVT(t *testing.T) {
	term, err := libghostty.NewTerminal(libghostty.WithSize(80, 24))
	if err != nil {
		t.Fatal(err)
	}
	if term == nil {
		t.Fatal("NewTerminal: expected non-nil *Terminal")
	}
	defer term.Close()
}
