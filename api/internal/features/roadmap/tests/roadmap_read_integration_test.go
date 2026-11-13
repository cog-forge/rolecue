//go:build integration

// Roadmap read API (KAN-352) against a real migrated PostgreSQL database: the repository snapshot, the pure
// scoring and the service together, with practice sessions seeded through SQL. Helpers are prefixed rd so they
// never collide with the helpers of the generation tests in this package.
package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	evalrepo "github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
	rpdomain "github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

var rdBase = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

// rdAnswer describes one core question of a seeded session.
type rdAnswer struct {
	question  uuid.UUID
	score     *float64 // nil: not graded
	delivered bool
	answered  bool
}

func rdScored(q uuid.UUID, score float64) rdAnswer {
	return rdAnswer{question: q, score: &score, delivered: true, answered: true}
}
func rdSkipped(q uuid.UUID) rdAnswer { return rdAnswer{question: q, delivered: true} }

// rdSession inserts a practice (or recruitment) session of the profile completed at base+hours, with its
// questions, turns and scores, and registers the cleanup of its turns.
func rdSession(t *testing.T, e env, profile uuid.UUID, kind, evaluation string, completedAt *time.Time, link bool, answers ...rdAnswer) uuid.UUID {
	t.Helper()
	id := uuid.New()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = e.pool.Exec(ctx, `DELETE FROM roadmap_assessments WHERE interview_id=$1`, id)
		_, _ = e.pool.Exec(ctx, `DELETE FROM conversation_turns WHERE interview_id=$1`, id)
	})
	if kind == "practice" {
		e.exec(t, `INSERT INTO interviews(id,role_profile_id,type,completed_at,evaluation_status) VALUES($1,$2,'practice',$3,$4)`, id, profile, completedAt, evaluation)
	} else {
		// A recruitment session has no role profile; its rows must never reach a practice Roadmap.
		t.Fatal("recruitment sessions are seeded by rdRecruitmentSession")
	}
	turnPosition := 0
	for i, a := range answers {
		pos := i + 1
		e.exec(t, `INSERT INTO interview_questions(interview_id,position,core_question_id,score) VALUES($1,$2,$3,$4)`, id, pos, a.question, a.score)
		if a.delivered {
			e.exec(t, `UPDATE interview_questions SET spoken_text=$3, asked_at=now() WHERE interview_id=$1 AND position=$2`, id, pos, fmt.Sprintf("Spoken question %d", pos))
		}
		if a.answered {
			turnPosition++
			e.exec(t, `INSERT INTO conversation_turns(interview_id,position,kind,question,answer) VALUES($1,$2,'core',$3,'My answer')`, id, turnPosition, fmt.Sprintf("Spoken question %d", pos))
		}
		if link {
			e.exec(t, `INSERT INTO roadmap_assessments(roadmap_item_id,interview_id,question_position)
				SELECT roadmap_item_id,$1,$2 FROM core_questions WHERE id=$3`, id, pos, a.question)
		}
	}
	return id
}

func rdAt(h int) *time.Time {
	v := rdBase.Add(time.Duration(h) * time.Hour)
	return &v
}

type rdFixture struct {
	e       env
	svc     *service.ReadService
	owner   uuid.UUID
	profile rpdomain.RoleProfile
	items   []domain.Item
}

func rdNew(t *testing.T, titles ...string) rdFixture {
	t.Helper()
	e := newEnv(t)
	owner := e.user(t, "candidate")
	profile := e.profile(t, owner)
	// The clock stands before every seeded session, so a result that is not linked yet is "updating" and never
	// "delayed" unless a test moves the clock.
	f := rdFixture{e: e, svc: service.NewReadService(e.roadmaps).WithClock(func() time.Time { return *rdAt(0) }), owner: owner, profile: profile}
	if len(titles) > 0 {
		f.items = e.publish(t, e.roadmap(t, owner, profile.ID).ID, titles...)
	}
	return f
}

func (f rdFixture) questions(t *testing.T, item domain.Item, n int) []uuid.UUID {
	t.Helper()
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = f.e.question(t, f.profile.JDID, item.ID)
	}
	return out
}

func (f rdFixture) view(t *testing.T) domain.RoadmapView {
	t.Helper()
	v, err := f.svc.Get(f.e.ctx, f.owner, "candidate", f.profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func rdWantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func TestReadPendingRoadmapOfANewProfile(t *testing.T) {
	f := rdNew(t)
	v := f.view(t)
	if v.GenerationStatus != domain.StatusPending || v.LearningStatus != domain.LearningNotStarted || v.ProgressPercent != nil {
		t.Fatalf("view = %+v", v)
	}
	if v.TotalItems != 0 || len(v.Items) != 0 || v.PracticeStarted || v.UpdateState != domain.UpdateUpToDate {
		t.Fatalf("view = %+v", v)
	}
}

func TestReadReadyEmptyRoadmapHasNoProgress(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	profile := e.profile(t, owner)
	e.publish(t, e.roadmap(t, owner, profile.ID).ID) // ready with no topics
	v, err := service.NewReadService(e.roadmaps).Get(e.ctx, owner, "candidate", profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.GenerationStatus != domain.StatusReady || v.ProgressPercent != nil || v.LearningStatus != domain.LearningNotStarted || v.TotalItems != 0 {
		t.Fatalf("view = %+v", v)
	}
}

func TestReadFailedRoadmap(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	profile := e.profile(t, owner)
	roadmap := e.roadmap(t, owner, profile.ID)
	fence, err := e.roadmaps.Claim(e.ctx, roadmap.ID, []domain.GenerationStatus{domain.StatusPending}, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.roadmaps.Fail(e.ctx, roadmap.ID, fence); err != nil {
		t.Fatal(err)
	}
	v, err := service.NewReadService(e.roadmaps).Get(e.ctx, owner, "candidate", profile.ID)
	if err != nil || v.GenerationStatus != domain.StatusFailed || v.ProgressPercent != nil {
		t.Fatalf("view = %+v, err = %v", v, err)
	}
}

func TestReadReadyWithoutResultsShowsTopicsInPositionOrder(t *testing.T) {
	f := rdNew(t, "SQL", "Go concurrency", "System design")
	v := f.view(t)
	if v.GenerationStatus != domain.StatusReady || v.LearningStatus != domain.LearningNotStarted || v.TotalItems != 3 {
		t.Fatalf("view = %+v", v)
	}
	if v.ProgressPercent == nil || *v.ProgressPercent != 0 {
		t.Fatalf("progress = %v, want 0", v.ProgressPercent)
	}
	for i, want := range []string{"SQL", "Go concurrency", "System design"} {
		it := v.Items[i]
		if it.Title != want || it.Score != nil || it.EvidenceQuestionCount != 0 || it.LearningStatus != domain.TopicNotStarted || it.LastEvaluatedAt != nil {
			t.Fatalf("item %d = %+v", i, it)
		}
		if it.Description == nil {
			t.Errorf("item %d lost its description", i)
		}
	}
}

// TestReadSpecExample reproduces spec 4.3 and 4.4 end to end through the database: SQL 70,80,90,80,90 = 82
// achieved; then the 70 question answered again with 20 -> 72; Go 72 in progress; System design not started.
func TestReadSpecExampleThroughTheDatabase(t *testing.T) {
	f := rdNew(t, "SQL", "Go concurrency", "System design")
	sql := f.questions(t, f.items[0], 5)
	gq := f.questions(t, f.items[1], 5)

	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), true, rdScored(sql[0], 70), rdScored(sql[1], 80), rdScored(gq[0], 60))
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(1), true, rdScored(sql[2], 90), rdScored(sql[3], 80), rdScored(gq[1], 70))
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(2), true, rdScored(sql[4], 90), rdScored(gq[2], 70), rdScored(gq[3], 75), rdScored(gq[4], 85))

	v := f.view(t)
	if v.LearningStatus != domain.LearningInProgress || v.CompletedItems != 1 || v.TotalItems != 3 || !v.PracticeStarted {
		t.Fatalf("view = %+v", v)
	}
	if v.ProgressPercent == nil || *v.ProgressPercent != 33.33 {
		t.Fatalf("progress = %v, want 33.33", v.ProgressPercent)
	}
	s := v.Items[0]
	if s.Score == nil || *s.Score != 82 || s.EvidenceQuestionCount != 5 || s.LearningStatus != domain.TopicAchieved || s.LastEvaluatedAt == nil || !s.LastEvaluatedAt.Equal(*rdAt(2)) {
		t.Fatalf("sql = %+v", s)
	}
	g := v.Items[1]
	if g.Score == nil || *g.Score != 72 || g.EvidenceQuestionCount != 5 || g.LearningStatus != domain.TopicInProgress {
		t.Fatalf("go = %+v", g)
	}
	if s.LastEvaluatedAt.Location() != time.UTC {
		t.Fatalf("last_evaluated_at is in %s, want UTC", s.LastEvaluatedAt.Location())
	}
	if sd := v.Items[2]; sd.Score != nil || sd.LearningStatus != domain.TopicNotStarted {
		t.Fatalf("system design = %+v", sd)
	}

	// The question that scored 70 is answered again with 20: it contributes 20 instead of 70 -> 72.
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(3), true, rdScored(sql[0], 20))
	again := f.view(t).Items[0]
	if again.Score == nil || *again.Score != 72 || again.LearningStatus != domain.TopicInProgress || again.EvidenceQuestionCount != 5 {
		t.Fatalf("sql after the repeat = %+v, want 72 in progress over 5 questions", again)
	}
	if f.view(t).CompletedItems != 0 || f.view(t).LearningStatus != domain.LearningInProgress {
		t.Fatal("the Roadmap must fall back with the topic")
	}
}

func TestReadSkippedCountsZeroAndUngradedAndUnaskedDoNot(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 6)
	// 4 answers of 100, one delivered and skipped (0), one never asked and one answered but ungraded.
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), true,
		rdScored(q[0], 100), rdScored(q[1], 100), rdScored(q[2], 100), rdScored(q[3], 100), rdSkipped(q[4]))
	unasked := rdAnswer{question: q[5]}
	ungraded := rdAnswer{question: f.e.question(t, f.profile.JDID, f.items[0].ID), delivered: true, answered: true}
	// The cores of a session are asked and answered in order, so each of these needs a session of its own.
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(1), true, ungraded)
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(2), true, unasked)

	it := f.view(t).Items[0]
	if it.Score == nil || *it.Score != 80 || it.EvidenceQuestionCount != 5 || it.LearningStatus != domain.TopicAchieved {
		t.Fatalf("item = %+v want 80 over 5 questions (the skip is a 0, the other two are not counted)", it)
	}
}

func TestReadIgnoresSessionsThatAreNotEvidence(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 5)
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), true, rdScored(q[0], 60))
	rdSession(t, f.e, f.profile.ID, "practice", "pending", rdAt(1), true, rdScored(q[1], 100))
	rdSession(t, f.e, f.profile.ID, "practice", "failed", rdAt(2), true, rdScored(q[2], 100))
	rdSession(t, f.e, f.profile.ID, "practice", "completed", nil, true, rdScored(q[3], 100)) // unfinished

	v := f.view(t)
	if it := v.Items[0]; it.Score == nil || *it.Score != 60 || it.EvidenceQuestionCount != 1 {
		t.Fatalf("item = %+v, want only the completed and evaluated session", it)
	}
	if v.UpdateState != domain.UpdateUpToDate {
		t.Fatalf("update state = %s: sessions that are not evidence must not make the Roadmap look unfinished", v.UpdateState)
	}
}

func TestReadUpdateStateComesFromResultsWithoutAssessments(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 3)
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), true, rdScored(q[0], 60))
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(1), false, rdScored(q[1], 100), rdSkipped(q[2])) // evaluated, not linked yet

	v := f.view(t)
	if v.UpdateState != domain.UpdateUpdating || v.PendingResultCount != 2 {
		t.Fatalf("update state = %s/%d, want updating/2", v.UpdateState, v.PendingResultCount)
	}
	// The result that is not linked yet is neither scored nor shown as a 0.
	if it := v.Items[0]; it.Score == nil || *it.Score != 60 || it.EvidenceQuestionCount != 1 {
		t.Fatalf("item = %+v, want only the linked result", it)
	}
}

func TestReadNothingLinkedYetIsNotStartedNotZero(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 1)
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), false, rdScored(q[0], 90))
	v := f.view(t)
	if it := v.Items[0]; it.Score != nil || it.LearningStatus != domain.TopicNotStarted || v.UpdateState != domain.UpdateUpdating || v.LearningStatus != domain.LearningNotStarted {
		t.Fatalf("view = %+v", v)
	}
}

func TestReadPracticeStartedIncludesUnfinishedSessions(t *testing.T) {
	f := rdNew(t, "SQL")
	if f.view(t).PracticeStarted {
		t.Fatal("practice_started before any session")
	}
	q := f.questions(t, f.items[0], 1)
	rdSession(t, f.e, f.profile.ID, "practice", "pending", nil, false, rdSkipped(q[0]))
	if !f.view(t).PracticeStarted {
		t.Fatal("an unfinished session must count as started")
	}
}

func TestReadHistoryWindowFlagsAndPaging(t *testing.T) {
	f := rdNew(t, "SQL", "Go")
	sql := f.questions(t, f.items[0], 6)
	gq := f.questions(t, f.items[1], 1)
	for i := 0; i < 6; i++ {
		rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(i), true, rdScored(sql[i], float64(50+i*5)))
	}
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(6), true, rdScored(sql[0], 20), rdScored(gq[0], 99)) // repeats Q1, adds a Go answer

	entries, total, err := f.svc.ItemHistory(f.e.ctx, f.owner, "candidate", f.profile.ID, f.items[0].ID, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 7 || len(entries) != 7 {
		t.Fatalf("history = %d entries, total %d, want 7 of the topic only", len(entries), total)
	}
	inWindow := 0
	for i, en := range entries {
		if i > 0 && en.EvaluatedAt.After(entries[i-1].EvaluatedAt) {
			t.Fatal("history is not newest first")
		}
		if en.InCurrentWindow {
			inWindow++
		}
		if en.QuestionText == nil || en.Score == nil || en.Result != domain.CoreScored {
			t.Fatalf("entry = %+v", en)
		}
	}
	if inWindow != 5 {
		t.Fatalf("%d entries in the current window, want 5", inWindow)
	}
	// The window of the history is exactly what the Roadmap scores: Q1 (20, latest), Q6, Q5, Q4, Q3.
	want := (20.0 + 75 + 70 + 65 + 60) / 5
	if it := f.view(t).Items[0]; it.Score == nil || *it.Score != want {
		t.Fatalf("topic score = %v, want %v", it.Score, want)
	}
	page, total, err := f.svc.ItemHistory(f.e.ctx, f.owner, "candidate", f.profile.ID, f.items[0].ID, 3, 5)
	if err != nil || total != 7 || len(page) != 2 {
		t.Fatalf("last page = %d entries of %d, err %v", len(page), total, err)
	}
	// The older answer to Q1 is in the history but outside the window.
	last := entries[len(entries)-1]
	if last.InCurrentWindow || last.Score == nil || *last.Score != 50 {
		t.Fatalf("oldest entry = %+v, want the superseded 50 outside the window", last)
	}
}

func TestReadHistoryShowsASkippedQuestionAsZeroWithoutItsText(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 2)
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), true, rdScored(q[0], 80), rdSkipped(q[1]))
	entries, _, err := f.svc.ItemHistory(f.e.ctx, f.owner, "candidate", f.profile.ID, f.items[0].ID, 20, 0)
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries = %v, err = %v", entries, err)
	}
	var skipped *domain.HistoryEntry
	for i := range entries {
		if entries[i].Result == domain.CoreSkipped {
			skipped = &entries[i]
		}
	}
	if skipped == nil || skipped.Score == nil || *skipped.Score != 0 || skipped.QuestionText != nil || skipped.QuestionNumber != 2 {
		t.Fatalf("skipped entry = %+v", skipped)
	}
}

func TestReadHistoryEmptyAndUnknownTopic(t *testing.T) {
	f := rdNew(t, "SQL", "Go")
	entries, total, err := f.svc.ItemHistory(f.e.ctx, f.owner, "candidate", f.profile.ID, f.items[0].ID, 20, 0)
	if err != nil || total != 0 || entries == nil || len(entries) != 0 {
		t.Fatalf("empty history = %#v, %d, %v", entries, total, err)
	}
	_, _, err = f.svc.ItemHistory(f.e.ctx, f.owner, "candidate", f.profile.ID, uuid.New(), 20, 0)
	rdWantCode(t, err, apperror.CodeRoadmapItemNotFound)
}

func TestReadIsOwnerOnly(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 1)
	rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), true, rdScored(q[0], 90))
	stranger := f.e.user(t, "candidate")
	recruiter := f.e.user(t, "recruiter")

	// A foreign Candidate gets the same answer as for a missing profile, for the Roadmap and the history.
	_, err := f.svc.Get(f.e.ctx, stranger, "candidate", f.profile.ID)
	rdWantCode(t, err, apperror.CodeJDNotFound)
	_, _, err = f.svc.ItemHistory(f.e.ctx, stranger, "candidate", f.profile.ID, f.items[0].ID, 20, 0)
	rdWantCode(t, err, apperror.CodeJDNotFound)
	_, err = f.svc.Get(f.e.ctx, f.owner, "candidate", uuid.New())
	rdWantCode(t, err, apperror.CodeJDNotFound)

	// A topic of someone else's Roadmap is not found through your own profile.
	own := f.e.profile(t, stranger)
	ownRoadmap := f.e.publish(t, f.e.roadmap(t, stranger, own.ID).ID, "Other")
	_, _, err = f.svc.ItemHistory(f.e.ctx, stranger, "candidate", own.ID, f.items[0].ID, 20, 0)
	rdWantCode(t, err, apperror.CodeRoadmapItemNotFound)
	if _, _, err = f.svc.ItemHistory(f.e.ctx, stranger, "candidate", own.ID, ownRoadmap[0].ID, 20, 0); err != nil {
		t.Fatalf("a Candidate must read their own history: %v", err)
	}

	// The owner's role must be candidate: a Recruiter or Admin is refused even with the owner's id.
	for _, role := range []string{"recruiter", "admin"} {
		_, err = f.svc.Get(f.e.ctx, f.owner, role, f.profile.ID)
		rdWantCode(t, err, apperror.CodeForbidden)
	}
	_, err = f.svc.Get(f.e.ctx, recruiter, "recruiter", f.profile.ID)
	rdWantCode(t, err, apperror.CodeForbidden)
}

func TestReadNeverReturnsRecruitmentResults(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 1)
	posting, app := rdRecruitmentApplication(t, f)
	_ = posting
	rec := uuid.New()
	f.e.exec(t, `INSERT INTO interviews(id,role_profile_id,type,application_id,completed_at,evaluation_status) VALUES($1,NULL,'recruitment',$2,$3,'completed')`, rec, app, rdAt(0))
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = f.e.pool.Exec(ctx, `DELETE FROM interview_questions WHERE interview_id=$1`, rec)
		_, _ = f.e.pool.Exec(ctx, `DELETE FROM interviews WHERE id=$1`, rec)
	})
	f.e.exec(t, `INSERT INTO interview_questions(interview_id,position,core_question_id,score) VALUES($1,1,$2,100)`, rec, q[0])

	v := f.view(t)
	if it := v.Items[0]; it.Score != nil || it.EvidenceQuestionCount != 0 || v.UpdateState != domain.UpdateUpToDate || v.PracticeStarted {
		t.Fatalf("a recruitment result leaked into the Roadmap: %+v", v)
	}
	entries, total, err := f.svc.ItemHistory(f.e.ctx, f.owner, "candidate", f.profile.ID, f.items[0].ID, 20, 0)
	if err != nil || total != 0 || len(entries) != 0 {
		t.Fatalf("history = %v %d %v", entries, total, err)
	}
}

// rdRecruitmentApplication inserts a recruiter, a posting and an application of the owner.
func rdRecruitmentApplication(t *testing.T, f rdFixture) (posting, app uuid.UUID) {
	t.Helper()
	recruiter := f.e.user(t, "recruiter")
	f.e.exec(t, `UPDATE users SET company_name='Acme' WHERE id=$1`, recruiter)
	posting, app = uuid.New(), uuid.New()
	f.e.exec(t, `INSERT INTO job_postings(id,status,jd_id,recruiter_id,title) VALUES($1,'open'::public.job_posting_status,$2,$3,'KAN352 posting')`, posting, f.profile.JDID, recruiter)
	f.e.exec(t, `INSERT INTO applications(id,job_posting_id,candidate_id,status,cv_path) VALUES($1,$2,$3,'interviewed'::public.application_status,'cvs/kan352.pdf')`, app, posting, f.owner)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = f.e.pool.Exec(ctx, `DELETE FROM applications WHERE id=$1`, app)
		_, _ = f.e.pool.Exec(ctx, `DELETE FROM job_postings WHERE id=$1`, posting)
	})
	return posting, app
}

// TestReadRowsAndEvaluationFactsAgree guards the two SQL derivations of "delivered" and "answered": the
// Roadmap read query (roadmap/repository) and the evaluation-side adapter query (evaluation/repository, the one
// the assessment writer uses). If they ever disagree, update_state would never settle.
func TestReadRowsAndEvaluationFactsAgree(t *testing.T) {
	f := rdNew(t, "SQL")
	q := f.questions(t, f.items[0], 4)
	// Answered and graded, answered and ungraded, delivered and skipped, never reached: in the order the engine
	// would produce (it asks and answers in order, so only the last shown question can be unanswered).
	scored := rdScored(q[0], 80)
	ungraded := rdAnswer{question: q[1], delivered: true, answered: true}
	iv := rdSession(t, f.e, f.profile.ID, "practice", "completed", rdAt(0), false, scored, ungraded, rdSkipped(q[2]), rdAnswer{question: q[3]})

	_, rows, err := f.e.roadmaps.LoadReadModel(f.e.ctx, f.owner, f.profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := evalrepo.NewRepository(f.e.pool).CoreResultFacts(f.e.ctx, iv)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || len(facts) != 4 {
		t.Fatalf("rows = %d, facts = %d, want 4 and 4", len(rows), len(facts))
	}
	byPosition := map[int32]domain.ResultRow{}
	for _, r := range rows {
		byPosition[r.Position] = r
	}
	for _, fact := range facts {
		r := byPosition[fact.Position]
		if r.Delivered != fact.Delivered || r.Answered != fact.Answered {
			t.Errorf("position %d: read query delivered/answered = %v/%v, evaluation adapter = %v/%v",
				fact.Position, r.Delivered, r.Answered, fact.Delivered, fact.Answered)
		}
		if (r.QuestionScore == nil) != (fact.Score == nil) || (fact.Score != nil && *r.QuestionScore != *fact.Score) {
			t.Errorf("position %d: scores differ (%v vs %v)", fact.Position, r.QuestionScore, fact.Score)
		}
	}
}
