package testutil

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Pty is a minimal interface matching Pty for test mocking.
type Pty interface {
	Read(p []byte) (n int, err error)
	Write(p []byte) (n int, err error)
	Close() error
	Resize(rows, cols int) error
	Wait() error
	Kill() error
	PID() int
}

// FakePTY is a mock PTY for deterministic testing.
// It implements the Pty interface and allows controlled behavior.
type FakePTY struct {
	mu         sync.RWMutex
	readBuf    []byte
	readOffset int
	writeBuf   []byte
	pid        int
	closed     bool

	// Configurable behaviors
	ReadErr        error
	WriteErr       error
	ResizeErr      error
	WaitErr        error
	KillErr        error
	CloseErr       error
	ReadDelay      time.Duration
	WriteDelay     time.Duration
	AutoClose      bool // Close after N writes
	writeCount     int
	AutoCloseAfter int
}

// NewFakePTY creates a new fake PTY with the given PID.
func NewFakePTY(pid int) *FakePTY {
	return &FakePTY{
		pid:            pid,
		readBuf:        make([]byte, 0),
		writeBuf:       make([]byte, 0),
		AutoCloseAfter: -1, // Disabled by default
	}
}

// Read implements Pty.
func (f *FakePTY) Read(p []byte) (int, error) {
	f.mu.Lock()
	closed := f.closed
	readErr := f.ReadErr
	delay := f.ReadDelay
	f.mu.Unlock()

	if closed {
		return 0, errors.New("pty closed")
	}
	if readErr != nil {
		return 0, readErr
	}
	if delay > 0 {
		time.Sleep(delay)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.readOffset >= len(f.readBuf) {
		return 0, nil // No data available, non-blocking
	}

	n := copy(p, f.readBuf[f.readOffset:])
	f.readOffset += n
	return n, nil
}

// Write implements Pty.
func (f *FakePTY) Write(p []byte) (int, error) {
	f.mu.Lock()
	closed := f.closed
	writeErr := f.WriteErr
	delay := f.WriteDelay
	f.writeCount++
	shouldAutoClose := f.AutoClose && f.AutoCloseAfter > 0 && f.writeCount >= f.AutoCloseAfter
	f.mu.Unlock()

	if closed {
		return 0, errors.New("pty closed")
	}
	if writeErr != nil {
		return 0, writeErr
	}
	if delay > 0 {
		time.Sleep(delay)
	}

	f.mu.Lock()
	f.writeBuf = append(f.writeBuf, p...)
	f.mu.Unlock()

	if shouldAutoClose {
		_ = f.Close()
	}

	return len(p), nil
}

// Close implements Pty.
func (f *FakePTY) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil
	}
	f.closed = true
	return f.CloseErr
}

// Resize implements Pty.
func (f *FakePTY) Resize(rows, cols int) error {
	f.mu.RLock()
	closed := f.closed
	resizeErr := f.ResizeErr
	f.mu.RUnlock()

	if closed {
		return errors.New("pty closed")
	}
	return resizeErr
}

// Wait implements Pty.
func (f *FakePTY) Wait() error {
	f.mu.RLock()
	waitErr := f.WaitErr
	f.mu.RUnlock()

	// Simulate process waiting by blocking until closed
	for {
		f.mu.RLock()
		closed := f.closed
		f.mu.RUnlock()

		if closed {
			return waitErr
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Kill implements Pty.
func (f *FakePTY) Kill() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closed = true
	return f.KillErr
}

// PID implements Pty.
func (f *FakePTY) PID() int {
	return f.pid
}

// SetReadData sets data that will be returned by Read calls.
func (f *FakePTY) SetReadData(data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.readBuf = data
	f.readOffset = 0
}

// WrittenData returns all data written to the PTY.
func (f *FakePTY) WrittenData() []byte {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]byte, len(f.writeBuf))
	copy(result, f.writeBuf)
	return result
}

// IsClosed reports whether the PTY is closed.
func (f *FakePTY) IsClosed() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.closed
}

// SetReadError sets an error to be returned from Read.
func (f *FakePTY) SetReadError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ReadErr = err
}

// SetWriteError sets an error to be returned from Write.
func (f *FakePTY) SetWriteError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.WriteErr = err
}

// SetResizeError sets an error to be returned from Resize.
func (f *FakePTY) SetResizeError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ResizeErr = err
}

// SetWaitError sets an error to be returned from Wait.
func (f *FakePTY) SetWaitError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.WaitErr = err
}

// FaultInjector provides controlled fault injection for PTY testing.
type FaultInjector struct {
	readFault  atomic.Bool
	writeFault atomic.Bool
	closeFault atomic.Bool
	faultCount atomic.Int32
	maxFaults  int32
}

// NewFaultInjector creates a new fault injector.
func NewFaultInjector() *FaultInjector {
	return &FaultInjector{
		maxFaults: -1, // Unlimited faults by default
	}
}

// SetMaxFaults limits the number of faults that will be injected.
func (f *FaultInjector) SetMaxFaults(n int) {
	f.maxFaults = int32(n)
}

// ShouldInjectRead returns true if a read fault should be injected.
func (f *FaultInjector) ShouldInjectRead() bool {
	if !f.readFault.Load() {
		return false
	}
	if f.maxFaults >= 0 && f.faultCount.Load() >= f.maxFaults {
		return false
	}
	f.faultCount.Add(1)
	return true
}

// ShouldInjectWrite returns true if a write fault should be injected.
func (f *FaultInjector) ShouldInjectWrite() bool {
	if !f.writeFault.Load() {
		return false
	}
	if f.maxFaults >= 0 && f.faultCount.Load() >= f.maxFaults {
		return false
	}
	f.faultCount.Add(1)
	return true
}

// ShouldInjectClose returns true if a close fault should be injected.
func (f *FaultInjector) ShouldInjectClose() bool {
	if !f.closeFault.Load() {
		return false
	}
	if f.maxFaults >= 0 && f.faultCount.Load() >= f.maxFaults {
		return false
	}
	f.faultCount.Add(1)
	return true
}

// EnableReadFault enables read fault injection.
func (f *FaultInjector) EnableReadFault() {
	f.readFault.Store(true)
}

// EnableWriteFault enables write fault injection.
func (f *FaultInjector) EnableWriteFault() {
	f.writeFault.Store(true)
}

// EnableCloseFault enables close fault injection.
func (f *FaultInjector) EnableCloseFault() {
	f.closeFault.Store(true)
}

// DisableAll disables all fault injection.
func (f *FaultInjector) DisableAll() {
	f.readFault.Store(false)
	f.writeFault.Store(false)
	f.closeFault.Store(false)
}

// Reset resets the fault count.
func (f *FaultInjector) Reset() {
	f.faultCount.Store(0)
}

// FaultCount returns the number of faults injected.
func (f *FaultInjector) FaultCount() int {
	return int(f.faultCount.Load())
}

// FaultyPTY wraps a PTY with fault injection capabilities.
type FaultyPTY struct {
	inner    Pty
	injector *FaultInjector
}

// NewFaultyPTY creates a PTY wrapper with fault injection.
func NewFaultyPTY(inner Pty, injector *FaultInjector) *FaultyPTY {
	return &FaultyPTY{
		inner:    inner,
		injector: injector,
	}
}

// Read implements Pty with fault injection.
func (f *FaultyPTY) Read(p []byte) (int, error) {
	if f.injector.ShouldInjectRead() {
		return 0, errors.New("injected read fault")
	}
	return f.inner.Read(p)
}

// Write implements Pty with fault injection.
func (f *FaultyPTY) Write(p []byte) (int, error) {
	if f.injector.ShouldInjectWrite() {
		return 0, errors.New("injected write fault")
	}
	return f.inner.Write(p)
}

// Close implements Pty with fault injection.
func (f *FaultyPTY) Close() error {
	if f.injector.ShouldInjectClose() {
		return errors.New("injected close fault")
	}
	return f.inner.Close()
}

// Resize implements Pty.
func (f *FaultyPTY) Resize(rows, cols int) error {
	return f.inner.Resize(rows, cols)
}

// Wait implements Pty.
func (f *FaultyPTY) Wait() error {
	return f.inner.Wait()
}

// Kill implements Pty.
func (f *FaultyPTY) Kill() error {
	return f.inner.Kill()
}

// PID implements Pty.
func (f *FaultyPTY) PID() int {
	return f.inner.PID()
}

// Compile-time interface check removed to avoid import cycle
// Compile-time interface check removed to avoid import cycle

// PTYRecorder records all PTY operations for later analysis.
type PTYRecorder struct {
	inner   Pty
	mu      sync.Mutex
	records []PTYRecord
}

// PTYRecord represents a single PTY operation.
type PTYRecord struct {
	Op   string
	Data []byte
	Err  error
	Time time.Time
	Rows int
	Cols int
}

// NewPTYRecorder creates a new PTY recorder.
func NewPTYRecorder(inner Pty) *PTYRecorder {
	return &PTYRecorder{
		inner:   inner,
		records: make([]PTYRecord, 0),
	}
}

// Read implements Pty with recording.
func (r *PTYRecorder) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)

	r.mu.Lock()
	defer r.mu.Unlock()

	data := make([]byte, n)
	copy(data, p[:n])
	r.records = append(r.records, PTYRecord{
		Op:   "Read",
		Data: data,
		Err:  err,
		Time: time.Now(),
	})

	return n, err
}

// Write implements Pty with recording.
func (r *PTYRecorder) Write(p []byte) (int, error) {
	n, err := r.inner.Write(p)

	r.mu.Lock()
	defer r.mu.Unlock()

	data := make([]byte, len(p))
	copy(data, p)
	r.records = append(r.records, PTYRecord{
		Op:   "Write",
		Data: data,
		Err:  err,
		Time: time.Now(),
	})

	return n, err
}

// Close implements Pty with recording.
func (r *PTYRecorder) Close() error {
	err := r.inner.Close()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.records = append(r.records, PTYRecord{
		Op:   "Close",
		Err:  err,
		Time: time.Now(),
	})

	return err
}

// Resize implements Pty with recording.
func (r *PTYRecorder) Resize(rows, cols int) error {
	err := r.inner.Resize(rows, cols)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.records = append(r.records, PTYRecord{
		Op:   "Resize",
		Err:  err,
		Time: time.Now(),
		Rows: rows,
		Cols: cols,
	})

	return err
}

// Wait implements Pty with recording.
func (r *PTYRecorder) Wait() error {
	err := r.inner.Wait()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.records = append(r.records, PTYRecord{
		Op:   "Wait",
		Err:  err,
		Time: time.Now(),
	})

	return err
}

// Kill implements Pty with recording.
func (r *PTYRecorder) Kill() error {
	err := r.inner.Kill()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.records = append(r.records, PTYRecord{
		Op:   "Kill",
		Err:  err,
		Time: time.Now(),
	})

	return err
}

// PID implements Pty.
func (r *PTYRecorder) PID() int {
	return r.inner.PID()
}

// Records returns all recorded operations.
func (r *PTYRecorder) Records() []PTYRecord {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]PTYRecord, len(r.records))
	copy(result, r.records)
	return result
}

// RecordCount returns the number of recorded operations.
func (r *PTYRecorder) RecordCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.records)
}

// WriteCount returns the number of write operations.
func (r *PTYRecorder) WriteCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, rec := range r.records {
		if rec.Op == "Write" {
			count++
		}
	}
	return count
}

// Reset clears all records.
func (r *PTYRecorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.records = r.records[:0]
}

// Compile-time interface check removed to avoid import cycle
