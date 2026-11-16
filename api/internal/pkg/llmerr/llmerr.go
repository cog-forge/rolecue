// Package llmerr classifies the failure of a routed model call and retries a call that failed for a transient
// reason. The classification is carried by the error itself (a Reason method), so a feature service can decide
// without importing the AI adapter and without ever seeing a provider message or response body.
package llmerr

import (
	"context"
	"errors"
	"time"
)

// Failure reasons. They are the same strings the model route logs.
const (
	ReasonTimeout       = "timeout"
	ReasonRateLimited   = "rate_limited"
	ReasonHTTP5xx       = "http_5xx"
	ReasonNetwork       = "network"
	ReasonInvalidOutput = "invalid_output"
)

// Failure is a classified model failure. It carries a reason and nothing else.
type Failure struct{ Kind string }

// NewFailure returns a failure with the given reason.
func NewFailure(reason string) *Failure { return &Failure{Kind: reason} }

func (f *Failure) Error() string  { return "llm call failed: " + f.Kind }
func (f *Failure) Reason() string { return f.Kind }

type reasoned interface{ Reason() string }

// Reason returns the failure reason carried by err, or "" when err is not a classified failure.
func Reason(err error) string {
	var r reasoned
	if errors.As(err, &r) {
		return r.Reason()
	}
	return ""
}

// Transient reports whether err is a provider-side failure that a repeated call can overcome: a timeout, a
// 429, a 5xx or a network error. A cancelled or expired caller context is never transient here; the caller
// decides that from its own context.
func Transient(err error) bool {
	switch Reason(err) {
	case ReasonTimeout, ReasonRateLimited, ReasonHTTP5xx, ReasonNetwork:
		return true
	}
	return false
}

// TransientOrInvalid is Transient plus a reply that could not be decoded or validated (truncated JSON, an
// unexpected EOF). It is the retry rule of generation, where a fresh sample usually succeeds.
func TransientOrInvalid(err error) bool {
	return Transient(err) || Reason(err) == ReasonInvalidOutput
}

// Policy bounds the retries of one call.
type Policy struct {
	// MaxRetries is the number of extra attempts after the first.
	MaxRetries int
	// Backoff is the wait before retry n (1-based). Nil means no wait.
	Backoff func(retry int) time.Duration
	// MinRemaining is the time that must be left in the context after the wait for another attempt to be
	// worth starting. Without it a retry could only be cut off by the deadline.
	MinRemaining time.Duration
	// Retryable decides which errors are retried. Nil means Transient.
	Retryable func(error) bool
	// Sleep waits for d or until ctx ends, and reports whether the wait completed. Nil uses a timer; tests
	// replace it.
	Sleep func(ctx context.Context, d time.Duration) bool
	// OnRetry is called before each wait with the 1-based retry number and the error that caused it.
	OnRetry func(retry int, err error)
}

// Exponential returns the backoff base, 2*base, 4*base, ... capped at max.
func Exponential(base, max time.Duration) func(int) time.Duration {
	return func(retry int) time.Duration {
		d := base << (retry - 1)
		if d <= 0 || d > max {
			return max
		}
		return d
	}
}

// Do calls fn and retries it while the error is retryable, the retries are not used up and the context has
// room for the wait and another attempt. The last error is returned unchanged. A context that ended stops
// the loop at once and its attempt's error is returned.
func Do[T any](ctx context.Context, p Policy, fn func(context.Context) (T, error)) (T, error) {
	retryable := p.Retryable
	if retryable == nil {
		retryable = Transient
	}
	sleep := p.Sleep
	if sleep == nil {
		sleep = sleepTimer
	}
	for retry := 1; ; retry++ {
		result, err := fn(ctx)
		if err == nil || ctx.Err() != nil || retry > p.MaxRetries || !retryable(err) {
			return result, err
		}
		var wait time.Duration
		if p.Backoff != nil {
			wait = p.Backoff(retry)
		}
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < wait+p.MinRemaining {
			return result, err
		}
		if p.OnRetry != nil {
			p.OnRetry(retry, err)
		}
		if wait > 0 && !sleep(ctx, wait) {
			return result, err
		}
	}
}

func sleepTimer(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
