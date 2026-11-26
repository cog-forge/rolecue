package ai

import (
	"sync"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
)

// Cooldown bounds (KAN-386). A provider that answered 429 is not called again until its Retry-After has passed:
// it is honoured in full, a missing or unreadable one means defaultCooldown, and maxCooldown is only a sanity cap.
const (
	minCooldown     = time.Second
	defaultCooldown = 10 * time.Second
	maxCooldown     = time.Hour
)

// ProviderHealth remembers which provider and model pairs are rate limited and until when. It lives in memory and
// per process; a restart or a second API instance starts with a clean slate.
type ProviderHealth struct {
	mu    sync.Mutex
	now   func() time.Time
	until map[string]time.Time
}

// NewProviderHealth returns an empty store that reads the time from now.
func NewProviderHealth(now func() time.Time) *ProviderHealth {
	return &ProviderHealth{now: now, until: map[string]time.Time{}}
}

// sharedHealth is the process-wide store every Route built by NewRoute uses. Groq limits are per model and one
// key serves every task, so a 429 seen by one task must hold back the others too. Tests swap it per test.
var sharedHealth = NewProviderHealth(time.Now)

// entryKey names what is rate limited: one model of one provider.
func entryKey(c config.LLMConfig) string { return c.Provider + ":" + c.Model }

// RateLimited starts a cooldown for key after a 429. A cooldown is never shortened by a later, smaller one.
func (h *ProviderHealth) RateLimited(key string, retryAfter time.Duration) {
	if retryAfter <= 0 {
		retryAfter = defaultCooldown
	}
	retryAfter = min(max(retryAfter, minCooldown), maxCooldown)
	h.mu.Lock()
	defer h.mu.Unlock()
	if until := h.now().Add(retryAfter); until.After(h.until[key]) {
		h.until[key] = until
	}
}

// Cooling reports how long key must still be left alone; zero means it can be called.
func (h *ProviderHealth) Cooling(key string) time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	until, ok := h.until[key]
	if !ok {
		return 0
	}
	if remaining := until.Sub(h.now()); remaining > 0 {
		return remaining
	}
	delete(h.until, key)
	return 0
}
