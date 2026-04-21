package testutil

import (
	"sync"
	"time"
)

// FakeClock provides deterministic time control for tests.
// This enables deterministic simulation testing as per AGENTS.md philosophy.
type FakeClock struct {
	mu       sync.RWMutex
	now      time.Time
	timers   []*fakeTimer
	frozen   bool
	autoTick time.Duration // If set, advances by this amount on each Now() call
}

// NewFakeClock creates a new fake clock starting at the given time.
func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{
		now:    start,
		timers: make([]*fakeTimer, 0),
	}
}

// NewFakeClockUnix creates a new fake clock starting at a Unix timestamp.
func NewFakeClockUnix(sec int64) *FakeClock {
	return NewFakeClock(time.Unix(sec, 0))
}

// Now returns the current fake time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.autoTick > 0 {
		c.now = c.now.Add(c.autoTick)
	}
	return c.now
}

// Advance moves the fake clock forward by the given duration.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)
	c.fireTimers()
}

// Set sets the fake clock to a specific time.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = t
	c.fireTimers()
}

// SetAutoTick enables automatic time advancement on each Now() call.
func (c *FakeClock) SetAutoTick(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.autoTick = d
}

// Freeze stops time from advancing automatically.
func (c *FakeClock) Freeze() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.frozen = true
	c.autoTick = 0
}

// Unfreeze allows time to advance.
func (c *FakeClock) Unfreeze() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.frozen = false
}

// Since returns the time elapsed since t according to the fake clock.
func (c *FakeClock) Since(t time.Time) time.Duration {
	return c.Now().Sub(t)
}

// Until returns the duration until t according to the fake clock.
func (c *FakeClock) Until(t time.Time) time.Duration {
	return t.Sub(c.Now())
}

// fireTimers triggers any timers that have reached their deadline.
func (c *FakeClock) fireTimers() {
	for _, t := range c.timers {
		if !t.fired && c.now.After(t.deadline) {
			t.fire()
		}
	}
}

// NewTimer creates a new fake timer.
func (c *FakeClock) NewTimer(d time.Duration) *fakeTimer {
	c.mu.Lock()
	defer c.mu.Unlock()

	t := &fakeTimer{
		clock:    c,
		deadline: c.now.Add(d),
		ch:       make(chan time.Time, 1),
	}
	c.timers = append(c.timers, t)
	return t
}

// After returns a channel that receives the current time after duration d.
func (c *FakeClock) After(d time.Duration) <-chan time.Time {
	return c.NewTimer(d).C()
}

// fakeTimer is a timer controlled by FakeClock.
type fakeTimer struct {
	clock    *FakeClock
	deadline time.Time
	ch       chan time.Time
	fired    bool
	stopped  bool
}

// C returns the timer's channel.
func (t *fakeTimer) C() <-chan time.Time {
	return t.ch
}

// Stop prevents the timer from firing.
func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()

	alreadyFired := t.fired
	t.stopped = true
	return alreadyFired
}

// Reset changes the timer's duration.
func (t *fakeTimer) Reset(d time.Duration) bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()

	wasActive := !t.fired && !t.stopped
	t.deadline = t.clock.now.Add(d)
	t.fired = false
	t.stopped = false
	return wasActive
}

// fire sends the current time on the timer's channel.
func (t *fakeTimer) fire() {
	if !t.stopped && !t.fired {
		t.fired = true
		select {
		case t.ch <- t.clock.now:
		default:
		}
	}
}

// DeterministicSleeper provides controllable sleep behavior for tests.
type DeterministicSleeper struct {
	mu       sync.Mutex
	sleeps   []time.Duration
	index    int
	override map[int]time.Duration // index -> custom duration
}

// NewDeterministicSleeper creates a sleeper that records and can override sleep durations.
func NewDeterministicSleeper() *DeterministicSleeper {
	return &DeterministicSleeper{
		sleeps:   make([]time.Duration, 0),
		override: make(map[int]time.Duration),
	}
}

// Sleep records the duration and advances the index.
func (s *DeterministicSleeper) Sleep(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sleep(d)
}

func (s *DeterministicSleeper) sleep(d time.Duration) {
	if override, ok := s.override[s.index]; ok {
		d = override
	}
	s.sleeps = append(s.sleeps, d)
	s.index++
}

// SetOverride sets a custom duration for a specific sleep index.
func (s *DeterministicSleeper) SetOverride(index int, d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.override[index] = d
}

// Sleeps returns all recorded sleep durations.
func (s *DeterministicSleeper) Sleeps() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]time.Duration, len(s.sleeps))
	copy(result, s.sleeps)
	return result
}

// Count returns the number of sleeps recorded.
func (s *DeterministicSleeper) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.sleeps)
}

// Total returns the sum of all sleep durations.
func (s *DeterministicSleeper) Total() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	var total time.Duration
	for _, d := range s.sleeps {
		total += d
	}
	return total
}

// Reset clears all recorded sleeps.
func (s *DeterministicSleeper) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sleeps = s.sleeps[:0]
	s.index = 0
}

// RetryBackoff provides deterministic exponential backoff for testing.
type RetryBackoff struct {
	Initial  time.Duration
	Max      time.Duration
	Factor   float64
	attempts int
	clock    *FakeClock
}

// NewRetryBackoff creates a new deterministic backoff generator.
func NewRetryBackoff(clock *FakeClock) *RetryBackoff {
	return &RetryBackoff{
		Initial: 10 * time.Millisecond,
		Max:     1 * time.Second,
		Factor:  2.0,
		clock:   clock,
	}
}

// Next returns the next backoff duration and advances the attempt counter.
func (b *RetryBackoff) Next() time.Duration {
	b.attempts++
	duration := b.Initial
	for i := 1; i < b.attempts; i++ {
		duration = time.Duration(float64(duration) * b.Factor)
		if duration > b.Max {
			duration = b.Max
			break
		}
	}
	return duration
}

// Attempts returns the current attempt count.
func (b *RetryBackoff) Attempts() int {
	return b.attempts
}

// Reset resets the attempt counter.
func (b *RetryBackoff) Reset() {
	b.attempts = 0
}
