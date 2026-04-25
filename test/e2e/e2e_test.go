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

// ctrlC is ETX (ASCII 3), what a TTY sends for ctrl+c.
const ctrlC = 0x03

func TestShuxRun_rendersTitleAndQuitsOnCtrlC(t *testing.T) {
	s, err := shux.NewShux()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	r, w := io.Pipe()
	go func() {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte{ctrlC})
		_ = w.Close()
	}()

	var out bytes.Buffer
	err = s.Run(
		tea.WithContext(ctx),
		tea.WithInput(r),
		tea.WithOutput(&out),
	)
	if err != nil {
		t.Fatal(err)
	}

	if out.Len() == 0 {
		t.Fatal("expected some terminal output from the program")
	}
	if s.SessionID != protocol.SessionID("s-1") || s.WindowID != protocol.WindowID("w-1") || s.PaneID != protocol.PaneID("p-1") {
		t.Fatalf("ids = %q %q %q", s.SessionID, s.WindowID, s.PaneID)
	}
}
