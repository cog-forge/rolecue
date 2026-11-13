//go:build integration

// Roadmap practice-result adapter (KAN-349, RM-08) against a real PostgreSQL database that is migrated and
// disposable: what is stored about delivered, answered, skipped and unasked core questions, and how the
// Roadmap rule reads it. The seed helpers come from scoring_integration_test.go.
package tests

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	roadmap "github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/google/uuid"
)

// deliver marks a core question as delivered to the Candidate, the way the live engine does.
func (sc *scSeed) deliver(interviewID uuid.UUID, position int32) {
	sc.exec(`UPDATE public.interview_questions
		SET spoken_text = 'spoken', asked_at = now() WHERE interview_id = $1 AND position = $2`, interviewID, position)
}

func (sc *scSeed) setQuestionScore(interviewID uuid.UUID, position int32, score float64) {
	sc.exec(`UPDATE public.interview_questions SET score = $3 WHERE interview_id = $1 AND position = $2`, interviewID, position, score)
}

func factsByPosition(t *testing.T, facts []domain.CoreResultFacts) map[int32]domain.CoreResultFacts {
	t.Helper()
	out := make(map[int32]domain.CoreResultFacts, len(facts))
	for _, f := range facts {
		out[f.Position] = f
	}
	return out
}

func TestCoreResultFactsDistinguishAnsweredSkippedAndUnasked(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	candidate := sc.user("candidate")
	jd := sc.jd()
	role := sc.roleProfile(candidate, jd)
	questions := make([]uuid.UUID, 0, 5)
	for _, content := range []string{"q1", "q2", "q3", "q4", "q5"} {
		questions = append(questions, sc.question(jd, content, nil))
	}
	// The Candidate answered q1, then q2 and one follow-up to it, was shown q3, and left. q4 and q5 were never
	// reached.
	iv := sc.interview(role, "practice", nil, questions...)
	sc.turn(iv, 1, "core", "q1", scStr("answer one"))
	sc.turn(iv, 2, "core", "q2", scStr("answer two"))
	sc.turn(iv, 3, "follow_up", "why?", scStr("because"))
	sc.deliver(iv, 1)
	sc.deliver(iv, 2)
	sc.deliver(iv, 3)
	sc.setQuestionScore(iv, 1, 70.5)
	sc.setQuestionScore(iv, 2, 40)

	facts, err := repo.CoreResultFacts(ctx, iv)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 5 {
		t.Fatalf("facts = %d rows, want 5", len(facts))
	}
	byPos := factsByPosition(t, facts)
	for pos, want := range map[int32]struct {
		delivered, answered bool
		score               *float64
		outcome             roadmap.CoreOutcome
		counted             float64
	}{
		1: {true, true, ptrF(70.5), roadmap.CoreScored, 70.5},
		2: {true, true, ptrF(40), roadmap.CoreScored, 40},
		3: {true, false, nil, roadmap.CoreSkipped, 0},
		4: {false, false, nil, roadmap.CoreNotAsked, -1},
		5: {false, false, nil, roadmap.CoreNotAsked, -1},
	} {
		got := byPos[pos]
		if got.CoreQuestionID != questions[pos-1] {
			t.Errorf("position %d core question = %s, want %s", pos, got.CoreQuestionID, questions[pos-1])
		}
		if got.Delivered != want.delivered || got.Answered != want.answered {
			t.Errorf("position %d delivered/answered = %v/%v, want %v/%v", pos, got.Delivered, got.Answered, want.delivered, want.answered)
		}
		if (got.Score == nil) != (want.score == nil) || (got.Score != nil && *got.Score != *want.score) {
			t.Errorf("position %d score = %v, want %v", pos, got.Score, want.score)
		}
		result := roadmap.ClassifyCore(roadmap.CoreFacts{Position: pos, Delivered: got.Delivered, Answered: got.Answered, QuestionScore: got.Score})
		if result.Outcome != want.outcome {
			t.Errorf("position %d outcome = %s, want %s", pos, result.Outcome, want.outcome)
		}
		if want.counted >= 0 && (result.Score == nil || *result.Score != want.counted) {
			t.Errorf("position %d counted score = %v, want %v", pos, result.Score, want.counted)
		}
		if want.counted < 0 && result.Score != nil {
			t.Errorf("position %d counted score = %v, want not counted", pos, *result.Score)
		}
	}
}

func ptrF(v float64) *float64 { return &v }

func TestCoreResultFactsAnsweredButUngradedIsNotZero(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	candidate := sc.user("candidate")
	jd := sc.jd()
	iv := sc.interview(sc.roleProfile(candidate, jd), "practice", nil, sc.question(jd, "legacy question", nil))
	sc.turn(iv, 1, "core", "legacy question", scStr("an answer"))
	sc.deliver(iv, 1)

	facts, err := repo.CoreResultFacts(ctx, iv)
	if err != nil || len(facts) != 1 {
		t.Fatalf("facts = %v, %v", facts, err)
	}
	if !facts[0].Answered || facts[0].Score != nil {
		t.Fatalf("facts = %+v, want answered without a score", facts[0])
	}
	if got := roadmap.ClassifyCore(roadmap.CoreFacts{Delivered: facts[0].Delivered, Answered: facts[0].Answered, QuestionScore: facts[0].Score}); got.Outcome != roadmap.CoreUngraded || got.Counted() {
		t.Fatalf("outcome = %+v, want ungraded and not counted", got)
	}
}

func TestCoreResultFactsAnswerOverRESTWithoutDeliveryIsStillAnswered(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	candidate := sc.user("candidate")
	jd := sc.jd()
	iv := sc.interview(sc.roleProfile(candidate, jd), "practice", nil, sc.question(jd, "q1", nil), sc.question(jd, "q2", nil))
	sc.turn(iv, 1, "core", "q1", scStr("rest answer"))
	sc.setQuestionScore(iv, 1, 55)

	facts, err := repo.CoreResultFacts(ctx, iv)
	if err != nil {
		t.Fatal(err)
	}
	byPos := factsByPosition(t, facts)
	if f := byPos[1]; f.Delivered || !f.Answered || f.Score == nil || *f.Score != 55 {
		t.Fatalf("position 1 = %+v, want answered, scored, not marked delivered", f)
	}
	// Without a delivery record the unanswered question cannot be told from an unasked one: not counted.
	if f := byPos[2]; f.Delivered || f.Answered {
		t.Fatalf("position 2 = %+v, want neither delivered nor answered", f)
	}
}

func TestCoreResultFactsNoQuestions(t *testing.T) {
	ctx, pool := scOpenPool(t)
	repo := repository.NewRepository(pool)
	facts, err := repo.CoreResultFacts(ctx, uuid.New())
	if err != nil || len(facts) != 0 {
		t.Fatalf("facts = %v, err = %v, want none", facts, err)
	}
}

func TestInterviewRecordDecidesRoadmapEligibility(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	candidate := sc.user("candidate")
	jd := sc.jd()
	role := sc.roleProfile(candidate, jd)
	question := sc.question(jd, "q", nil)

	eligibleOf := func(id uuid.UUID) bool {
		t.Helper()
		record, err := repo.InterviewRecord(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return roadmap.SessionFacts{Type: record.Type, CompletedAt: record.CompletedAt, EvaluationStatus: record.EvaluationStatus}.Eligible()
	}

	done := sc.interview(role, "practice", nil, question)
	sc.exec(`UPDATE public.interviews SET evaluation_status = 'completed' WHERE id = $1`, done)
	if !eligibleOf(done) {
		t.Error("completed practice session with a successful evaluation must be eligible")
	}
	pending := sc.interview(role, "practice", nil, question)
	if eligibleOf(pending) {
		t.Error("a pending evaluation must not be eligible")
	}
	failed := sc.interview(role, "practice", nil, question)
	sc.exec(`UPDATE public.interviews SET evaluation_status = 'failed' WHERE id = $1`, failed)
	if eligibleOf(failed) {
		t.Error("a failed evaluation must not be eligible")
	}
	open := sc.interview(role, "practice", nil, question)
	sc.exec(`UPDATE public.interviews SET completed_at = NULL, evaluation_status = 'completed' WHERE id = $1`, open)
	if eligibleOf(open) {
		t.Error("an unfinished session must not be eligible")
	}

	recruiter := sc.user("recruiter")
	posting := sc.posting(recruiter, jd)
	app := sc.application(posting, candidate, "interviewed")
	recruitment := sc.interview(role, "recruitment", &app, question)
	sc.exec(`UPDATE public.interviews SET evaluation_status = 'completed' WHERE id = $1`, recruitment)
	if eligibleOf(recruitment) {
		t.Error("a recruitment session must never be eligible")
	}
}
