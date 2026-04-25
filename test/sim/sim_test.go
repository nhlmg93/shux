package sim

import (
	"context"
	"testing"
	"time"

	"github.com/mitchellh/go-libghostty"
	"shux/internal/protocol"
	"shux/internal/shux"
)

func TestShux_bootstrapsDefaultSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	app, err := shux.NewShux()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	if err := app.BootstrapDefaultSession(ctx); err != nil {
		t.Fatal(err)
	}
	if app.SessionID != protocol.SessionID("s-1") || app.WindowID != protocol.WindowID("w-1") || app.PaneID != protocol.PaneID("p-1") {
		t.Fatalf("ids = %q %q %q", app.SessionID, app.WindowID, app.PaneID)
	}
}

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
