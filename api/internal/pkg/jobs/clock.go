package jobs

import (
	"sort"
	"sync"
	"time"
)

// Clock is the time source of the runner: the time a run is given and the ticker that schedules runs.
type Clock interface {
	Now() time.Time
	NewTicker(d time.Duration) Ticker
}

// Ticker delivers ticks like time.Ticker.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// SystemClock is the wall clock.
func SystemClock() Clock { return systemClock{} }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) NewTicker(d time.Duration) Ticker { return systemTicker{time.NewTicker(d)} }

type systemTicker struct{ t *time.Ticker }

func (s systemTicker) C() <-chan time.Time { return s.t.C }
func (s systemTicker) Stop()               { s.t.Stop() }

// FakeClock is a manual clock for tests: time moves only on Advance, which fires every ticker that is
// due, so a test drives scheduled runs without sleeping. Like time.Ticker, a tick that finds the
// previous one unread is dropped.
type FakeClock struct {
	mu      sync.Mutex
	now     time.Time
	tickers []*fakeTicker
	created chan struct{}
}

// NewFakeClock starts a fake clock at start.
func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start, created: make(chan struct{}, 64)}
}

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) NewTicker(d time.Duration) Ticker {
	if d <= 0 {
		panic("jobs: non-positive ticker interval")
	}
	c.mu.Lock()
	t := &fakeTicker{clock: c, every: d, next: c.now.Add(d), ch: make(chan time.Time, 1)}
	c.tickers = append(c.tickers, t)
	c.mu.Unlock()
	select {
	case c.created <- struct{}{}:
	default:
	}
	return t
}

// WaitTickers blocks until n tickers were created, so a test knows the runner's loops are scheduled
// before it advances the clock.
func (c *FakeClock) WaitTickers(n int) {
	for range n {
		<-c.created
	}
}

// Advance moves the clock by d and fires every ticker that came due, in time order.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	target := c.now.Add(d)
	for {
		due := c.dueTickers(target)
		if len(due) == 0 {
			break
		}
		t := due[0]
		c.now = t.next
		t.next = t.next.Add(t.every)
		select {
		case t.ch <- c.now:
		default:
		}
	}
	c.now = target
	c.mu.Unlock()
}

// dueTickers lists the running tickers due at or before target, earliest first. The caller holds mu.
func (c *FakeClock) dueTickers(target time.Time) []*fakeTicker {
	var due []*fakeTicker
	for _, t := range c.tickers {
		if !t.stopped && !t.next.After(target) {
			due = append(due, t)
		}
	}
	sort.Slice(due, func(i, j int) bool { return due[i].next.Before(due[j].next) })
	return due
}

type fakeTicker struct {
	clock   *FakeClock
	every   time.Duration
	next    time.Time
	ch      chan time.Time
	stopped bool
}

func (t *fakeTicker) C() <-chan time.Time { return t.ch }

func (t *fakeTicker) Stop() {
	t.clock.mu.Lock()
	t.stopped = true
	t.clock.mu.Unlock()
}
