//go:build integration

package tests

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// practiceInput asks for a practice session from the role profile.
func practiceInput(profileID uuid.UUID) domain.CreateInput {
	return domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profileID}
}

// interviewRows counts the interviews that start from a role profile.
func (s *seed) interviewRows(profileID uuid.UUID) int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM public.interviews WHERE role_profile_id = $1`, profileID)
}

// questionRows counts the interview_questions rows of every session from a role profile.
func (s *seed) questionRows(profileID uuid.UUID) int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM public.interview_questions iq
		JOIN public.interviews i ON i.id = iq.interview_id WHERE i.role_profile_id = $1`, profileID)
}

// TestPracticeCreateSnapshotsQuestionsAndChargesOnce starts a practice session from a confirmed profile
// with four bank questions and a limit of three. It proves the session asks the first three bank
// questions in order, that the first one is the current question, and that exactly one coin debit of
// the configured price was written.
func TestPracticeCreateSnapshotsQuestionsAndChargesOnce(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, bank := s.confirmedProfile(candidate, 4)

	got, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !created {
		t.Fatal("created = false, want true for a new practice session")
	}
	if got.Type != domain.TypePractice || got.QuestionCount != 3 || got.AnsweredCount != 0 || got.CompletedAt != nil {
		t.Fatalf("Create() = %+v, want a fresh practice session of 3 questions", got)
	}
	if got.CurrentQuestion == nil || got.CurrentQuestion.Position != 1 || got.CurrentQuestion.Content != s.contentOf(bank[0]) {
		t.Fatalf("current question = %+v, want position 1 with the first bank question", got.CurrentQuestion)
	}

	snap := s.snapshot(got.ID)
	if len(snap) != 3 {
		t.Fatalf("snapshot has %d questions, want 3", len(snap))
	}
	for i := range snap {
		if snap[i] != bank[i] {
			t.Fatalf("snapshot[%d] = %s, want bank question %s (bank order by id)", i, snap[i], bank[i])
		}
	}

	if b := s.balance(candidate); b != 15 {
		t.Fatalf("balance = %d, want 15 after one practice charge of 5", b)
	}
	rows := s.coinRows(candidate)
	if len(rows) != 1 {
		t.Fatalf("coin rows = %+v, want exactly one", rows)
	}
	r := rows[0]
	if r.amount != 5 || r.from != candidate.String() || r.to != "system" || r.currency != "COIN" || r.status != "success" {
		t.Fatalf("coin row = %+v, want a successful COIN debit of 5 from the Candidate to system", r)
	}
	if r.description != "Practice interview "+got.ID.String() {
		t.Fatalf("coin row description = %q, want it to name the session", r.description)
	}
}

// TestPracticeInsufficientBalanceRollsBack proves a practice start the Candidate cannot pay for leaves
// no interview, no questions and no ledger row. The session rows are written before the charge, so
// this is the rollback that keeps them out.
func TestPracticeInsufficientBalanceRollsBack(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 4)
	profile, _ := s.confirmedProfile(candidate, 4)

	_, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	assertCode(t, err, apperror.CodeInsufficientBalance)
	if n := s.interviewRows(profile); n != 0 {
		t.Fatalf("interview rows = %d after a failed charge, want 0", n)
	}
	if n := s.questionRows(profile); n != 0 {
		t.Fatalf("interview_questions rows = %d after a failed charge, want 0", n)
	}
	if b := s.balance(candidate); b != 4 {
		t.Fatalf("balance = %d, want 4 unchanged", b)
	}
	if rows := s.coinRows(candidate); len(rows) != 0 {
		t.Fatalf("coin rows = %+v, want none", rows)
	}
}

// TestPracticeRequiresConfirmedRoleProfile proves a draft role profile cannot start a session and
// nothing is written or charged.
func TestPracticeRequiresConfirmedRoleProfile(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.roleProfile(candidate, "draft", 4)

	_, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	assertCode(t, err, apperror.CodeRoleProfileRequired)
	if n := s.interviewRows(profile); n != 0 {
		t.Fatalf("interview rows = %d for a draft profile, want 0", n)
	}
	if b := s.balance(candidate); b != 20 {
		t.Fatalf("balance = %d, want 20 unchanged", b)
	}
}

// TestPracticeWithEmptyQuestionBankIsNotReady proves a confirmed profile whose Job Description has no
// questions cannot start a session, and nothing is written or charged.
func TestPracticeWithEmptyQuestionBankIsNotReady(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 0)

	_, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	assertCode(t, err, apperror.CodeQuestionBankNotReady)
	if n := s.interviewRows(profile); n != 0 {
		t.Fatalf("interview rows = %d for an empty bank, want 0", n)
	}
	if b := s.balance(candidate); b != 20 {
		t.Fatalf("balance = %d, want 20 unchanged", b)
	}
}

// TestPracticeForeignRoleProfileIsHidden proves another Candidate's confirmed profile is reported exactly
// as a profile that does not exist, so its existence is not revealed. Nothing is charged to the caller.
func TestPracticeForeignRoleProfileIsHidden(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	owner := s.user("candidate")
	other := s.user("candidate")
	s.wallet(other, 20)
	foreign, _ := s.confirmedProfile(owner, 4)

	_, _, foreignErr := svc.Create(ctx, other, "candidate", practiceInput(foreign))
	missing := uuid.New()
	_, _, missingErr := svc.Create(ctx, other, "candidate", practiceInput(missing))

	if codeOf(foreignErr) == "" {
		t.Fatalf("foreign profile error = %v, want an AppError", foreignErr)
	}
	if codeOf(foreignErr) != codeOf(missingErr) {
		t.Fatalf("foreign profile code %s differs from missing profile code %s", codeOf(foreignErr), codeOf(missingErr))
	}
	if n := s.interviewRows(foreign); n != 0 {
		t.Fatalf("interview rows on the owner's profile = %d, want 0", n)
	}
	if b := s.balance(other); b != 20 {
		t.Fatalf("balance of the caller = %d, want 20 unchanged", b)
	}
}

// TestPracticeEachNewSessionIsCharged proves every new practice session is its own charge: two starts
// from the same profile make two sessions and two debits.
func TestPracticeEachNewSessionIsCharged(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 4)

	first, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	second, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}
	if !created || second.ID == first.ID {
		t.Fatalf("second practice start = %s created %v, want a new session distinct from %s", second.ID, created, first.ID)
	}
	if n := s.interviewRows(profile); n != 2 {
		t.Fatalf("interview rows = %d, want 2", n)
	}
	if b := s.balance(candidate); b != 10 {
		t.Fatalf("balance = %d, want 10 after two charges of 5", b)
	}
	if rows := s.coinRows(candidate); len(rows) != 2 {
		t.Fatalf("coin rows = %d, want 2", len(rows))
	}
}

// TestPracticeDefaultQuestionLimitIsEight proves a zero configured limit falls back to eight questions,
// the default the service documents.
func TestPracticeDefaultQuestionLimitIsEight(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 0, 0)
	candidate := s.user("candidate")
	s.wallet(candidate, 0)
	profile, bank := s.confirmedProfile(candidate, 10)

	got, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.QuestionCount != 8 || len(s.snapshot(got.ID)) != 8 {
		t.Fatalf("question count = %d, want the default 8", got.QuestionCount)
	}
	if snap := s.snapshot(got.ID); snap[7] != bank[7] {
		t.Fatalf("last snapshot question = %s, want bank[7] %s", snap[7], bank[7])
	}
}

// TestRecruiterCannotStartPractice proves a non-Candidate role is refused before any row is read or
// written, so a Recruiter cannot open a practice session on a profile.
func TestRecruiterCannotStartPractice(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	recruiter := s.user("recruiter")
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 4)

	_, _, err := svc.Create(ctx, recruiter, "recruiter", practiceInput(profile))
	assertCode(t, err, apperror.CodeForbidden)
	if n := s.interviewRows(profile); n != 0 {
		t.Fatalf("interview rows = %d after a refused start, want 0", n)
	}
}
