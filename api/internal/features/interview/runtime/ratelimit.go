package runtime

import (
	"errors"
	"time"
)

// ErrSessionInvalid is what Participant.Verify returns when the login behind the socket no longer holds
// (logged out, banned, password reset, expired). Any other error from Verify is treated as a transient
// failure of the check and does not close the socket.
var ErrSessionInvalid = errors.New("the login behind this socket is no longer valid")

// Defaults of the per-socket guards (Settings zero values).
const (
	defaultRevalidateInterval = 60 * time.Second
	// A recording streams a few chunks a second, so 20 messages/s with a burst of 60 never touches a
	// real client, while a flood is cut off within seconds.
	defaultMessageRate  = 20.0
	defaultMessageBurst = 60
	// maxThrottled is how many messages in a row may be refused before the socket is closed.
	maxThrottled = 100
	// throttleNoticeEvery limits the RATE_LIMITED notices sent to a flooding client.
	throttleNoticeEvery = 25
)

// tokenBucket is a plain token bucket. It is used by one goroutine only.
type tokenBucket struct {
	rate   float64 // tokens added per second
	burst  float64
	tokens float64
	last   time.Time
	now    func() time.Time
}

func newTokenBucket(rate float64, burst int, now func() time.Time) *tokenBucket {
	if now == nil {
		now = time.Now
	}
	return &tokenBucket{rate: rate, burst: float64(burst), tokens: float64(burst), last: now(), now: now}
}

// allow takes one token and reports whether one was available.
func (b *tokenBucket) allow() bool {
	t := b.now()
	if elapsed := t.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens = min(b.burst, b.tokens+elapsed*b.rate)
	}
	b.last = t
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
