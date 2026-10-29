package email

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

const (
	defaultQueueSize = 100
	// maxBackoff caps the exponential wait so a large MaxAttempts cannot park a worker for hours.
	maxBackoff = 30 * time.Second
)

// errPanic marks a Sender that panicked. The panic value is not kept: it can contain the message body.
var errPanic = errors.New("sender panicked")

// DispatcherOptions configures a Dispatcher. Zero values pick safe defaults: one worker, a 100-message queue,
// a single attempt and no backoff.
type DispatcherOptions struct {
	Workers     int
	QueueSize   int
	MaxAttempts int
	BaseBackoff time.Duration
	Logger      *logger.Logger
}

// job is one queued message. Label is the caller's safe name for it, the only identity that is ever logged.
type job struct {
	label string
	msg   Message
}

// Dispatcher sends mail on background workers so a request handler never waits for a provider. Callers
// cannot observe send errors; the outcome is only visible in the log.
type Dispatcher struct {
	sender      Sender
	maxAttempts int
	baseBackoff time.Duration
	log         *logger.Logger

	queue chan job
	// mu guards closed against sends on queue: Dispatch holds the read lock while it enqueues, and Close
	// takes the write lock before it closes the channel, so a Dispatch never sends on a closed channel.
	mu       sync.RWMutex
	closed   bool
	stopOnce sync.Once

	// ctx is cancelled when Close runs out of time, which aborts in-flight sends and backoff waits.
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewDispatcher starts the workers and returns a Dispatcher. Call Close to stop it.
func NewDispatcher(s Sender, o DispatcherOptions) *Dispatcher {
	workers := max(o.Workers, 1)
	queueSize := o.QueueSize
	if queueSize <= 0 {
		queueSize = defaultQueueSize
	}
	log := o.Logger
	if log == nil {
		log = &logger.Logger{Logger: zap.NewNop()}
	}

	ctx, cancel := context.WithCancel(context.Background())
	d := &Dispatcher{
		sender:      s,
		maxAttempts: max(o.MaxAttempts, 1),
		baseBackoff: max(o.BaseBackoff, 0),
		log:         log,
		queue:       make(chan job, queueSize),
		ctx:         ctx,
		cancel:      cancel,
		done:        make(chan struct{}),
	}

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.work()
		}()
	}
	go func() {
		wg.Wait()
		d.cancel()
		close(d.done)
	}()
	return d
}

// Dispatch queues msg for delivery. It never blocks: when the queue is full the message is dropped and a
// warning is logged, because a stuck provider must not stall the request that triggered the email. After
// Close it does nothing.
func (d *Dispatcher) Dispatch(label string, msg Message) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return
	}
	if msg.ID == "" {
		msg.ID = newMessageID()
	}
	select {
	case d.queue <- job{label: label, msg: msg}:
	default:
		d.log.Warn("email queue full, dropped", logger.String("label", label))
	}
}

// Close stops accepting mail and waits for the queued mail to go out. If ctx ends first, the remaining mail
// is dropped, in-flight sends are cancelled, and ctx.Err() is returned. Close is safe to call twice.
func (d *Dispatcher) Close(ctx context.Context) error {
	d.stopOnce.Do(d.stop)
	// An expired context means no time to drain, so the queue is abandoned even if the workers are idle.
	if err := ctx.Err(); err != nil {
		d.abandon()
		return err
	}
	select {
	case <-d.done:
		return nil
	case <-ctx.Done():
		d.abandon()
		return ctx.Err()
	}
}

// abandon cancels in-flight sends and says how much queued mail will never go out, so a shutdown that ran
// out of time is visible in the logs with a number.
func (d *Dispatcher) abandon() {
	d.cancel()
	if n := len(d.queue); n > 0 {
		d.log.Warn("email shutdown abandoned queued mail", logger.Int("count", n))
	}
}

// newMessageID is a random 128-bit hex string; it only has to be unique, not secret.
func newMessageID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

func (d *Dispatcher) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	close(d.queue)
}

func (d *Dispatcher) work() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case j, ok := <-d.queue:
			if !ok {
				return
			}
			d.deliver(j)
		}
	}
}

// deliver makes up to maxAttempts tries. A Permanent error or an exhausted budget drops the message.
func (d *Dispatcher) deliver(j job) {
	for attempt := 1; ; attempt++ {
		err := d.send(j.msg)
		if err == nil {
			d.log.Info("email sent", logger.String("label", j.label), logger.Int("attempt", attempt))
			return
		}
		if d.ctx.Err() != nil {
			d.log.Warn("email dropped", logger.String("label", j.label), logger.Int("attempt", attempt),
				logger.String("outcome", "aborted"), logger.String("kind", kind(err)))
			return
		}
		if IsPermanent(err) || attempt >= d.maxAttempts {
			d.log.Warn("email dropped", logger.String("label", j.label), logger.Int("attempt", attempt),
				logger.String("outcome", "dropped"), logger.String("kind", kind(err)))
			return
		}
		d.log.Warn("email attempt failed, retrying", logger.String("label", j.label),
			logger.Int("attempt", attempt), logger.String("outcome", "retrying"), logger.String("kind", kind(err)))
		if !d.wait(d.backoff(attempt)) {
			d.log.Warn("email dropped", logger.String("label", j.label), logger.Int("attempt", attempt),
				logger.String("outcome", "aborted"), logger.String("kind", kind(err)))
			return
		}
	}
}

// send makes one attempt. A panic in the Sender is recovered and turned into a Permanent error, so one bad
// message cannot kill the worker and is not retried.
func (d *Dispatcher) send(msg Message) (err error) {
	defer func() {
		if recover() != nil {
			err = Permanent(errPanic)
		}
	}()
	return d.sender.Send(d.ctx, msg)
}

// backoff is the wait before retry number attempt: BaseBackoff doubled per retry, capped at maxBackoff.
func (d *Dispatcher) backoff(attempt int) time.Duration {
	delay := d.baseBackoff
	for i := 1; i < attempt && delay < maxBackoff; i++ {
		delay *= 2
	}
	return min(delay, maxBackoff)
}

// wait sleeps for delay and reports false if the dispatcher was aborted first.
func (d *Dispatcher) wait(delay time.Duration) bool {
	if delay <= 0 {
		return d.ctx.Err() == nil
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-d.ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// kind names an error class for the log. It is deliberately coarse: the error text can contain the
// recipient or the provider's response body, so it is never logged.
func kind(err error) string {
	var timeout interface{ Timeout() bool }
	switch {
	case errors.Is(err, errPanic):
		return "panic"
	case IsPermanent(err):
		return "permanent"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &timeout) && timeout.Timeout():
		return "timeout"
	default:
		return "provider"
	}
}
