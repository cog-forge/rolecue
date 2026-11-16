package service

import (
	"crypto/sha256"
	"encoding/json"
	"sync"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmerr"
	"github.com/google/uuid"
)

const (
	// gradeRetries is how many times one question's grading call is repeated after a transient provider
	// failure (every provider of the route timed out, answered 429 or 5xx, or the network failed). The repeats
	// belong to the call, not to the session: they never use one of the interview's evaluation attempts.
	gradeRetries = 2
	// gradeMinRemaining is the time a repeat needs left in the evaluation's own bound; the route splits its
	// 30 s budget between its providers, so less than this cannot complete a call.
	gradeMinRemaining = 15 * time.Second
	// gradeCacheTTL and gradeCacheMax bound the in-process memory of validated grades. It exists so that a
	// session whose evaluation failed on one question keeps the questions it did grade when the evaluation
	// is claimed again; it is a shortcut, never a source of truth.
	gradeCacheTTL = 30 * time.Minute
	gradeCacheMax = 512
)

// defaultGradeRetry is the per-call retry policy of grading (KAN-383): only transient provider failures, with
// a short capped backoff. An invalid grade is not repeated here: the route already retries or falls back on it.
func defaultGradeRetry() llmerr.Policy {
	return llmerr.Policy{
		MaxRetries:   gradeRetries,
		Backoff:      llmerr.Exponential(time.Second, 4*time.Second),
		MinRemaining: gradeMinRemaining,
		Retryable:    llmerr.Transient,
	}
}

type gradeKey struct {
	interview uuid.UUID
	position  int32
}

type gradeEntry struct {
	digest  [sha256.Size]byte
	grade   domain.QuestionGrade
	expires time.Time
}

// gradeCache keeps the validated grades of sessions whose evaluation has not completed yet. An entry is used
// only for the same input (a digest of the question, its rubric criteria and its answers), so a grade is never
// applied to a different answer. Nothing in it is logged.
type gradeCache struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[gradeKey]gradeEntry
}

func newGradeCache() *gradeCache {
	return &gradeCache{now: time.Now, entries: map[gradeKey]gradeEntry{}}
}

func digestOf(in domain.QuestionGradeInput) ([sha256.Size]byte, bool) {
	raw, err := json.Marshal(in)
	if err != nil {
		return [sha256.Size]byte{}, false
	}
	return sha256.Sum256(raw), true
}

func (c *gradeCache) get(interview uuid.UUID, in domain.QuestionGradeInput) (domain.QuestionGrade, bool) {
	digest, ok := digestOf(in)
	if !ok {
		return domain.QuestionGrade{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, found := c.entries[gradeKey{interview, in.QuestionPosition}]
	if !found || entry.digest != digest || !entry.expires.After(c.now()) {
		return domain.QuestionGrade{}, false
	}
	return entry.grade, true
}

func (c *gradeCache) put(interview uuid.UUID, in domain.QuestionGradeInput, grade domain.QuestionGrade) {
	digest, ok := digestOf(in)
	if !ok {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if len(c.entries) >= gradeCacheMax {
		for key, entry := range c.entries {
			if !entry.expires.After(now) {
				delete(c.entries, key)
			}
		}
		for key := range c.entries {
			if len(c.entries) < gradeCacheMax {
				break
			}
			delete(c.entries, key)
		}
	}
	c.entries[gradeKey{interview, in.QuestionPosition}] = gradeEntry{digest: digest, grade: grade, expires: now.Add(gradeCacheTTL)}
}

// forget drops everything kept for a session, once its evaluation is saved.
func (c *gradeCache) forget(interview uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		if key.interview == interview {
			delete(c.entries, key)
		}
	}
}
