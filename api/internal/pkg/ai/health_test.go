package ai

import (
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)} }

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// useSharedHealth swaps the process-wide store for one test, so a cooldown recorded here cannot reach another test.
func useSharedHealth(t *testing.T, h *ProviderHealth) {
	t.Helper()
	old := sharedHealth
	sharedHealth = h
	t.Cleanup(func() { sharedHealth = old })
}

func TestProviderHealth(t *testing.T) {
	for _, tc := range []struct {
		name       string
		retryAfter time.Duration
		want       time.Duration
	}{
		{"honoured in full", 30 * time.Second, 30 * time.Second},
		{"minutes are kept", 20 * time.Minute, 20 * time.Minute},
		{"absent means the default", 0, 10 * time.Second},
		{"negative means the default", -5 * time.Second, 10 * time.Second},
		{"tiny is raised to the minimum", time.Millisecond, time.Second},
		{"huge is capped at an hour", 3 * time.Hour, time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			h := NewProviderHealth(clock.now)
			h.RateLimited("groq:m", tc.retryAfter)
			if got := h.Cooling("groq:m"); got != tc.want {
				t.Fatalf("cooling = %s, want %s", got, tc.want)
			}
			clock.advance(tc.want - time.Millisecond)
			if h.Cooling("groq:m") <= 0 {
				t.Fatal("cooldown ended early")
			}
			clock.advance(time.Millisecond)
			if got := h.Cooling("groq:m"); got != 0 {
				t.Fatalf("cooling = %s after expiry, want 0", got)
			}
			if got := h.Cooling("other:m"); got != 0 {
				t.Fatalf("another key is cooling: %s", got)
			}
		})
	}
}

func TestProviderHealthNeverShortensACooldown(t *testing.T) {
	clock := newFakeClock()
	h := NewProviderHealth(clock.now)
	h.RateLimited("k", time.Minute)
	h.RateLimited("k", 5*time.Second)
	if got := h.Cooling("k"); got != time.Minute {
		t.Fatalf("cooling = %s, want 1m0s", got)
	}
}

func TestProviderHealthConcurrent(t *testing.T) {
	h := NewProviderHealth(time.Now)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				h.RateLimited("k", time.Second)
				_ = h.Cooling("k")
				_ = h.Cooling("other")
			}
		}()
	}
	wg.Wait()
}

func TestEntryKey(t *testing.T) {
	if got := entryKey(config.LLMConfig{Provider: "groq", Model: "openai/gpt-oss-120b"}); got != "groq:openai/gpt-oss-120b" {
		t.Fatalf("key = %q", got)
	}
}
