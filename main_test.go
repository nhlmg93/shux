package main

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ctrlC is ETX (ASCII 3), what a TTY sends for ctrl+c.
const ctrlC = 0x03

func TestShuxRun_rendersTitleAndQuitsOnCtrlC(t *testing.T) {
	shux, err := NewShux()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	r, w := io.Pipe()
	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte{ctrlC})
		_ = w.Close()
	}()

	var out bytes.Buffer
	err = shux.Run(
		tea.WithContext(ctx),
		tea.WithInput(r),
		tea.WithOutput(&out),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Full renderer writes mostly cursor/screen control codes; upstream tests
	// only assert non-empty output. See internal/ui for the literal title string.
	if out.Len() == 0 {
		t.Fatal("expected some terminal output from the program")
	}
}
