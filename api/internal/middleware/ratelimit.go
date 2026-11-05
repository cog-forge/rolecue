package middleware

import (
	"errors"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// limiterIdleTTL is how long a user's bucket is kept after their last request. A bucket idle that long has
// refilled anyway, so dropping it changes nothing for the user and bounds the memory of the map.
const limiterIdleTTL = 10 * time.Minute

// UserRateLimiter is an in-memory token bucket per authenticated user. It protects endpoints that cost
// money (an LLM call per request) from a signed-in user hammering them. The buckets live in one process,
// so with several API instances the effective limit is per instance.
type UserRateLimiter struct {
	limit rate.Limit
	burst int
	now   func() time.Time

	mu        sync.Mutex
	buckets   map[uuid.UUID]*userBucket
	lastSweep time.Time
}

type userBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewUserRateLimiter allows burst requests at once per user and then one more every refill.
func NewUserRateLimiter(refill time.Duration, burst int) *UserRateLimiter {
	return &UserRateLimiter{
		limit:   rate.Every(refill),
		burst:   burst,
		now:     time.Now,
		buckets: make(map[uuid.UUID]*userBucket),
	}
}

// Allow takes one token of the user's bucket and reports whether the request may go on. When it may not,
// the second value is how long until a token is available.
func (l *UserRateLimiter) Allow(userID uuid.UUID) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)
	bucket, ok := l.buckets[userID]
	if !ok {
		bucket = &userBucket{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.buckets[userID] = bucket
	}
	bucket.lastSeen = now
	reservation := bucket.limiter.ReserveN(now, 1)
	if !reservation.OK() {
		return false, time.Duration(math.MaxInt64)
	}
	if delay := reservation.DelayFrom(now); delay > 0 {
		reservation.CancelAt(now)
		return false, delay
	}
	return true, 0
}

// sweep drops the buckets idle for limiterIdleTTL. It runs at most once per TTL, under the lock.
func (l *UserRateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < limiterIdleTTL {
		return
	}
	l.lastSweep = now
	for id, bucket := range l.buckets {
		if now.Sub(bucket.lastSeen) >= limiterIdleTTL {
			delete(l.buckets, id)
		}
	}
}

// Middleware must run after RequireAuth. A request without an authenticated user is rejected the same
// way RequireRole rejects it, so the limiter never keys on an unauthenticated value. Over the limit it
// answers 429 RATE_LIMITED with a Retry-After header.
func (l *UserRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := CurrentUserID(c)
		if !ok {
			response.Error(c, apperror.Wrap(apperror.CodeInvalidToken, "authentication required", errors.New("no authenticated user in context")))
			c.Abort()
			return
		}
		allowed, retryAfter := l.Allow(userID)
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			response.Error(c, apperror.New(apperror.CodeRateLimited, "too many requests, try again shortly"))
			c.Abort()
			return
		}
		c.Next()
	}
}
