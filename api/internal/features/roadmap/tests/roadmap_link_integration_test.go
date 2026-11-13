//go:build integration

// Assessment writer (KAN-350, RM-09) against a real migrated PostgreSQL database. It uses the real session
// source (interview record + evaluation adapter query) and the real store, with sessions seeded through SQL.
package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	evalrepo "github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	interviewrepo "github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
	"github.com/cog-forge/rolecue/api/internal/provider"
	"github.com/google/uuid"
)

func rdLinker(f rdFixture) *service.AssessmentService {
	return provider.ProvideRoadmapAssessmentService(provider.ProvideRoadmapSessionSource(evalrepo.NewRepository(f.e.pool)), f.e.roadmaps)
}

func (f rdFixture) assessments(t *testing.T, interview uuid.UUID) int {
	t.Helper()
	return f.e.count(t, `SELECT count(*) FROM roadmap_assessments WHERE interview_id=$1`, interview)
}

func TestLinkWritesCountedCoresAndARetryAddsNothing(t *testing.T) {
	f := rdNew(t, "SQL", "Go")
	sql := f.questions(t, f.items[0], 3)
	goQ := f.questions(t, f.items[1], 1)
	unmapped := f.e.question(t, f.profile.JDID, uuid.Nil)
	// scored (SQL), scored (Go), skipped (SQL, the last shown question); the unmapped bank question is a core without a topic.
	iv := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), false,
		rdScored(sql[0], 80), rdScored(goQ[0], 60), rdScored(unmapped, 90), rdSkipped(sql[1]))
	link := rdLinker(f)

	for attempt, wantLinked := range []int{3, 0, 0} {
		res, err := link.LinkPracticeResults(f.e.ctx, iv)
		if err != nil || !res.Eligible || res.Counted != 4 || res.Linked != wantLinked || res.Unlinked != 0 {
			t.Fatalf("attempt %d: result = %+v, err = %v, want 4 counted, %d linked and none refused (the core without a topic is not a failure)", attempt+1, res, err, wantLinked)
		}
	}
	if n := f.assessments(t, iv); n != 3 {
		t.Fatalf("assessments = %d, want 3 (two scored cores and the skipped one; the core without a topic is not linked)", n)
	}
	var topics int
	if err := f.e.pool.QueryRow(f.e.ctx, `SELECT count(DISTINCT roadmap_item_id) FROM roadmap_assessments WHERE interview_id=$1`, iv).Scan(&topics); err != nil || topics != 2 {
		t.Fatalf("topics = %d, err = %v, want the two topics of the questions", topics, err)
	}
	// The Roadmap now counts the skipped core as 0: SQL = (80 + 0) / 2, Go = 60.
	v := f.view(t)
	if v.UpdateState != domain.UpdateUpToDate || v.Items[0].Score == nil || *v.Items[0].Score != 40 || v.Items[1].Score == nil || *v.Items[1].Score != 60 {
		t.Fatalf("view = %+v", v)
	}
}

func TestLinkNeverLinksWhatIsNotEvidence(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 6)
	link := rdLinker(f)

	pending := rdSession(t, f.e, f.profile.ID, "practice", "pending", rdAt(0), false, rdScored(q[0], 90))
	failed := rdSession(t, f.e, f.profile.ID, "practice", "failed", rdAt(1), false, rdScored(q[1], 90))
	open := rdSession(t, f.e, f.profile.ID, "practice", "completed", nil, false, rdScored(q[2], 90))
	flagged := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(3), false, rdScored(q[3], 90))
	f.e.exec(t, `UPDATE interviews SET injection_flagged = true WHERE id = $1`, flagged)
	unasked := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(4), false, rdAnswer{question: q[4]})
	ungraded := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(5), false, rdAnswer{question: q[5], delivered: true, answered: true})

	for name, iv := range map[string]uuid.UUID{"evaluation pending": pending, "evaluation failed": failed, "unfinished": open,
		"injection flagged": flagged, "never asked": unasked, "answered but ungraded": ungraded} {
		if _, err := link.LinkPracticeResults(f.e.ctx, iv); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n := f.assessments(t, iv); n != 0 {
			t.Errorf("%s: %d assessments written, want none", name, n)
		}
	}
	// The store refuses even when the caller does not check: unfinished sessions and strangers.
	for name, call := range map[string]func() (service.LinkOutcome, error){
		"unfinished session": func() (service.LinkOutcome, error) { return f.e.roadmaps.LinkCores(f.e.ctx, f.owner, open, []int32{1}) },
		"another candidate": func() (service.LinkOutcome, error) {
			return f.e.roadmaps.LinkCores(f.e.ctx, f.e.user(t, "candidate"), flagged, []int32{1})
		},
		"unknown interview": func() (service.LinkOutcome, error) {
			return f.e.roadmaps.LinkCores(f.e.ctx, f.owner, uuid.New(), []int32{1})
		},
		"unassigned position": func() (service.LinkOutcome, error) {
			return f.e.roadmaps.LinkCores(f.e.ctx, f.owner, flagged, []int32{9})
		},
	} {
		if out, err := call(); err != nil || out.Written != 0 || out.Present != 0 {
			t.Errorf("store, %s: outcome %+v, err %v, want nothing written and nothing present", name, out, err)
		}
	}
}

func TestLinkIgnoresRecruitmentInterviews(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 1)
	_, app := rdRecruitmentApplication(t, f)
	rec := uuid.New()
	f.e.exec(t, `INSERT INTO interviews(id,role_profile_id,type,application_id,completed_at,evaluation_status) VALUES($1,NULL,'recruitment',$2,$3,'completed')`, rec, app, rdAt(0))
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = f.e.pool.Exec(ctx, `DELETE FROM roadmap_assessments WHERE interview_id=$1`, rec)
		_, _ = f.e.pool.Exec(ctx, `DELETE FROM interview_questions WHERE interview_id=$1`, rec)
		_, _ = f.e.pool.Exec(ctx, `DELETE FROM interviews WHERE id=$1`, rec)
	})
	f.e.exec(t, `INSERT INTO interview_questions(interview_id,position,core_question_id,score) VALUES($1,1,$2,100)`, rec, q[0])

	res, err := rdLinker(f).LinkPracticeResults(f.e.ctx, rec)
	if err != nil || res.Eligible || f.assessments(t, rec) != 0 {
		t.Fatalf("result = %+v, err = %v, assessments = %d: recruitment evidence must never reach a Roadmap", res, err, f.assessments(t, rec))
	}
	if out, err := f.e.roadmaps.LinkCores(f.e.ctx, f.owner, rec, []int32{1}); err != nil || out.Written != 0 || out.Present != 0 {
		t.Fatalf("store outcome %+v for a recruitment interview, err %v", out, err)
	}
}

func TestLinkConcurrentWorkersWriteOneRowPerCore(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 3)
	iv := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), false, rdScored(q[0], 70), rdScored(q[1], 80), rdSkipped(q[2]))
	link := rdLinker(f)
	const workers = 8
	results := make(chan int, workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			res, err := link.LinkPracticeResults(f.e.ctx, iv)
			errs <- err
			results <- res.Linked
		}()
	}
	total := 0
	for i := 0; i < workers; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		total += <-results
	}
	if total != 3 || f.assessments(t, iv) != 3 {
		t.Fatalf("workers wrote %d rows in total, table has %d, want 3 and 3", total, f.assessments(t, iv))
	}
}

// failingStore fails the write once and then delegates, to prove a link failure leaves the interview result alone
// and a later run still links.
type failingStore struct {
	inner service.AssessmentStore
	fails int
}

func (s *failingStore) LinkCores(ctx context.Context, candidate, interview uuid.UUID, positions []int32) (service.LinkOutcome, error) {
	if s.fails > 0 {
		s.fails--
		return service.LinkOutcome{}, errors.New("simulated write failure")
	}
	return s.inner.LinkCores(ctx, candidate, interview, positions)
}

type fakeEval struct{ calls int }

func (e *fakeEval) Evaluate(context.Context, uuid.UUID) error { e.calls++; return nil }

func TestLinkFailureDoesNotUndoTheInterviewResultAndIsFoundAgain(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 1)
	iv := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), false, rdScored(q[0], 90))
	f.e.exec(t, `UPDATE interviews SET score = 90, score_outcome = 'complete' WHERE id = $1`, iv)
	source := provider.ProvideRoadmapSessionSource(evalrepo.NewRepository(f.e.pool))
	store := &failingStore{inner: f.e.roadmaps, fails: 1}
	hook := service.NewEvaluateThenLink(&fakeEval{}, service.NewAssessmentService(source, store))

	if err := hook.Evaluate(f.e.ctx, iv); err != nil {
		t.Fatalf("Evaluate = %v: a Roadmap failure must not fail the evaluation", err)
	}
	var status string
	var score float64
	if err := f.e.pool.QueryRow(f.e.ctx, `SELECT evaluation_status, score::float8 FROM interviews WHERE id=$1`, iv).Scan(&status, &score); err != nil || status != "completed" || score != 90 {
		t.Fatalf("interview = %s %v, err %v: the saved result must be untouched", status, score, err)
	}
	if f.assessments(t, iv) != 0 {
		t.Fatal("a failed write left a partial link")
	}
	// Not lost: the result shows as an update that is still on its way, and a retry links it.
	if v := f.view(t); v.UpdateState != domain.UpdateUpdating || v.PendingResultCount != 1 {
		t.Fatalf("view = %+v, want updating with 1 pending result", v)
	}
	if err := hook.Evaluate(f.e.ctx, iv); err != nil {
		t.Fatal(err)
	}
	if f.assessments(t, iv) != 1 || f.view(t).UpdateState != domain.UpdateUpToDate {
		t.Fatal("the retry must link the result")
	}
}

func TestLinkOldSessionAfterNewOneKeepsTheCurrentScore(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 1)
	oldSession := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), false, rdScored(q[0], 20))
	newSession := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(5), false, rdScored(q[0], 90))
	link := rdLinker(f)

	// The newer session is linked first, the older one later (a catch-up after a restart): the later link time
	// of the old session must not make its result the current one.
	if _, err := link.LinkPracticeResults(f.e.ctx, newSession); err != nil {
		t.Fatal(err)
	}
	if it := f.view(t).Items[0]; it.Score == nil || *it.Score != 90 {
		t.Fatalf("score = %v, want 90", it.Score)
	}
	if _, err := link.LinkPracticeResults(f.e.ctx, oldSession); err != nil {
		t.Fatal(err)
	}
	v := f.view(t)
	if it := v.Items[0]; it.Score == nil || *it.Score != 90 || it.EvidenceQuestionCount != 1 {
		t.Fatalf("score = %v over %d, want 90 over 1: the old session was evaluated earlier, so the newer result stays current", it.Score, it.EvidenceQuestionCount)
	}
	entries, _, err := f.svc.ItemHistory(f.e.ctx, f.owner, "candidate", f.profile.ID, f.items[0].ID, 20, 0)
	if err != nil || len(entries) != 2 || !entries[0].InCurrentWindow || entries[1].InCurrentWindow {
		t.Fatalf("history = %+v, err %v", entries, err)
	}
}

// The transcript retention pass clears the answer text and keeps the turn. The Roadmap must not read the
// missing text as "not answered": that would turn an old ungraded answer into a skip worth 0.
func TestExpiredTranscriptTextNeverTurnsAnAnswerIntoASkip(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 3)
	ungraded := rdAnswer{question: q[1], delivered: true, answered: true}
	iv := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), false, rdScored(q[0], 80), ungraded)
	skipSession := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(1), false, rdSkipped(q[2]))
	link := rdLinker(f)
	for _, id := range []uuid.UUID{iv, skipSession} {
		if _, err := link.LinkPracticeResults(f.e.ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	before := f.view(t).Items[0]

	// What the retention pass does (interview/repository/query/retention.sql).
	f.e.exec(t, `UPDATE conversation_turns SET answer = NULL, feedback = NULL WHERE interview_id = ANY($1::uuid[])`, []uuid.UUID{iv, skipSession})

	after := f.view(t)
	if after.Items[0].Score == nil || before.Score == nil || *after.Items[0].Score != *before.Score || after.Items[0].EvidenceQuestionCount != before.EvidenceQuestionCount {
		t.Fatalf("score changed after the text expired: %+v -> %+v", before, after.Items[0])
	}
	if after.UpdateState != domain.UpdateUpToDate {
		t.Fatalf("update state = %s: nothing new is pending after retention", after.UpdateState)
	}
	// The answered-but-ungraded core must still not be linked or counted by a later run.
	res, err := link.LinkPracticeResults(f.e.ctx, iv)
	if err != nil || res.Counted != 1 || res.Linked != 0 {
		t.Fatalf("result = %+v, err %v, want 1 counted (the scored core) and nothing new", res, err)
	}
}

// The same through the real retention pass of the interview feature (KAN-356): after it runs, the Roadmap, the
// catch-up filter and the history read exactly as before, with only the expired answer text gone.
func TestRealTranscriptRetentionLeavesTheRoadmapUnchanged(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 3)
	scored := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), false, rdScored(q[0], 80), rdAnswer{question: q[1], delivered: true, answered: true})
	skipped := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(1), false, rdSkipped(q[2]))
	link := rdLinker(f)
	for _, id := range []uuid.UUID{scored, skipped} {
		if _, err := link.LinkPracticeResults(f.e.ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	before := f.view(t)
	beforeHistory, _, err := f.svc.ItemHistory(f.e.ctx, f.owner, "candidate", f.profile.ID, f.items[0].ID, 20, 0)
	if err != nil {
		t.Fatal(err)
	}

	cleared, err := interviewrepo.NewRepository(f.e.pool).ClearExpiredTranscripts(f.e.ctx, rdBase.Add(24*time.Hour), 100)
	if err != nil || cleared < 1 {
		t.Fatalf("retention cleared %d turns, err %v, want at least the seeded answers", cleared, err)
	}
	if left := f.e.count(t, `SELECT count(*) FROM conversation_turns WHERE interview_id = $1 AND answer IS NOT NULL`, scored); left != 0 {
		t.Fatalf("%d answers survived the retention pass: the fixture does not exercise it", left)
	}

	after := f.view(t)
	if after.Items[0].Score == nil || *after.Items[0].Score != *before.Items[0].Score || after.Items[0].EvidenceQuestionCount != before.Items[0].EvidenceQuestionCount || after.UpdateState != domain.UpdateUpToDate {
		t.Fatalf("Roadmap changed after retention: %+v -> %+v", before.Items[0], after.Items[0])
	}
	afterHistory, _, err := f.svc.ItemHistory(f.e.ctx, f.owner, "candidate", f.profile.ID, f.items[0].ID, 20, 0)
	if err != nil || len(afterHistory) != len(beforeHistory) {
		t.Fatalf("history = %d rows after retention, %d before, err %v", len(afterHistory), len(beforeHistory), err)
	}
	for i := range afterHistory {
		if afterHistory[i].Result != beforeHistory[i].Result || afterHistory[i].Score == nil || *afterHistory[i].Score != *beforeHistory[i].Score {
			t.Fatalf("history row %d changed: %+v -> %+v (an expired answer must never become a skip)", i, beforeHistory[i], afterHistory[i])
		}
	}
	for _, id := range f.missing(t) {
		t.Errorf("session %s is listed by the catch-up after retention: expired text must not create work", id)
	}
}
