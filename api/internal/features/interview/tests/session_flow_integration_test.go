//go:build integration

package tests

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// turnRow is one conversation_turns row as stored.
type turnRow struct {
	position int32
	question string
	answer   string
}

// turnRowsOf reads the conversation_turns of a session in position order.
func (s *seed) turnRowsOf(interviewID uuid.UUID) []turnRow {
	s.t.Helper()
	rows, err := s.pool.Query(s.ctx,
		`SELECT position, question, COALESCE(answer, '') FROM public.conversation_turns
		 WHERE interview_id = $1 ORDER BY position`, interviewID)
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	var out []turnRow
	for rows.Next() {
		var r turnRow
		if err := rows.Scan(&r.position, &r.question, &r.answer); err != nil {
			s.t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		s.t.Fatal(err)
	}
	return out
}

// practiceSession starts a practice session of a funded candidate from a fresh confirmed profile with the
// given bank size, and returns it with the bank.
func practiceSession(t *testing.T, ctx context.Context, s *seed, svc *service.Service, candidate uuid.UUID, questions int) (domain.Session, []uuid.UUID) {
	t.Helper()
	profile, bank := s.confirmedProfile(candidate, questions)
	got, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil || !created {
		t.Fatalf("practice Create() created=%v error=%v", created, err)
	}
	return got, bank
}

// TestResumeReturnsSameSessionWithoutCharge proves resume returns the same session and never charges,
// however often it is called.
func TestResumeReturnsSameSessionWithoutCharge(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 4)

	for i := 0; i < 2; i++ {
		got, err := svc.Resume(ctx, candidate, "candidate", started.ID)
		if err != nil {
			t.Fatalf("Resume() #%d error = %v", i+1, err)
		}
		if got.ID != started.ID || got.CompletedAt != nil {
			t.Fatalf("Resume() #%d = %+v, want the open session %s", i+1, got, started.ID)
		}
	}
	if b := s.balance(candidate); b != 15 {
		t.Fatalf("balance = %d after resumes, want 15 (one charge only)", b)
	}
	if rows := s.coinRows(candidate); len(rows) != 1 {
		t.Fatalf("coin rows = %d after resumes, want 1", len(rows))
	}
}

// TestGetIsOwnerOnly proves another Candidate reads a session as not found, with the same code as a
// session that does not exist.
func TestGetIsOwnerOnly(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	owner := s.user("candidate")
	s.wallet(owner, 20)
	started, _ := practiceSession(t, ctx, s, svc, owner, 2)
	intruder := s.user("candidate")

	_, foreignErr := svc.Get(ctx, intruder, "candidate", started.ID)
	_, missingErr := svc.Get(ctx, intruder, "candidate", uuid.New())
	assertCode(t, foreignErr, apperror.CodeInterviewNotFound)
	if codeOf(missingErr) != codeOf(foreignErr) {
		t.Fatalf("missing code %s differs from foreign code %s", codeOf(missingErr), codeOf(foreignErr))
	}
	if _, err := svc.Resume(ctx, intruder, "candidate", started.ID); codeOf(err) != apperror.CodeInterviewNotFound {
		t.Fatalf("Resume() by another candidate = %v, want INTERVIEW_NOT_FOUND", err)
	}
}

// TestTurnsAnswerInOrderAndTrackTheCurrentQuestion answers the three questions of a practice session in
// order. It proves the current question moves on each time, the stored turn carries the question text
// as asked and the answer trimmed, and the last answer ends the questions.
func TestTurnsAnswerInOrderAndTrackTheCurrentQuestion(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	started, bank := practiceSession(t, ctx, s, svc, candidate, 4)

	answers := []string{"  first answer  ", "second answer", "third answer"}
	for i, answer := range answers {
		position := int32(i + 1)
		got, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, position, answer)
		if err != nil {
			t.Fatalf("AppendTurn(%d) error = %v", position, err)
		}
		if got.AnsweredCount != position {
			t.Fatalf("answered count after %d = %d, want %d", position, got.AnsweredCount, position)
		}
		if position < 3 {
			if got.CurrentQuestion == nil || got.CurrentQuestion.Position != position+1 ||
				got.CurrentQuestion.Content != s.contentOf(bank[position]) {
				t.Fatalf("current question after %d = %+v, want position %d", position, got.CurrentQuestion, position+1)
			}
		} else if got.CurrentQuestion != nil {
			t.Fatalf("current question after the last answer = %+v, want nil", got.CurrentQuestion)
		}
	}

	rows := s.turnRowsOf(started.ID)
	if len(rows) != 3 {
		t.Fatalf("turn rows = %+v, want 3", rows)
	}
	for i, row := range rows {
		if row.position != int32(i+1) || row.question != s.contentOf(bank[i]) {
			t.Fatalf("turn %d = %+v, want the bank question of that position", i+1, row)
		}
	}
	if rows[0].answer != "first answer" {
		t.Fatalf("stored answer = %q, want the trimmed text", rows[0].answer)
	}

	final, err := svc.Get(ctx, candidate, "candidate", started.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if final.Turns == nil || len(*final.Turns) != 3 {
		t.Fatalf("practice turns = %+v, want 3 in order", final.Turns)
	}
	for i, turn := range *final.Turns {
		if turn.Position != int32(i+1) || turn.Answer != strings.TrimSpace(answers[i]) {
			t.Fatalf("practice turn %d = %+v", i+1, turn)
		}
	}
}

// TestTurnRetryWithSameAnswerChangesNothing proves a repeated answer for the same position returns the
// session unchanged and stores no second row.
func TestTurnRetryWithSameAnswerChangesNothing(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 4)

	first, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, "same text")
	if err != nil {
		t.Fatalf("first AppendTurn() error = %v", err)
	}
	retry, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, "  same text ")
	if err != nil {
		t.Fatalf("retry AppendTurn() error = %v, want the session unchanged", err)
	}
	if retry.AnsweredCount != first.AnsweredCount || retry.CurrentQuestion == nil || retry.CurrentQuestion.Position != 2 {
		t.Fatalf("retry = %+v, want the same state as the first answer", retry)
	}
	if rows := s.turnRowsOf(started.ID); len(rows) != 1 {
		t.Fatalf("turn rows after retry = %d, want 1", len(rows))
	}
}

// TestTurnConflictOnDifferentAnswerKeepsStoredText proves a different answer for an answered position is
// refused and the stored answer does not change.
func TestTurnConflictOnDifferentAnswerKeepsStoredText(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 4)
	if _, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, "original"); err != nil {
		t.Fatalf("AppendTurn() error = %v", err)
	}

	_, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, "rewritten")
	assertCode(t, err, apperror.CodeInterviewTurnConflict)
	rows := s.turnRowsOf(started.ID)
	if len(rows) != 1 || rows[0].answer != "original" {
		t.Fatalf("turn rows = %+v, want the original answer kept", rows)
	}
}

// TestTurnOutOfOrderIsConflict proves an answer for a position that is not the next unanswered one is
// refused, both a later position and a position past the bank, and nothing is stored.
func TestTurnOutOfOrderIsConflict(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 4)

	for _, position := range []int32{2, 3, 4, 9} {
		_, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, position, "answer")
		assertCode(t, err, apperror.CodeInterviewTurnConflict)
	}
	if rows := s.turnRowsOf(started.ID); len(rows) != 0 {
		t.Fatalf("turn rows = %+v after out-of-order answers, want none", rows)
	}
}

// TestTurnAfterCompletionIsRejected proves a completed session takes no more answers.
func TestTurnAfterCompletionIsRejected(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 4)
	if _, err := svc.Complete(ctx, candidate, "candidate", started.ID); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	_, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, "too late")
	assertCode(t, err, apperror.CodeInterviewCompleted)
	if _, err := svc.Resume(ctx, candidate, "candidate", started.ID); codeOf(err) != apperror.CodeInterviewCompleted {
		t.Fatalf("Resume() of a completed session = %v, want INTERVIEW_COMPLETED", err)
	}
	if rows := s.turnRowsOf(started.ID); len(rows) != 0 {
		t.Fatalf("turn rows = %+v after completion, want none", rows)
	}
}

// TestTurnAnswerBoundsAreEnforced proves an empty answer is refused, and the 8000 character limit is
// counted in characters: 8000 multibyte characters pass and 8001 do not.
func TestTurnAnswerBoundsAreEnforced(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 4)

	_, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, "   ")
	assertCode(t, err, apperror.CodeValidation)

	tooLong := strings.Repeat("é", 8001)
	_, err = svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, tooLong)
	assertCode(t, err, apperror.CodeValidation)

	atLimit := strings.Repeat("é", 8000)
	if _, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, atLimit); err != nil {
		t.Fatalf("AppendTurn() with 8000 characters error = %v", err)
	}
	if rows := s.turnRowsOf(started.ID); len(rows) != 1 {
		t.Fatalf("turn rows = %d, want 1 after the valid answer", len(rows))
	}
}

// TestCompleteIsIdempotentAndEvaluatesOnce proves completing twice keeps the first completion time and
// starts the evaluator once, for the first completion only.
func TestCompleteIsIdempotentAndEvaluatesOnce(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	evaluator := newCountingEvaluator()
	svc.SetEvaluator(evaluator)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 4)

	first, err := svc.Complete(ctx, candidate, "candidate", started.ID)
	if err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}
	if first.CompletedAt == nil {
		t.Fatal("completed_at is nil after Complete()")
	}
	second, err := svc.Complete(ctx, candidate, "candidate", started.ID)
	if err != nil {
		t.Fatalf("second Complete() error = %v", err)
	}
	if second.CompletedAt == nil || !second.CompletedAt.Equal(*first.CompletedAt) {
		t.Fatalf("completed_at changed from %v to %v on the repeat", first.CompletedAt, second.CompletedAt)
	}
	if second.EvaluationStatus != domain.EvaluationPending {
		t.Fatalf("evaluation status = %s, want pending before the evaluator runs", second.EvaluationStatus)
	}

	select {
	case id := <-evaluator.done:
		if id != started.ID {
			t.Fatalf("evaluated %s, want %s", id, started.ID)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("evaluator was not started after the first completion")
	}
	// The repeat starts no evaluation: the spawn happens only on the first completion, which has been
	// consumed above, so nothing else can be queued.
	if len(evaluator.done) != 0 {
		t.Fatalf("%d extra evaluations queued after a repeat completion", len(evaluator.done))
	}
}

// TestRecruitmentCompletionMovesApplicationToInterviewed completes a recruitment session and proves the
// Application moves from interview_eligible to interviewed. A repeat keeps it there.
func TestRecruitmentCompletionMovesApplicationToInterviewed(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.confirmedProfile(candidate, 1)
	_, posting, _ := s.openPosting(slotOf(2), 4)
	application := s.eligibleApplication(posting, candidate)
	started, _, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	done, err := svc.Complete(ctx, candidate, "candidate", started.ID)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if done.CompletedAt == nil {
		t.Fatal("recruitment session not completed")
	}
	if got := s.applicationStatus(application); got != "interviewed" {
		t.Fatalf("application status = %s after completion, want interviewed", got)
	}
	if _, err := svc.Complete(ctx, candidate, "candidate", started.ID); err != nil {
		t.Fatalf("repeat Complete() error = %v", err)
	}
	if got := s.applicationStatus(application); got != "interviewed" {
		t.Fatalf("application status = %s after a repeat, want interviewed", got)
	}
}

// TestPracticeCompletionLeavesApplicationsAlone proves a practice session never touches the Candidate's
// Applications: an interview_eligible Application elsewhere keeps its status.
func TestPracticeCompletionLeavesApplicationsAlone(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	_, posting, _ := s.openPosting(slotOf(2), 4)
	eligible := s.eligibleApplication(posting, candidate)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 4)

	if _, err := svc.Complete(ctx, candidate, "candidate", started.ID); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got := s.applicationStatus(eligible); got != "interview_eligible" {
		t.Fatalf("application status = %s after a practice completion, want interview_eligible", got)
	}
}

// TestOnlyOneCompleteClaimsAFailedOrAbandonedEvaluation proves the evaluation has one owner at a time:
// concurrent repeats of Complete on a failed evaluation start it once, a fresh pending claim is left
// alone, and a pending claim older than the stale window (its run died) is taken over once.
func TestOnlyOneCompleteClaimsAFailedOrAbandonedEvaluation(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	evaluator := newCountingEvaluator()
	svc.SetEvaluator(evaluator)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 4)
	if _, err := svc.Complete(ctx, candidate, "candidate", started.ID); err != nil {
		t.Fatal(err)
	}
	waitForEvaluation(t, evaluator)

	completeTogether := func() {
		var wg sync.WaitGroup
		for range 6 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := svc.Complete(ctx, candidate, "candidate", started.ID); err != nil {
					t.Errorf("Complete() error = %v", err)
				}
			}()
		}
		wg.Wait()
	}

	s.exec(`UPDATE public.interviews SET evaluation_status = 'failed' WHERE id = $1`, started.ID)
	completeTogether()
	waitForEvaluation(t, evaluator)
	time.Sleep(200 * time.Millisecond)
	if n := len(evaluator.done); n != 0 {
		t.Fatalf("%d extra evaluations after concurrent repeats on a failed evaluation, want none", n)
	}

	s.exec(`UPDATE public.interviews SET evaluation_claimed_at = now() - interval '1 hour' WHERE id = $1`, started.ID)
	completeTogether()
	waitForEvaluation(t, evaluator)
	time.Sleep(200 * time.Millisecond)
	if n := len(evaluator.done); n != 0 {
		t.Fatalf("%d extra evaluations after taking over a stale claim, want none", n)
	}
}

func waitForEvaluation(t *testing.T, evaluator *countingEvaluator) {
	t.Helper()
	select {
	case <-evaluator.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the evaluation was not started")
	}
}
