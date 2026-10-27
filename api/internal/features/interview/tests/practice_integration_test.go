//go:build integration

package tests

import (
	"sync"
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

// assertSampleOf fails the test unless the snapshot holds different questions, all of the bank.
func assertSampleOf(t *testing.T, snap, bank []uuid.UUID) {
	t.Helper()
	inBank := make(map[uuid.UUID]bool, len(bank))
	for _, id := range bank {
		inBank[id] = true
	}
	seen := make(map[uuid.UUID]bool, len(snap))
	for _, id := range snap {
		if !inBank[id] || seen[id] {
			t.Fatalf("snapshot %v is not a sample of different bank questions %v", snap, bank)
		}
		seen[id] = true
	}
}

// questionRows counts the interview_questions rows of every session from a role profile.
func (s *seed) questionRows(profileID uuid.UUID) int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM public.interview_questions iq
		JOIN public.interviews i ON i.id = iq.interview_id WHERE i.role_profile_id = $1`, profileID)
}

// TestPracticeCreateSnapshotsQuestionsAndChargesOnce starts a practice session from a confirmed profile
// with four bank questions and a limit of three. It proves the session stores three different bank
// questions, that the first stored one is the current question, and that exactly one coin debit of the
// configured price was written.
func TestPracticeCreateSnapshotsQuestionsAndChargesOnce(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
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

	snap := s.snapshot(got.ID)
	if len(snap) != 3 {
		t.Fatalf("snapshot has %d questions, want 3", len(snap))
	}
	assertSampleOf(t, snap, bank)
	if got.CurrentQuestion == nil || got.CurrentQuestion.Position != 1 || got.CurrentQuestion.Content != s.contentOf(snap[0]) {
		t.Fatalf("current question = %+v, want position 1 with the first stored question", got.CurrentQuestion)
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
	svc := newService(t, pool, 3)
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
	svc := newService(t, pool, 3)
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
	svc := newService(t, pool, 3)
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
	svc := newService(t, pool, 3)
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

// TestPracticeStartResumesTheUnfinishedSessionForFree proves a second start from the same profile while
// the first session is unfinished (a double click, a retry) returns that session and charges nothing;
// once it is completed, the next start is a new session and a new charge.
func TestPracticeStartResumesTheUnfinishedSessionForFree(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 4)

	first, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	again, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil || created || again.ID != first.ID {
		t.Fatalf("second start = %s created %v error %v, want the unfinished %s without creating", again.ID, created, err, first.ID)
	}
	if n := s.interviewRows(profile); n != 1 {
		t.Fatalf("interview rows = %d, want 1", n)
	}
	if b := s.balance(candidate); b != 15 {
		t.Fatalf("balance = %d, want 15 after a single charge of 5", b)
	}

	if _, err := svc.Complete(ctx, candidate, "candidate", first.ID); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	next, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil || !created || next.ID == first.ID {
		t.Fatalf("start after completion = %s created %v error %v, want a new session", next.ID, created, err)
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
	svc := newService(t, pool, 0)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, bank := s.confirmedProfile(candidate, 10)

	got, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	snap := s.snapshot(got.ID)
	if got.QuestionCount != 8 || len(snap) != 8 {
		t.Fatalf("question count = %d, want the default 8", got.QuestionCount)
	}
	assertSampleOf(t, snap, bank)
}

// TestRecruiterCannotStartPractice proves a non-Candidate role is refused before any row is read or
// written, so a Recruiter cannot open a practice session on a profile.
func TestRecruiterCannotStartPractice(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
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

// TestConcurrentPracticeStartsChargeOnce fires six starts at once, with and without an Idempotency-Key.
// They all return one session and the Candidate is charged once.
func TestConcurrentPracticeStartsChargeOnce(t *testing.T) {
	for _, key := range []string{"", "retry-me"} {
		t.Run("key="+key, func(t *testing.T) {
			ctx, pool := openPool(t)
			s := newSeed(t, ctx, pool)
			svc := newService(t, pool, 3)
			candidate := s.user("candidate")
			s.wallet(candidate, 100)
			profile, _ := s.confirmedProfile(candidate, 3)
			input := practiceInput(profile)
			input.IdempotencyKey = key

			ids := make([]uuid.UUID, 6)
			var wg sync.WaitGroup
			for i := range ids {
				wg.Add(1)
				go func() {
					defer wg.Done()
					got, _, err := svc.Create(ctx, candidate, "candidate", input)
					if err != nil {
						t.Errorf("Create() error = %v", err)
						return
					}
					ids[i] = got.ID
				}()
			}
			wg.Wait()
			for _, id := range ids {
				if id != ids[0] {
					t.Fatalf("concurrent starts returned different sessions: %v", ids)
				}
			}
			if b := s.balance(candidate); b != 95 {
				t.Fatalf("balance = %d after concurrent starts, want 95 (charged once)", b)
			}
			if n := s.interviewRows(profile); n != 1 {
				t.Fatalf("interview rows = %d, want 1", n)
			}
		})
	}
}

// TestPracticeRefusesAnotherUsersAvatarAndInactiveVoice proves a practice start cannot use an avatar
// outside the Candidate's own inventory or a voice that is not active, and nothing is charged.
func TestPracticeRefusesAnotherUsersAvatarAndInactiveVoice(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	other := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 3)

	foreign := practiceInput(profile)
	avatar := s.avatarOf(other)
	foreign.AvatarID = &avatar
	_, _, err := svc.Create(ctx, candidate, "candidate", foreign)
	assertCode(t, err, apperror.CodeValidation)

	inactive := practiceInput(profile)
	voice := s.voice(false)
	inactive.VoiceID = &voice
	_, _, err = svc.Create(ctx, candidate, "candidate", inactive)
	assertCode(t, err, apperror.CodeValidation)

	own := practiceInput(profile)
	ownAvatar := s.avatarOf(candidate)
	activeVoice := s.voice(true)
	own.AvatarID, own.VoiceID = &ownAvatar, &activeVoice
	if _, created, err := svc.Create(ctx, candidate, "candidate", own); err != nil || !created {
		t.Fatalf("start with an own avatar and an active voice created=%v error=%v", created, err)
	}
	if b := s.balance(candidate); b != 15 {
		t.Fatalf("balance = %d, want 15 (only the valid start charged)", b)
	}
}

// TestPracticeSamplesQuestionsRandomlyFromTheBank starts six sessions one after the other (each completed
// before the next) on one bank of twelve questions with a limit of three. It proves every session asks
// three different questions of that bank, and that
// the sessions do not all ask the first three by id (Decision 14: questions are sampled at random). Six
// sessions all drawing the first three in order by chance is about one in 10^23.
func TestPracticeSamplesQuestionsRandomlyFromTheBank(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 100)
	profile, bank := s.confirmedProfile(candidate, 12)

	firstN := 0
	for i := 0; i < 6; i++ {
		got, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
		if err != nil || !created {
			t.Fatalf("Create() #%d created=%v error=%v", i+1, created, err)
		}
		snap := s.snapshot(got.ID)
		if len(snap) != 3 || got.QuestionCount != 3 {
			t.Fatalf("session #%d asks %d questions (count %d), want 3", i+1, len(snap), got.QuestionCount)
		}
		assertSampleOf(t, snap, bank)
		if snap[0] == bank[0] && snap[1] == bank[1] && snap[2] == bank[2] {
			firstN++
		}
		if _, err := svc.Complete(ctx, candidate, "candidate", got.ID); err != nil {
			t.Fatalf("Complete() #%d error = %v", i+1, err)
		}
	}
	if firstN == 6 {
		t.Fatal("every session asked the first three bank questions by id, want a random sample")
	}
}

// TestPracticeQuestionsStayStableAcrossResume proves the sampled questions are fixed when the session
// starts: Get, Resume and a repeated start return the same questions in the same order, before and after
// an answer.
func TestPracticeQuestionsStayStableAcrossResume(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 12)
	started, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	snap := s.snapshot(started.ID)

	assertCurrent := func(label string, got domain.Session, position int) {
		t.Helper()
		if got.ID != started.ID || got.CurrentQuestion == nil || got.CurrentQuestion.Position != int32(position) ||
			got.CurrentQuestion.Content != s.contentOf(snap[position-1]) {
			t.Fatalf("%s current question = %+v, want position %d of the snapshot", label, got.CurrentQuestion, position)
		}
	}
	for i := 0; i < 2; i++ {
		got, err := svc.Get(ctx, candidate, "candidate", started.ID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		assertCurrent("Get()", got, 1)
		got, err = svc.Resume(ctx, candidate, "candidate", started.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		assertCurrent("Resume()", got, 1)
	}
	again, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil || created {
		t.Fatalf("repeat Create() created=%v error=%v, want the unfinished session", created, err)
	}
	assertCurrent("repeat Create()", again, 1)

	if _, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, "an answer"); err != nil {
		t.Fatalf("AppendTurn() error = %v", err)
	}
	got, err := svc.Resume(ctx, candidate, "candidate", started.ID)
	if err != nil {
		t.Fatalf("Resume() after an answer error = %v", err)
	}
	assertCurrent("Resume() after an answer", got, 2)
	if turns := s.turnRowsOf(started.ID); len(turns) != 1 || turns[0].question != s.contentOf(snap[0]) {
		t.Fatalf("turns = %+v, want the first snapshot question answered", turns)
	}
	if after := s.snapshot(started.ID); len(after) != len(snap) || after[0] != snap[0] || after[1] != snap[1] || after[2] != snap[2] {
		t.Fatalf("snapshot changed from %v to %v", snap, after)
	}
}
