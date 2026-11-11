//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"testing"

	guidelinesrepository "github.com/cog-forge/rolecue/api/internal/features/admin/guidelines/repository"
	guidelinesservice "github.com/cog-forge/rolecue/api/internal/features/admin/guidelines/service"
	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// guidelineTestLock is the advisory lock every guideline integration test holds, shared with the admin
// guideline tests: the table is global, so these tests run one at a time.
const guidelineTestLock = 4276001

// admin saves guidelines through the real Admin service and removes them afterwards.
type guidelineAdmin struct {
	t    *testing.T
	ctx  context.Context
	id   uuid.UUID
	svc  *guidelinesservice.Service
	repo *guidelinesrepository.Repository
}

// newGuidelineAdmin must run before newSeed: its cleanup deletes the guideline rows, which interviews
// reference, so it has to run after the seed cleanup has deleted the interviews (cleanups run last in,
// first out).
func newGuidelineAdmin(t *testing.T, ctx context.Context, pool *pgxpool.Pool, followUpCeiling int32) *guidelineAdmin {
	t.Helper()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, int64(guidelineTestLock)); err != nil {
		t.Fatal(err)
	}
	repo := guidelinesrepository.NewRepository(pool)
	svc, err := guidelinesservice.New(repo, followUpCeiling)
	if err != nil {
		t.Fatal(err)
	}
	a := &guidelineAdmin{t: t, ctx: ctx, id: uuid.New(), svc: svc, repo: repo}
	if _, err := pool.Exec(ctx, `INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, 'Guideline Admin', $2, 'admin', true)`,
		a.id, "guideline-admin-"+a.id.String()+"@example.test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `UPDATE public.interviews SET behavior_guideline_id = NULL WHERE behavior_guideline_id IN (SELECT id FROM public.interview_behavior_guidelines WHERE updated_by = $1)`, a.id)
		if _, err := pool.Exec(bg, `DELETE FROM public.interview_behavior_guidelines WHERE updated_by = $1`, a.id); err != nil {
			t.Errorf("cleanup guidelines: %v", err)
		}
		_, _ = pool.Exec(bg, `DELETE FROM public.users WHERE id = $1`, a.id)
		_, _ = conn.Exec(bg, `SELECT pg_advisory_unlock($1)`, int64(guidelineTestLock))
		conn.Release()
	})
	return a
}

// save appends a version holding values, as an Admin would.
func (a *guidelineAdmin) save(values domain.BehaviorGuideline) {
	a.t.Helper()
	current, found, err := a.repo.Current(a.ctx)
	if err != nil {
		a.t.Fatal(err)
	}
	expected := int32(0)
	if found {
		expected = current.Version
	}
	if _, err := a.svc.Save(a.ctx, a.id, "admin", expected, values); err != nil {
		a.t.Fatal(err)
	}
}

func intp(n int32) *int32 { return &n }

func tuned(change func(*domain.BehaviorGuideline)) domain.BehaviorGuideline {
	g := domain.DefaultBehaviorGuideline()
	change(&g)
	return g
}

// recordedGuideline reads the guideline version a session started under, or "" when it recorded none.
func (s *seed) recordedGuideline(interviewID uuid.UUID) (tone string, recorded bool) {
	s.t.Helper()
	var value *string
	if err := s.pool.QueryRow(s.ctx, `SELECT g.tone FROM public.interviews i
		LEFT JOIN public.interview_behavior_guidelines g ON g.id = i.behavior_guideline_id WHERE i.id = $1`, interviewID).Scan(&value); err != nil {
		s.t.Fatal(err)
	}
	if value == nil {
		return "", false
	}
	return *value, true
}

// TestSessionRunsUnderTheGuidelineItStartedWith proves the snapshot: a session started under one version
// keeps it, for every follow-up and after a reconnect, while a save made afterwards applies only to the
// sessions that start later.
func TestSessionRunsUnderTheGuidelineItStartedWith(t *testing.T) {
	ctx, pool := openPool(t)
	admin := newGuidelineAdmin(t, ctx, pool, 2)
	s := newSeed(t, ctx, pool)
	svc, err := service.New(repository.NewRepository(pool), service.Settings{QuestionLimit: 3, FollowUpLimit: 2})
	if err != nil {
		t.Fatal(err)
	}
	decider := &recordingDecider{}
	svc.SetFollowUpDecider(decider)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)

	admin.save(tuned(func(g *domain.BehaviorGuideline) { g.Tone, g.QuestionLength = domain.ToneFormal, domain.LengthConcise }))
	first, _ := practiceSession(t, ctx, s, svc, candidate, 3)
	if tone, ok := s.recordedGuideline(first.ID); !ok || tone != domain.ToneFormal {
		t.Fatalf("session recorded tone %q (recorded %v), want formal", tone, ok)
	}

	if _, err := svc.AppendTurn(ctx, candidate, "candidate", first.ID, 1, "first answer"); err != nil {
		t.Fatal(err)
	}
	if d, err := svc.FollowUp(ctx, candidate, "candidate", first.ID); err != nil || d.Question == nil {
		t.Fatalf("FollowUp = %+v, %v", d.Question, err)
	}

	// An Admin saves a new version while the session is running.
	admin.save(tuned(func(g *domain.BehaviorGuideline) {
		g.Tone, g.FollowUpMax, g.AvoidTopics = domain.ToneChallenging, intp(0), []string{domain.TopicCompensation}
	}))

	// The running session still decides under the first version, including after a "reconnect" (a second
	// decision for the same slot reads the session's own snapshot, not the latest version).
	for range 2 {
		d, err := svc.FollowUp(ctx, candidate, "candidate", first.ID)
		if err != nil || d.Question == nil {
			t.Fatalf("running session FollowUp = %+v, %v", d.Question, err)
		}
	}
	for i, seen := range decider.seen() {
		if seen.Guideline.Tone != domain.ToneFormal || seen.Guideline.QuestionLength != domain.LengthConcise || seen.Guideline.FollowUpMax != nil || len(seen.Guideline.AvoidTopics) != 0 {
			t.Fatalf("decision %d saw %+v, want the first version", i+1, seen.Guideline)
		}
	}

	// A session started after the save runs under the new version: its follow-ups are off (max 0), so the
	// decider is not called at all and nothing is charged for a decision.
	calls := len(decider.seen())
	second, _ := practiceSession(t, ctx, s, svc, candidate, 3)
	if tone, ok := s.recordedGuideline(second.ID); !ok || tone != domain.ToneChallenging {
		t.Fatalf("new session recorded tone %q (recorded %v), want challenging", tone, ok)
	}
	if _, err := svc.AppendTurn(ctx, candidate, "candidate", second.ID, 1, "first answer"); err != nil {
		t.Fatal(err)
	}
	if d, err := svc.FollowUp(ctx, candidate, "candidate", second.ID); err != nil || d.Question != nil {
		t.Fatalf("follow_up_max 0 must ask nothing, got %+v, %v", d.Question, err)
	}
	if len(decider.seen()) != calls {
		t.Fatal("follow_up_max 0 must not even call the decider")
	}
}

// TestFollowUpMaxLowersTheConfiguredLimitAndNeverRaisesIt proves the turn rules: the effective limit is the
// smaller of interview.follow_up_limit and the session's follow_up_max, for the decision and for storing the
// answer alike.
func TestFollowUpMaxLowersTheConfiguredLimitAndNeverRaisesIt(t *testing.T) {
	cases := []struct {
		name       string
		configured int32
		max        *int32
		want       int // follow-ups asked after one core answer
	}{
		{"no guideline value uses the configured limit", 2, nil, 2},
		{"a lower max wins", 3, intp(1), 1},
		{"a higher max never exceeds the configured limit", 1, intp(3), 1},
		{"zero turns follow-ups off", 3, intp(0), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, pool := openPool(t)
			admin := newGuidelineAdmin(t, ctx, pool, tc.configured)
			s := newSeed(t, ctx, pool)
			svc, err := service.New(repository.NewRepository(pool), service.Settings{QuestionLimit: 3, FollowUpLimit: tc.configured})
			if err != nil {
				t.Fatal(err)
			}
			svc.SetFollowUpDecider(&recordingDecider{})
			candidate := s.user("candidate")
			s.wallet(candidate, 20)
			admin.save(tuned(func(g *domain.BehaviorGuideline) { g.FollowUpMax = tc.max }))
			started, _ := practiceSession(t, ctx, s, svc, candidate, 3)
			if _, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, "answer"); err != nil {
				t.Fatal(err)
			}
			asked := 0
			for {
				d, err := svc.FollowUp(ctx, candidate, "candidate", started.ID)
				if err != nil {
					t.Fatal(err)
				}
				if d.Question == nil {
					break
				}
				asked++
				if _, err := svc.AppendFollowUp(ctx, candidate, "candidate", started.ID, *d.Question, "more detail"); err != nil {
					t.Fatalf("storing follow-up %d: %v", asked, err)
				}
				if asked > 5 {
					t.Fatal("follow-ups never stop")
				}
			}
			if asked != tc.want {
				t.Fatalf("asked %d follow-ups, want %d", asked, tc.want)
			}
		})
	}
}

// TestSessionWithoutARecordedGuidelineRunsUnderTheDefaults is the fallback: an interview that started before
// guidelines existed (or before any was saved) has no recorded version and behaves exactly as today, whatever
// is saved later.
func TestSessionWithoutARecordedGuidelineRunsUnderTheDefaults(t *testing.T) {
	ctx, pool := openPool(t)
	admin := newGuidelineAdmin(t, ctx, pool, 2)
	s := newSeed(t, ctx, pool)
	svc, err := service.New(repository.NewRepository(pool), service.Settings{QuestionLimit: 3, FollowUpLimit: 2})
	if err != nil {
		t.Fatal(err)
	}
	decider := &recordingDecider{}
	svc.SetFollowUpDecider(decider)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 3)
	s.exec(`UPDATE public.interviews SET behavior_guideline_id = NULL WHERE id = $1`, started.ID)
	admin.save(tuned(func(g *domain.BehaviorGuideline) {
		g.Tone, g.FollowUpMax, g.QuestionLength = domain.ToneChallenging, intp(0), domain.LengthConcise
	}))

	if _, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, "answer"); err != nil {
		t.Fatal(err)
	}
	d, err := svc.FollowUp(ctx, candidate, "candidate", started.ID)
	if err != nil || d.Question == nil {
		t.Fatalf("FollowUp = %+v, %v; the configured limit of 2 applies", d.Question, err)
	}
	seen := decider.seen()
	if len(seen) != 1 || !seen[0].Guideline.IsDefault() {
		t.Fatalf("decider saw %+v, want the defaults", seen)
	}
}

// TestSavingGuidelinesLeavesCompletedInterviewsUntouched proves a completed interview keeps its rows, its
// result and the guideline version it ran under when an Admin saves a new version.
func TestSavingGuidelinesLeavesCompletedInterviewsUntouched(t *testing.T) {
	ctx, pool := openPool(t)
	admin := newGuidelineAdmin(t, ctx, pool, 1)
	s := newSeed(t, ctx, pool)
	svc, err := service.New(repository.NewRepository(pool), service.Settings{QuestionLimit: 2, FollowUpLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetFollowUpDecider(&recordingDecider{})
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	admin.save(tuned(func(g *domain.BehaviorGuideline) { g.Tone = domain.ToneFriendly }))
	started, _ := practiceSession(t, ctx, s, svc, candidate, 2)
	for position := int32(1); position <= 2; position++ {
		if _, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, position, "answer"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Complete(ctx, candidate, "candidate", started.ID); err != nil {
		t.Fatal(err)
	}

	snapshot := func() string {
		var interview, turns json.RawMessage
		if err := pool.QueryRow(ctx, `SELECT to_jsonb(i) FROM public.interviews i WHERE i.id = $1`, started.ID).Scan(&interview); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.position), '[]') FROM public.conversation_turns t WHERE t.interview_id = $1`, started.ID).Scan(&turns); err != nil {
			t.Fatal(err)
		}
		return string(interview) + string(turns)
	}
	before := snapshot()
	admin.save(tuned(func(g *domain.BehaviorGuideline) {
		g.Tone, g.FollowUpFocus = domain.ToneChallenging, domain.FocusThorough
	}))
	if after := snapshot(); after != before {
		t.Fatalf("a completed interview changed after a guideline save:\nbefore %s\nafter  %s", before, after)
	}
	if tone, ok := s.recordedGuideline(started.ID); !ok || tone != domain.ToneFriendly {
		t.Fatalf("the interview records tone %q (recorded %v), want the friendly version it ran under", tone, ok)
	}
}
