//go:build integration

package tests

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var startable = []domain.GenerationStatus{domain.StatusPending, domain.StatusFailed}

func longAgo() time.Time { return time.Now().Add(-time.Hour) }

func TestConcurrentClaimsStartExactlyOneGeneration(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	roadmap := e.roadmap(t, owner, e.profile(t, owner).ID)

	var won, refused atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.roadmaps.Claim(e.ctx, roadmap.ID, startable, longAgo())
			switch {
			case err == nil:
				won.Add(1)
			case errors.Is(err, repository.ErrNotClaimable):
				refused.Add(1)
			default:
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if won.Load() != 1 || refused.Load() != 7 {
		t.Fatalf("claims won=%d refused=%d, want 1 and 7", won.Load(), refused.Load())
	}
	if got := e.roadmap(t, owner, roadmap.RoleProfileID).GenerationStatus; got != domain.StatusGenerating {
		t.Fatalf("status = %s, want generating", got)
	}
}

func TestStaleWorkerCannotOverwriteANewerGeneration(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	profile := e.profile(t, owner)
	roadmap := e.roadmap(t, owner, profile.ID)

	stale, err := e.roadmaps.Claim(e.ctx, roadmap.ID, startable, longAgo())
	if err != nil {
		t.Fatal(err)
	}
	// The first worker went silent for longer than the lease: a second one takes over.
	fresh, err := e.roadmaps.Claim(e.ctx, roadmap.ID, startable, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal("takeover of a silent generation", err)
	}
	if !fresh.UpdatedAt.After(stale.UpdatedAt) {
		t.Fatalf("fence did not move: stale=%s fresh=%s", stale.UpdatedAt, fresh.UpdatedAt)
	}
	if _, err = e.roadmaps.Publish(e.ctx, roadmap.ID, stale, []domain.NewItem{{Title: "Old", Position: 0}}, nil); !errors.Is(err, repository.ErrFenceLost) {
		t.Fatalf("stale publish: %v, want ErrFenceLost", err)
	}
	if err = e.roadmaps.Fail(e.ctx, roadmap.ID, stale); !errors.Is(err, repository.ErrFenceLost) {
		t.Fatalf("stale fail: %v, want ErrFenceLost", err)
	}
	if items, _ := e.roadmaps.ListItems(e.ctx, owner, profile.ID); len(items) != 0 {
		t.Fatalf("a stale worker stored %d topics", len(items))
	}
	if _, err = e.roadmaps.Publish(e.ctx, roadmap.ID, fresh, []domain.NewItem{{Title: "New", Position: 0}}, nil); err != nil {
		t.Fatal("current worker publish", err)
	}
	if got := e.roadmap(t, owner, profile.ID); got.GenerationStatus != domain.StatusReady {
		t.Fatalf("status = %s, want ready", got.GenerationStatus)
	}
}

func TestClaimIsRefusedFromGeneratingAndReady(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	roadmap := e.roadmap(t, owner, e.profile(t, owner).ID)
	e.publish(t, roadmap.ID, "SQL")
	if _, err := e.roadmaps.Claim(e.ctx, roadmap.ID, startable, longAgo()); !errors.Is(err, repository.ErrNotClaimable) {
		t.Fatalf("claim of a ready roadmap: %v, want ErrNotClaimable", err)
	}
}

func TestFailedGenerationCanBeClaimedAgain(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	roadmap := e.roadmap(t, owner, e.profile(t, owner).ID)
	fence, err := e.roadmaps.Claim(e.ctx, roadmap.ID, startable, longAgo())
	if err != nil {
		t.Fatal(err)
	}
	if err = e.roadmaps.Fail(e.ctx, roadmap.ID, fence); err != nil {
		t.Fatal(err)
	}
	if got := e.roadmap(t, owner, roadmap.RoleProfileID).GenerationStatus; got != domain.StatusFailed {
		t.Fatalf("status = %s, want failed", got)
	}
	if _, err = e.roadmaps.Claim(e.ctx, roadmap.ID, startable, longAgo()); err != nil {
		t.Fatal("retry of a failed generation", err)
	}
}

func TestPublishKeepsTheOldTopicsWhenTheHookFails(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	profile := e.profile(t, owner)
	roadmap := e.roadmap(t, owner, profile.ID)
	old := e.publish(t, roadmap.ID, "SQL", "Go")
	oldQuestion := e.question(t, profile.JDID, old[0].ID)

	fence, err := e.roadmaps.Claim(e.ctx, roadmap.ID, []domain.GenerationStatus{domain.StatusReady}, longAgo())
	if err != nil {
		t.Fatal("claim of a ready roadmap for regeneration", err)
	}
	boom := errors.New("bank insert failed")
	_, err = e.roadmaps.Publish(e.ctx, roadmap.ID, fence, []domain.NewItem{{Title: "Kafka", Position: 0}},
		func(_ context.Context, _ pgx.Tx, _ []domain.Item) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("publish error = %v, want the hook error", err)
	}
	items, _ := e.roadmaps.ListItems(e.ctx, owner, profile.ID)
	if len(items) != 2 || items[0].ID != old[0].ID || items[1].ID != old[1].ID {
		t.Fatalf("old topics were not kept intact: %+v", items)
	}
	var mapped uuid.UUID
	if err = e.pool.QueryRow(e.ctx, `SELECT roadmap_item_id FROM core_questions WHERE id=$1`, oldQuestion).Scan(&mapped); err != nil || mapped != old[0].ID {
		t.Fatalf("old question mapping changed: %v (err=%v)", mapped, err)
	}
	// The aborted generation is still in flight: the worker marks it failed and the old set stays active.
	if err = e.roadmaps.Fail(e.ctx, roadmap.ID, fence); err != nil {
		t.Fatal(err)
	}
}

func TestPublishReplacementDetachesQuestionsOfReplacedTopics(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	profile := e.profile(t, owner)
	roadmap := e.roadmap(t, owner, profile.ID)
	old := e.publish(t, roadmap.ID, "SQL")
	question := e.question(t, profile.JDID, old[0].ID)

	fence, err := e.roadmaps.Claim(e.ctx, roadmap.ID, []domain.GenerationStatus{domain.StatusReady}, longAgo())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.roadmaps.Publish(e.ctx, roadmap.ID, fence, []domain.NewItem{{Title: "Kafka", Position: 0}}, nil); err != nil {
		t.Fatal(err)
	}
	if n := e.count(t, `SELECT count(*) FROM core_questions WHERE id=$1 AND roadmap_item_id IS NULL`, question); n != 1 {
		t.Fatal("a question still points at a replaced topic")
	}
}
