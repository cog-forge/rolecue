package email_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/email"
)

// fakeSender records every successful send. send, when set, runs before recording and receives the 1-based
// call number; returning an error or panicking simulates a provider failure.
type fakeSender struct {
	mu    sync.Mutex
	calls int
	sent  []email.Message
	send  func(ctx context.Context, call int, msg email.Message) error
}

func (f *fakeSender) Send(ctx context.Context, msg email.Message) error {
	f.mu.Lock()
	f.calls++
	call := f.calls
	f.mu.Unlock()
	if f.send != nil {
		if err := f.send(ctx, call, msg); err != nil {
			return err
		}
	}
	f.mu.Lock()
	f.sent = append(f.sent, msg)
	f.mu.Unlock()
	return nil
}

func (f *fakeSender) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeSender) Sent() []email.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]email.Message(nil), f.sent...)
}

func msg(i int) email.Message {
	return email.Message{To: fmt.Sprintf("user%d@example.com", i), Subject: "s", Body: "b"}
}

// closeNow drains the dispatcher with a generous budget and fails the test if the drain did not finish.
func closeNow(t *testing.T, d *email.Dispatcher) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDispatchDeliversOnSuccess(t *testing.T) {
	s := &fakeSender{}
	d := email.NewDispatcher(s, email.DispatcherOptions{})
	d.Dispatch("welcome", msg(1))
	closeNow(t, d)

	if got := s.Sent(); len(got) != 1 || got[0] != msg(1) {
		t.Fatalf("sent = %+v", got)
	}
}

func TestTransientFailureRetriedThenDropped(t *testing.T) {
	s := &fakeSender{send: func(context.Context, int, email.Message) error { return errors.New("provider down") }}
	d := email.NewDispatcher(s, email.DispatcherOptions{MaxAttempts: 3})
	d.Dispatch("welcome", msg(1))
	closeNow(t, d)

	if s.Calls() != 3 {
		t.Fatalf("attempts = %d, want 3", s.Calls())
	}
	if len(s.Sent()) != 0 {
		t.Fatal("a message that never succeeded was recorded as sent")
	}
}

func TestTransientFailureThenSuccess(t *testing.T) {
	s := &fakeSender{send: func(_ context.Context, call int, _ email.Message) error {
		if call == 1 {
			return errors.New("timeout")
		}
		return nil
	}}
	d := email.NewDispatcher(s, email.DispatcherOptions{MaxAttempts: 3})
	d.Dispatch("welcome", msg(1))
	closeNow(t, d)

	if s.Calls() != 2 || len(s.Sent()) != 1 {
		t.Fatalf("calls = %d, sent = %d", s.Calls(), len(s.Sent()))
	}
}

func TestPermanentErrorNotRetried(t *testing.T) {
	s := &fakeSender{send: func(context.Context, int, email.Message) error {
		return email.Permanent(errors.New("address rejected"))
	}}
	d := email.NewDispatcher(s, email.DispatcherOptions{MaxAttempts: 5})
	d.Dispatch("welcome", msg(1))
	closeNow(t, d)

	if s.Calls() != 1 {
		t.Fatalf("attempts = %d, want exactly 1", s.Calls())
	}
}

func TestDispatchNeverBlocksOnHangingSender(t *testing.T) {
	s := &fakeSender{send: func(ctx context.Context, _ int, _ email.Message) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	d := email.NewDispatcher(s, email.DispatcherOptions{Workers: 1, QueueSize: 1})

	returned := make(chan struct{})
	go func() {
		for i := range 10 {
			d.Dispatch("welcome", msg(i))
		}
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("Dispatch blocked while the sender hung and the queue was full")
	}

	// Close cannot drain a hung sender; it must give up when its context expires.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := d.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close err = %v, want DeadlineExceeded", err)
	}
}

func TestQueueFullDropsWithoutBlocking(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	s := &fakeSender{send: func(context.Context, int, email.Message) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return nil
	}}
	d := email.NewDispatcher(s, email.DispatcherOptions{Workers: 1, QueueSize: 1})

	d.Dispatch("first", msg(1))
	<-started // the worker now holds message 1, so the queue is empty and has room for exactly one more
	d.Dispatch("second", msg(2))

	returned := make(chan struct{})
	go func() {
		d.Dispatch("third", msg(3))
		d.Dispatch("fourth", msg(4))
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("Dispatch blocked on a full queue")
	}

	close(release)
	closeNow(t, d)
	if got := len(s.Sent()); got != 2 {
		t.Fatalf("sent = %d, want 2 (the dropped messages must not be delivered)", got)
	}
}

func TestCloseDrainsQueuedMail(t *testing.T) {
	s := &fakeSender{}
	d := email.NewDispatcher(s, email.DispatcherOptions{Workers: 2, QueueSize: 50})
	for i := range 20 {
		d.Dispatch("bulk", msg(i))
	}
	closeNow(t, d)

	if got := len(s.Sent()); got != 20 {
		t.Fatalf("sent = %d, want 20", got)
	}
}

func TestCloseWithExpiredContextReturnsContextError(t *testing.T) {
	d := email.NewDispatcher(&fakeSender{}, email.DispatcherOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := d.Close(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Close err = %v, want context.Canceled", err)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	d := email.NewDispatcher(&fakeSender{}, email.DispatcherOptions{})
	closeNow(t, d)
	closeNow(t, d)
}

func TestDispatchAfterCloseIsNoop(t *testing.T) {
	s := &fakeSender{}
	d := email.NewDispatcher(s, email.DispatcherOptions{})
	closeNow(t, d)

	d.Dispatch("late", msg(1))
	if s.Calls() != 0 {
		t.Fatalf("a Dispatch after Close reached the sender %d times", s.Calls())
	}
}

func TestPanicInSendIsRecovered(t *testing.T) {
	s := &fakeSender{send: func(_ context.Context, call int, _ email.Message) error {
		if call == 1 {
			panic("provider exploded")
		}
		return nil
	}}
	d := email.NewDispatcher(s, email.DispatcherOptions{Workers: 1, MaxAttempts: 3})
	d.Dispatch("bad", msg(1))
	d.Dispatch("good", msg(2))
	closeNow(t, d)

	// The panicking message is treated as permanent: one attempt, no retry, and the worker survives for the next.
	if s.Calls() != 2 {
		t.Fatalf("calls = %d, want 2", s.Calls())
	}
	if got := s.Sent(); len(got) != 1 || got[0] != msg(2) {
		t.Fatalf("sent = %+v, want only the second message", got)
	}
}

func TestConcurrentDispatchIsRaceFree(t *testing.T) {
	const goroutines, perGoroutine = 50, 20
	s := &fakeSender{}
	d := email.NewDispatcher(s, email.DispatcherOptions{Workers: 4, QueueSize: goroutines * perGoroutine})

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perGoroutine {
				d.Dispatch("concurrent", msg(g*perGoroutine+i))
			}
		}()
	}
	wg.Wait()
	closeNow(t, d)

	if got := len(s.Sent()); got != goroutines*perGoroutine {
		t.Fatalf("sent = %d, want %d", got, goroutines*perGoroutine)
	}
}

func TestBackoffStopsWhenCloseRunsOut(t *testing.T) {
	started := make(chan struct{}, 1)
	s := &fakeSender{send: func(context.Context, int, email.Message) error {
		select {
		case started <- struct{}{}:
		default:
		}
		return errors.New("provider down")
	}}
	// An hour-long backoff: the only way the test finishes quickly is if the wait honours the abort.
	d := email.NewDispatcher(s, email.DispatcherOptions{MaxAttempts: 3, BaseBackoff: time.Hour})
	d.Dispatch("welcome", msg(1))
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := d.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close err = %v, want DeadlineExceeded", err)
	}
	if s.Calls() != 1 {
		t.Fatalf("calls = %d, want 1 (no retry after abort)", s.Calls())
	}
}
