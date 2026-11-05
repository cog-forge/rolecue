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

// limiterIdleTTL is how long a bucket is kept after its last request. A bucket idle that long has
// refilled anyway, so dropping it changes nothing for the client and bounds the memory of the map.
const limiterIdleTTL = 10 * time.Minute

// bucketSet is an in-memory token bucket per key. The buckets live in one process, so with several API
// instances the effective limit is per instance. now is injectable so tests can move time.
type bucketSet[K comparable] struct {
	limit rate.Limit
	burst int
	now   func() time.Time

	mu        sync.Mutex
	buckets   map[K]*userBucket
	lastSweep time.Time
}

type userBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newBucketSet[K comparable](refill time.Duration, burst int) *bucketSet[K] {
	return &bucketSet[K]{
		limit:   rate.Every(refill),
		burst:   burst,
		now:     time.Now,
		buckets: make(map[K]*userBucket),
	}
}

// SetClock replaces the clock the buckets refill by. It is for tests; call it before serving requests.
func (l *bucketSet[K]) SetClock(now func() time.Time) { l.now = now }

// Allow takes one token of the key's bucket and reports whether the request may go on. When it may not,
// the second value is how long until a token is available.
func (l *bucketSet[K]) Allow(key K) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)
	bucket, ok := l.buckets[key]
	if !ok {
		bucket = &userBucket{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.buckets[key] = bucket
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
func (l *bucketSet[K]) sweep(now time.Time) {
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

// refuse answers 429 RATE_LIMITED with a Retry-After header of at least one second.
func refuse(c *gin.Context, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	c.Header("Retry-After", strconv.Itoa(seconds))
	response.Error(c, apperror.New(apperror.CodeRateLimited, "too many requests, try again shortly"))
	c.Abort()
}

// PerMinute turns a requests-per-minute setting into the refill interval of a bucket (the burst is the
// number of requests allowed at once). Callers skip the limiter for a setting below one.
func PerMinute(requests int) time.Duration {
	if requests < 1 {
		requests = 1
	}
	return time.Minute / time.Duration(requests)
}

// UserRateLimiter is a token bucket per authenticated user. It protects endpoints that cost money (an
// LLM call per request) from a signed-in user hammering them.
type UserRateLimiter struct {
	*bucketSet[uuid.UUID]
}

// NewUserRateLimiter allows burst requests at once per user and then one more every refill.
func NewUserRateLimiter(refill time.Duration, burst int) *UserRateLimiter {
	return &UserRateLimiter{newBucketSet[uuid.UUID](refill, burst)}
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
			refuse(c, retryAfter)
			return
		}
		c.Next()
	}
}

// IPRateLimiter is a token bucket per client address. c.ClientIP() only believes X-Forwarded-For from
// the configured trusted proxies, so a client cannot pick its own bucket by sending that header.
type IPRateLimiter struct {
	*bucketSet[string]
}

// NewIPRateLimiter allows burst requests at once per address and then one more every refill.
func NewIPRateLimiter(refill time.Duration, burst int) *IPRateLimiter {
	return &IPRateLimiter{newBucketSet[string](refill, burst)}
}

// Middleware answers 429 RATE_LIMITED with a Retry-After header once the client address is over its
// limit. It needs no login, so it can sit before or after RequireAuth.
func (l *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, retryAfter := l.Allow(c.ClientIP())
		if !allowed {
			refuse(c, retryAfter)
			return
		}
		c.Next()
	}
}
