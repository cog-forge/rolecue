package service

import (
	"sync"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const (
	// DefaultUserMaxInflight and DefaultUserDailyQuota apply when the configuration leaves the limits unset.
	DefaultUserMaxInflight = 1
	DefaultUserDailyQuota  = 20

	conversionQuotaWindow = 24 * time.Hour
	// gateSweepThreshold bounds the attempt history: past it, an acquire also drops owners whose
	// attempts have all aged out.
	gateSweepThreshold = 1024
)

// conversionGate limits the expensive Blender conversion per user. It reserves a slot for the whole
// request, so a burst of parallel uploads from one user runs at most maxInflight conversions and none
// of them starts without a reservation. State is per API instance and in memory; the daily quota also
// takes the persisted count of avatars made in the window, so a restart does not reset it.
type conversionGate struct {
	mu          sync.Mutex
	maxInflight int
	dailyQuota  int
	now         func() time.Time
	inflight    map[uuid.UUID]int
	attempts    map[uuid.UUID][]time.Time
}

func newConversionGate(maxInflight, dailyQuota int) *conversionGate {
	if maxInflight <= 0 {
		maxInflight = DefaultUserMaxInflight
	}
	return &conversionGate{
		maxInflight: maxInflight, dailyQuota: dailyQuota, now: time.Now,
		inflight: map[uuid.UUID]int{}, attempts: map[uuid.UUID][]time.Time{},
	}
}

// reservation is held from acquire until release. others is how many conversions of the same user were
// already in flight, each of which will claim one free slot.
type reservation struct {
	release func()
	others  int
}

// acquire reserves one conversion for the user. persisted is the number of avatars the user created in
// the quota window, read from the database by the caller. A daily quota of zero or less is unlimited.
func (g *conversionGate) acquire(user uuid.UUID, persisted int64) (reservation, error) {
	if g == nil {
		return reservation{release: func() {}}, nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	if len(g.attempts) > gateSweepThreshold {
		for owner := range g.attempts {
			g.prune(owner, now)
		}
	}
	history := g.prune(user, now)
	if g.dailyQuota > 0 && max(int64(len(history)), persisted) >= int64(g.dailyQuota) {
		return reservation{}, apperror.New(apperror.CodeAvatarConversionQuota, "daily avatar conversion limit reached")
	}
	others := g.inflight[user]
	if others >= g.maxInflight {
		return reservation{}, apperror.New(apperror.CodeAvatarConversionQuota, "an avatar conversion is already running for your account")
	}
	g.inflight[user] = others + 1
	g.attempts[user] = append(history, now)
	var once sync.Once
	return reservation{others: others, release: func() {
		once.Do(func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			if g.inflight[user] <= 1 {
				delete(g.inflight, user)
			} else {
				g.inflight[user]--
			}
		})
	}}, nil
}

// prune drops the user's attempts older than the window and returns what is left.
func (g *conversionGate) prune(user uuid.UUID, now time.Time) []time.Time {
	history := g.attempts[user]
	keep := history[:0]
	for _, at := range history {
		if now.Sub(at) < conversionQuotaWindow {
			keep = append(keep, at)
		}
	}
	if len(keep) == 0 {
		delete(g.attempts, user)
		return nil
	}
	g.attempts[user] = keep
	return keep
}
