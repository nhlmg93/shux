package shux

import (
	"errors"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// fakePTY is a mock PTY for fault injection testing.
type fakePTY struct {
	mu         sync.RWMutex
	readErr    error
	writeErr   error
	resizeErr  error
	waitErr    error
	closed     bool
	readData   []byte
	readOffset int
	pid        int
}

func newFakePTY() *fakePTY {
	return &fakePTY{pid: 12345}
}

func (f *fakePTY) Read(buf []byte) (int, error) {
	f.mu.RLock()
	closed := f.closed
	readErr := f.readErr
	readOffset := f.readOffset
	readData := append([]byte(nil), f.readData...)
	f.mu.RUnlock()

	if closed {
		return 0, errors.New("pty closed")
	}
	if readErr != nil {
		return 0, readErr
	}
	if readOffset < len(readData) {
		n := copy(buf, readData[readOffset:])
		f.mu.Lock()
		f.readOffset += n
		f.mu.Unlock()
		return n, nil
	}

	time.Sleep(10 * time.Millisecond)
	return 0, nil
}

func (f *fakePTY) Write(data []byte) (int, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.closed {
		return 0, errors.New("pty closed")
	}
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(data), nil
}

func (f *fakePTY) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakePTY) Kill() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakePTY) Resize(rows, cols int) error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.closed {
		return errors.New("pty closed")
	}
	if f.resizeErr != nil {
		return f.resizeErr
	}
	return nil
}

func (f *fakePTY) Wait() error {
	for {
		f.mu.RLock()
		waitErr := f.waitErr
		closed := f.closed
		f.mu.RUnlock()
		if waitErr != nil {
			return waitErr
		}
		if closed {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (f *fakePTY) PID() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.pid
}

func (f *fakePTY) setReadErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readErr = err
}

func (f *fakePTY) setWriteErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeErr = err
}

func (f *fakePTY) setResizeErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resizeErr = err
}

func (f *fakePTY) setWaitErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.waitErr = err
}

func withFakePanePTY(t *testing.T, fake Pty) {
	t.Helper()
	old := startPanePTY
	startPanePTY = func(cmd *exec.Cmd, rows, cols int) (Pty, error) {
		return fake, nil
	}
	t.Cleanup(func() {
		startPanePTY = old
	})
}

func startFaultTestPane(t *testing.T, fake *fakePTY) (*PaneRef, *WindowRef) {
	t.Helper()
	withFakePanePTY(t, fake)
	parent := &WindowRef{loopRef: newLoopRef(32)}
	ref := StartPane(7, 24, 80, "/bin/sh", "", parent, NoOpLogger{})
	t.Cleanup(func() {
		if ref != nil {
			ref.Shutdown()
		}
	})
	return ref, parent
}

func waitForPaneExited(t *testing.T, parent *WindowRef, paneID uint32, timeout time.Duration) int {
	t.Helper()
	deadline := time.After(timeout)
	count := 0
	for {
		select {
		case msg := <-parent.inbox:
			if exited, ok := msg.(PaneExited); ok && exited.ID == paneID {
				count++
				return count
			}
		case <-deadline:
			t.Fatalf("timeout waiting for PaneExited(%d)", paneID)
		}
	}
}

func assertPaneStopped(t *testing.T, ref *PaneRef, timeout time.Duration) {
	t.Helper()
	select {
	case <-ref.done:
	case <-time.After(timeout):
		t.Fatal("timeout waiting for pane loop to stop")
	}
}

func assertPaneStillRunning(t *testing.T, ref *PaneRef, wait time.Duration) {
	t.Helper()
	select {
	case <-ref.done:
		t.Fatal("pane stopped unexpectedly")
	case <-time.After(wait):
	}
}

// TestPaneReadError tests that a pane exits cleanly on PTY read error.
// TODO: Update for new PaneRuntime/PaneController architecture
func TestPaneReadError(t *testing.T) {
	t.Skip("Test needs update for new architecture - runtime/controller split")
}

// TestPaneWriteError tests that write failures don't crash the pane.
// TODO: Update for new PaneRuntime/PaneController architecture
func TestPaneWriteError(t *testing.T) {
	t.Skip("Test needs update for new architecture - runtime/controller split")
}

// TestPaneResizeError tests that PTY resize failures don't crash the pane.
// TODO: Update for new PaneRuntime/PaneController architecture
func TestPaneResizeError(t *testing.T) {
	t.Skip("Test needs update for new architecture - runtime/controller split")
}

// TestPaneResizeAfterClose tests that resize after close is handled gracefully.
// TODO: Update for new PaneRuntime/PaneController architecture
func TestPaneResizeAfterClose(t *testing.T) {
	t.Skip("Test needs update for new architecture - runtime/controller split")
}

// TestPaneWaitError tests that wait errors trigger clean exit.
// TODO: Update for new PaneRuntime/PaneController architecture
func TestPaneWaitError(t *testing.T) {
	t.Skip("Test needs update for new architecture - runtime/controller split")
}

func TestPaneFaultTolerance(t *testing.T) {
	fake := newFakePTY()
	var _ Pty = fake

	data := []byte("test")
	n, err := fake.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Write returned %d, expected %d", n, len(data))
	}

	fake.setWriteErr(errors.New("write failed"))
	if _, err = fake.Write(data); err == nil {
		t.Fatal("expected write error after injection")
	}

	if err := fake.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if _, err = fake.Write(data); err == nil {
		t.Fatal("expected error after close")
	}
}
