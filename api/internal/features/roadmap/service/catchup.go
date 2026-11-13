package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
)

// MaxCatchUpBatch bounds one run's batch, so a run never scans or locks without limit.
const MaxCatchUpBatch = 1000

// maxTrackedFailures bounds the in-memory back-off table. Past it the table is cleared, which only means a
// failing session is tried again sooner than its back-off.
const maxTrackedFailures = 10000

// MissingLister finds practice sessions whose evaluated results still lack Roadmap links.
type MissingLister interface {
	// ListSessionsMissingAssessments returns at most limit interview ids, oldest evaluation first (event time:
	// interviews.completed_at, then id), skipping the excluded ids. Only sessions that may feed a Roadmap and
	// have at least one countable, mapped core question without a link are returned.
	ListSessionsMissingAssessments(ctx context.Context, limit int32, exclude []uuid.UUID) ([]uuid.UUID, error)
}

// CatchUpSettings bounds the catch-up job.
type CatchUpSettings struct {
	// BatchSize is the number of sessions one run examines.
	BatchSize int
	// BackoffBase is the wait after a session's first failure; it doubles on every further failure up to
	// BackoffMax.
	BackoffBase time.Duration
	BackoffMax  time.Duration
}

// CatchUp links the results of practice sessions the evaluation hook did not link (KAN-351, RM-10): a link that
// failed, an API restart between the saved result and the link, or sessions evaluated before the Roadmap
// existed. It runs as a System Handler job, reads its work from the database on every run and keeps no queue.
//
// Recency never depends on the run: assessments carry no event time, and the Roadmap orders results by the
// session's completion time, so linking an old session after a newer one cannot change the current score. The
// job never calls an AI model. It is safe on any number of instances: the runner holds a cluster-wide lock per
// run, and the write itself is idempotent.
type CatchUp struct {
	lister   MissingLister
	linker   Linker
	settings CatchUpSettings

	mu       sync.Mutex
	failures map[uuid.UUID]failure
}

type failure struct {
	count  int
	nextAt time.Time
}

func NewCatchUp(lister MissingLister, linker Linker, settings CatchUpSettings) (*CatchUp, error) {
	switch {
	case lister == nil || linker == nil:
		return nil, errors.New("roadmap catch-up needs a lister and a linker")
	case settings.BatchSize < 1 || settings.BatchSize > MaxCatchUpBatch:
		return nil, fmt.Errorf("roadmap catch-up batch size must be between 1 and %d", MaxCatchUpBatch)
	case settings.BackoffBase <= 0 || settings.BackoffMax < settings.BackoffBase:
		return nil, errors.New("roadmap catch-up back-off must be positive and its maximum at least its base")
	}
	return &CatchUp{lister: lister, linker: linker, settings: settings, failures: map[uuid.UUID]failure{}}, nil
}

// Run links one batch of sessions and returns how many sessions got at least one new link. A session that fails,
// or that counts but cannot be linked, is backed off (it is not examined again until its delay passes) so it
// never blocks the sessions behind it, and the run returns an error that says how many failed. The error and
// the logs carry interview ids and error classes only, never answers or database text.
func (c *CatchUp) Run(ctx context.Context, now time.Time) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	sessions, err := c.lister.ListSessionsMissingAssessments(ctx, int32(c.settings.BatchSize), c.excluded(now))
	if err != nil {
		return 0, fmt.Errorf("list sessions missing roadmap links: %w", err)
	}
	linked, failed := 0, 0
	for _, id := range sessions {
		if ctx.Err() != nil {
			failed++
			break
		}
		result, err := c.linker.LinkPracticeResults(ctx, id)
		switch {
		case err != nil:
			failed++
			c.fail(id, now)
			logger.FromContext(ctx).Warn("roadmap catch-up could not link a session",
				logger.Op("roadmap.catchup"), logger.String("interview_id", id.String()), logger.Cause(err))
		case result.Unlinked > 0:
			// Results count, yet the store left some without a link: the data does not satisfy the link rules.
			// Back the session off like a failure instead of listing it first on every run. A position another
			// worker linked meanwhile is not unlinked, so concurrent runs never trip this.
			failed++
			c.fail(id, now)
			logger.FromContext(ctx).Warn("roadmap catch-up found a session that counts but cannot be linked",
				logger.Op("roadmap.catchup"), logger.String("interview_id", id.String()), logger.Int("unlinked", result.Unlinked))
		default:
			c.clear(id)
			if result.Linked > 0 {
				linked++
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return linked, err
	}
	if failed > 0 {
		return linked, fmt.Errorf("%d of %d sessions could not be linked to the roadmap", failed, len(sessions))
	}
	return linked, nil
}

func (c *CatchUp) excluded(now time.Time) []uuid.UUID {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]uuid.UUID, 0, len(c.failures))
	for id, f := range c.failures {
		if now.Before(f.nextAt) {
			out = append(out, id)
		}
	}
	return out
}

func (c *CatchUp) fail(id uuid.UUID, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.failures) >= maxTrackedFailures {
		c.failures = map[uuid.UUID]failure{}
	}
	f := c.failures[id]
	f.count++
	delay := c.settings.BackoffBase
	for i := 1; i < f.count && delay < c.settings.BackoffMax; i++ {
		delay *= 2
	}
	if delay > c.settings.BackoffMax {
		delay = c.settings.BackoffMax
	}
	f.nextAt = now.Add(delay)
	c.failures[id] = f
}

func (c *CatchUp) clear(id uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.failures, id)
}
