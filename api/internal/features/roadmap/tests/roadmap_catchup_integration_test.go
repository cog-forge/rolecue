//go:build integration

// Roadmap catch-up (KAN-351, RM-10) against a real migrated PostgreSQL database: the job's exact SQL filter, the
// bounded batches, restart and concurrent workers, out-of-order processing and the "delayed" read state. The
// seed helpers come from roadmap_read_integration_test.go.
package tests

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
	"github.com/google/uuid"
)

func rdCatchUp(t *testing.T, f rdFixture, batch int) *service.CatchUp {
	t.Helper()
	c, err := service.NewCatchUp(f.e.roadmaps, rdLinker(f), service.CatchUpSettings{BatchSize: batch, BackoffBase: time.Minute, BackoffMax: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// missing returns the sessions of this fixture's profile that the job would pick up.
func (f rdFixture) missing(t *testing.T) []uuid.UUID {
	t.Helper()
	all, err := f.e.roadmaps.ListSessionsMissingAssessments(f.e.ctx, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	mine := map[uuid.UUID]bool{}
	rows, err := f.e.pool.Query(f.e.ctx, `SELECT id FROM interviews WHERE role_profile_id = $1`, f.profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		mine[id] = true
	}
	var out []uuid.UUID
	for _, id := range all {
		if mine[id] {
			out = append(out, id)
		}
	}
	return out
}

// The disposable database may hold sessions of other tests; the job lists them too, so batch limits are asserted
// on this fixture's sessions through missing(), and runs use a batch large enough to reach them.

func TestCatchUpLinksABacklogInBoundedBatchesOldestFirst(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 5)
	var sessions []uuid.UUID
	for i := 0; i < 5; i++ {
		sessions = append(sessions, rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(i), false, rdScored(q[i], float64(60+i*10))))
	}
	if got := f.missing(t); len(got) != 5 {
		t.Fatalf("missing = %d sessions, want 5", len(got))
	}
	// Oldest first: the head of the listing is the oldest session of this fixture.
	listed, err := f.e.roadmaps.ListSessionsMissingAssessments(f.e.ctx, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	var mineInOrder []uuid.UUID
	for _, id := range listed {
		for _, s := range sessions {
			if id == s {
				mineInOrder = append(mineInOrder, id)
			}
		}
	}
	for i := range sessions {
		if mineInOrder[i] != sessions[i] {
			t.Fatalf("listing order = %v, want oldest evaluation first %v", mineInOrder, sessions)
		}
	}
	// A bounded batch examines at most batch-size sessions.
	limited, err := f.e.roadmaps.ListSessionsMissingAssessments(f.e.ctx, 2, nil)
	if err != nil || len(limited) != 2 {
		t.Fatalf("limited listing = %d, err %v, want 2", len(limited), err)
	}

	// Run with a big enough batch to reach this fixture even when other tests left rows (none stay after cleanup).
	if _, err := rdCatchUp(t, f, 1000).Run(f.e.ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := f.missing(t); len(got) != 0 {
		t.Fatalf("still missing after the run: %v", got)
	}
	v := f.view(t)
	if v.UpdateState != domain.UpdateUpToDate || v.Items[0].EvidenceQuestionCount != 5 || *v.Items[0].Score != 80 || v.Items[0].LearningStatus != domain.TopicAchieved {
		t.Fatalf("view = %+v", v)
	}
	// Nothing left to do, nothing written twice.
	n, err := rdCatchUp(t, f, 1000).Run(f.e.ctx, time.Now())
	if err != nil || n != 0 {
		t.Fatalf("second run linked %d sessions, err %v", n, err)
	}
	if got := f.e.count(t, `SELECT count(*) FROM roadmap_assessments a JOIN interviews i ON i.id = a.interview_id WHERE i.role_profile_id = $1`, f.profile.ID); got != 5 {
		t.Fatalf("%d assessments, want 5", got)
	}
}

func TestCatchUpRestartAndConcurrentWorkersNeverDuplicate(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 8)
	for i := 0; i < 4; i++ {
		rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(i), false, rdScored(q[2*i], 70), rdSkipped(q[2*i+1]))
	}
	// Two "API instances" (separate CatchUp values, no shared memory) running at the same time, then a "restarted"
	// third one: the UNIQUE key makes the writes idempotent even without the runner's advisory lock.
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := rdCatchUp(t, f, 1000).Run(context.Background(), time.Now())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := rdCatchUp(t, f, 1000).Run(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := f.e.count(t, `SELECT count(*) FROM roadmap_assessments a JOIN interviews i ON i.id = a.interview_id WHERE i.role_profile_id = $1`, f.profile.ID); got != 8 {
		t.Fatalf("%d assessments, want exactly 8 (one per counted core)", got)
	}
	if got := f.e.count(t, `SELECT count(*) FROM (SELECT interview_id, question_position FROM roadmap_assessments GROUP BY 1,2 HAVING count(*) > 1) d`); got != 0 {
		t.Fatalf("%d duplicate links", got)
	}
}

func TestCatchUpProcessesSessionsOutOfOrderWithoutChangingTheCurrentScore(t *testing.T) {
	// The same question answered three times at different event times. Whatever order the sessions are linked in,
	// the current result is the one of the latest session: 90. A catch-up that ran last for the oldest session
	// (late old session) must not bring 20 back.
	orders := [][]int{{0, 1, 2}, {2, 1, 0}, {1, 0, 2}, {2, 0, 1}}
	for _, order := range orders {
		f := rdNew(t, "SQL")
		q := f.questions(t, f.items[0], 1)
		scores := []float64{20, 55, 90}
		sessions := []uuid.UUID{
			rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), false, rdScored(q[0], scores[0])),
			rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(3), false, rdScored(q[0], scores[1])),
			rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(9), false, rdScored(q[0], scores[2])),
		}
		for _, i := range order {
			if _, err := rdLinker(f).LinkPracticeResults(f.e.ctx, sessions[i]); err != nil {
				t.Fatal(err)
			}
			if got := f.e.count(t, `SELECT count(*) FROM roadmap_assessments a JOIN interviews i ON i.id = a.interview_id WHERE i.role_profile_id = $1`, f.profile.ID); got == 0 {
				t.Fatal("nothing linked")
			}
		}
		it := f.view(t).Items[0]
		if it.Score == nil || *it.Score != 90 || it.EvidenceQuestionCount != 1 || !it.LastEvaluatedAt.Equal(*rdAt(9)) {
			t.Fatalf("link order %v: score = %v over %d (last %v), want 90 over 1 evaluated at hour 9", order, it.Score, it.EvidenceQuestionCount, it.LastEvaluatedAt)
		}
	}
}

// TestCatchUpFilterIsExactAndAgreesWithTheReadModel guards the SQL pre-filter against the Go rules: the sessions
// the job lists are exactly those with a countable, mapped, unlinked core, and once the job has run, the read model
// has nothing pending. A looser filter would make the same sessions starve the batch on every run.
func TestCatchUpFilterIsExactAndAgreesWithTheReadModel(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 12)
	unmapped := f.e.question(t, f.profile.JDID, uuid.Nil)

	listed := map[string]uuid.UUID{
		"scored":  rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), false, rdScored(q[0], 70)),
		"skipped": rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(1), false, rdSkipped(q[1])),
	}
	notListed := map[string]uuid.UUID{
		"never asked":        rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(2), false, rdAnswer{question: q[2]}),
		"answered ungraded":  rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(3), false, rdAnswer{question: q[3], delivered: true, answered: true}),
		"evaluation failed":  rdSession(t, f.e, f.profile.ID, "practice", "failed", rdAt(4), false, rdScored(q[4], 90)),
		"evaluation pending": rdSession(t, f.e, f.profile.ID, "practice", "pending", rdAt(5), false, rdScored(q[5], 90)),
		"unfinished":         rdSession(t, f.e, f.profile.ID, "practice", "completed", nil, false, rdScored(q[6], 90)),
		"core without topic": rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(7), false, rdScored(unmapped, 90)),
		"already linked":     rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(8), true, rdScored(q[8], 90)),
	}
	flagged := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(9), false, rdScored(q[9], 90))
	f.e.exec(t, `UPDATE interviews SET injection_flagged = true WHERE id = $1`, flagged)
	notListed["injection flagged"] = flagged

	got := map[uuid.UUID]bool{}
	for _, id := range f.missing(t) {
		got[id] = true
	}
	for name, id := range listed {
		if !got[id] {
			t.Errorf("%q must be listed", name)
		}
	}
	for name, id := range notListed {
		if got[id] {
			t.Errorf("%q must not be listed (it would be re-examined on every run forever)", name)
		}
	}
	// The read model counts exactly the same two sessions as pending.
	if v := f.view(t); v.PendingResultCount != len(listed) || v.UpdateState != domain.UpdateUpdating {
		t.Fatalf("read model pending = %d (%s), want %d", v.PendingResultCount, v.UpdateState, len(listed))
	}
	if _, err := rdCatchUp(t, f, 1000).Run(f.e.ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if left := f.missing(t); len(left) != 0 {
		t.Fatalf("still listed after the run: %v", left)
	}
	if v := f.view(t); v.PendingResultCount != 0 || v.UpdateState != domain.UpdateUpToDate {
		t.Fatalf("read model after the run = %d pending (%s)", v.PendingResultCount, v.UpdateState)
	}
	// The set of sessions with links is the same as the set the job listed plus the one that was linked before.
	var withLinks []string
	rows, err := f.e.pool.Query(f.e.ctx, `SELECT DISTINCT a.interview_id::text FROM roadmap_assessments a JOIN interviews i ON i.id = a.interview_id WHERE i.role_profile_id = $1`, f.profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		withLinks = append(withLinks, id)
	}
	sort.Strings(withLinks)
	if len(withLinks) != 3 {
		t.Fatalf("sessions with links = %v, want the two listed sessions and the one linked before", withLinks)
	}
}

func TestReadStateBecomesDelayedWhenNothingLinksAResult(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 1)
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), false, rdScored(q[0], 90))

	for name, tc := range map[string]struct {
		now  time.Time
		want domain.UpdateState
	}{
		"just evaluated":      {rdAt(0).Add(time.Minute), domain.UpdateUpdating},
		"job off for an hour": {*rdAt(1), domain.UpdateDelayed},
	} {
		svc := service.NewReadService(f.e.roadmaps).WithClock(func() time.Time { return tc.now })
		v, err := svc.Get(f.e.ctx, f.owner, "candidate", f.profile.ID)
		if err != nil || v.UpdateState != tc.want || v.PendingResultCount != 1 || v.Items[0].Score != nil {
			t.Fatalf("%s: view = %+v, err %v, want %s with the result neither scored nor shown as 0", name, v, err, tc.want)
		}
	}
	// Once the job runs, the delay is gone.
	if _, err := rdCatchUp(t, f, 1000).Run(f.e.ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	svc := service.NewReadService(f.e.roadmaps).WithClock(func() time.Time { return *rdAt(5) })
	if v, err := svc.Get(f.e.ctx, f.owner, "candidate", f.profile.ID); err != nil || v.UpdateState != domain.UpdateUpToDate {
		t.Fatalf("view = %+v, err %v", v, err)
	}
}

// A session with a mapped core and a core whose bank question has no topic is healthy: the unmapped core is simply
// not part of any Roadmap. It must be linked once, reported as success and never backed off.
func TestCatchUpTreatsACoreWithoutATopicAsHealthy(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 1)
	unmapped := f.e.question(t, f.profile.JDID, uuid.Nil)
	iv := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), false, rdScored(q[0], 80), rdScored(unmapped, 90))
	c := rdCatchUp(t, f, 1000)
	now := time.Now()

	n, err := c.Run(f.e.ctx, now)
	if err != nil || n < 1 {
		t.Fatalf("run linked %d sessions, err %v: a session with an unmapped core is healthy", n, err)
	}
	if f.assessments(t, iv) != 1 {
		t.Fatalf("%d assessments, want 1 (only the mapped core)", f.assessments(t, iv))
	}
	if got := f.missing(t); len(got) != 0 {
		t.Fatalf("still listed: %v", got)
	}
	// Not backed off: the next run, one second later, has nothing to do and reports no failure.
	if n, err := c.Run(f.e.ctx, now.Add(time.Second)); err != nil || n != 0 {
		t.Fatalf("second run = %d, %v", n, err)
	}
}
