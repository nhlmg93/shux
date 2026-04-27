package e2e

import (
	"bytes"
	"context"
	"io"
	"sync"
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
	out := newReadyOutput()
	go func() {
		out.waitReady(time.Second)
		_, _ = w.Write([]byte{ctrlB, 'd'})
		_ = w.Close()
	}()

	err = s.Run(
		tea.WithContext(ctx),
		tea.WithInput(r),
		tea.WithOutput(out),
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
	out := newReadyOutput()
	go func() {
		out.waitReady(time.Second)
		_, _ = w.Write([]byte("\x1b[200~printf shux-e2e-ok\\n\n\x1b[201~"))
		out.waitContains([]byte("shux-e2e-ok"), time.Second)
		_, _ = w.Write([]byte{ctrlB, 'd'})
		_ = w.Close()
	}()

	err = s.Run(
		tea.WithContext(ctx),
		tea.WithInput(r),
		tea.WithOutput(out),
		tea.WithWindowSize(80, 24),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("shux-e2e-ok")) {
		t.Fatalf("expected command output in UI buffer; got %q", out.String())
	}
}

type readyOutput struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	once  sync.Once
	ready chan struct{}
}

func newReadyOutput() *readyOutput {
	return &readyOutput{ready: make(chan struct{})}
}

func (o *readyOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	n, err := o.buf.Write(p)
	o.mu.Unlock()
	if n > 0 {
		o.once.Do(func() { close(o.ready) })
	}
	return n, err
}

func (o *readyOutput) Len() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.Len()
}

func (o *readyOutput) Bytes() []byte {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]byte(nil), o.buf.Bytes()...)
}

func (o *readyOutput) String() string { return string(o.Bytes()) }

func (o *readyOutput) waitReady(timeout time.Duration) {
	select {
	case <-o.ready:
	case <-time.After(timeout):
	}
}

func (o *readyOutput) waitContains(needle []byte, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if bytes.Contains(o.Bytes(), needle) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
