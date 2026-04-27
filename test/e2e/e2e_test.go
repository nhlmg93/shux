package e2e

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"shux/internal/protocol"
	"shux/internal/shux"
)

// ctrlB is STX (ASCII 2), what a TTY sends for ctrl+b.
const ctrlB = 0x02

func TestShuxRun_rendersTitleAndDetachesOnPrefixD(t *testing.T) {
	s, err := shux.NewShux()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	r, w := io.Pipe()
	go func() {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte{ctrlB, 'd'})
		_ = w.Close()
	}()

	var out bytes.Buffer
	err = s.Run(
		tea.WithContext(ctx),
		tea.WithInput(r),
		tea.WithOutput(&out),
		tea.WithWindowSize(80, 24),
	)
	if err != nil {
		t.Fatal(err)
	}

	if out.Len() == 0 {
		t.Fatal("expected some terminal output from the program")
	}
	if s.DefaultSessionID != protocol.SessionID("s-1") || s.DefaultWindowID != protocol.WindowID("w-1") || s.DefaultPaneID != protocol.PaneID("p-1") {
		t.Fatalf("ids = %q %q %q", s.DefaultSessionID, s.DefaultWindowID, s.DefaultPaneID)
	}
}

func TestShuxRun_userCanTypeShellCommandAndSeeOutput(t *testing.T) {
	s, err := shux.NewShux()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	r, w := io.Pipe()
	go func() {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte("\x1b[200~printf shux-e2e-ok\\n\n\x1b[201~"))
		time.Sleep(700 * time.Millisecond)
		_, _ = w.Write([]byte{ctrlB, 'd'})
		_ = w.Close()
	}()

	var out bytes.Buffer
	err = s.Run(
		tea.WithContext(ctx),
		tea.WithInput(r),
		tea.WithOutput(&out),
		tea.WithWindowSize(80, 24),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("shux-e2e-ok")) {
		t.Fatalf("expected command output in UI buffer; got %q", out.String())
	}
}
