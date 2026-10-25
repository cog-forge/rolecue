//go:build integration

package tests

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// recruitmentInput asks for the recruitment session of an Application.
func recruitmentInput(applicationID uuid.UUID) domain.CreateInput {
	return domain.CreateInput{Type: domain.TypeRecruitment, ApplicationID: &applicationID}
}

// slotOf returns a pointer to a funded interview slot count.
func slotOf(n int32) *int32 {
	return &n
}

// interviewsOfApplication counts the interview rows of an Application. The unique application_id makes
// the answer 0 or 1.
func (s *seed) interviewsOfApplication(applicationID uuid.UUID) int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM public.interviews WHERE application_id = $1`, applicationID)
}

// interviewsOfPosting counts the interviews that use a slot of a posting, the same rule the capacity
// check applies.
func (s *seed) interviewsOfPosting(postingID uuid.UUID) int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM public.interviews i
		JOIN public.applications a ON a.id = i.application_id WHERE a.job_posting_id = $1`, postingID)
}

// TestRecruitmentCreateForEligibleApplication starts the recruitment session of an interview_eligible
// Application within its deadline. It proves the Candidate is not charged, the session asks the bank in
// order from the Candidate's confirmed profile, and the recruitment session carries no turns.
func TestRecruitmentCreateForEligibleApplication(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 10)
	profile, _ := s.confirmedProfile(candidate, 1)
	_, posting, bank := s.openPosting(slotOf(2), 4)
	application := s.eligibleApplication(posting, candidate)

	got, created, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !created {
		t.Fatal("created = false, want true for a new recruitment session")
	}
	if got.Type != domain.TypeRecruitment || got.ApplicationID == nil || *got.ApplicationID != application {
		t.Fatalf("Create() = %+v, want a recruitment session of the Application", got)
	}
	if got.RoleProfileID != profile {
		t.Fatalf("role profile = %s, want the Candidate's confirmed profile %s", got.RoleProfileID, profile)
	}
	if got.QuestionCount != 3 || got.CurrentQuestion == nil || got.CurrentQuestion.Position != 1 {
		t.Fatalf("Create() = %+v, want 3 questions with position 1 current", got)
	}
	if got.Turns != nil {
		t.Fatalf("recruitment session Turns = %v, want nil", *got.Turns)
	}
	if got.CurrentQuestion.Content != s.contentOf(bank[0]) {
		t.Fatalf("current question = %q, want the first bank question of the posting", got.CurrentQuestion.Content)
	}
	snap := s.snapshot(got.ID)
	if len(snap) != 3 || snap[0] != bank[0] || snap[1] != bank[1] || snap[2] != bank[2] {
		t.Fatalf("snapshot = %v, want the first 3 bank questions of the posting", snap)
	}
	if b := s.balance(candidate); b != 10 {
		t.Fatalf("candidate balance = %d, want 10: a recruitment session is free", b)
	}
	if rows := s.coinRows(candidate); len(rows) != 0 {
		t.Fatalf("candidate coin rows = %+v, want none", rows)
	}
	if n := s.interviewsOfApplication(application); n != 1 {
		t.Fatalf("interview rows for the Application = %d, want 1", n)
	}
}

// TestRecruitmentRequiresConfirmedProfileWhenNoneGiven proves a Candidate with no confirmed role profile
// cannot start a recruitment session, and no row is written.
func TestRecruitmentRequiresConfirmedProfileWhenNoneGiven(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.roleProfile(candidate, "draft", 1)
	_, posting, _ := s.openPosting(slotOf(2), 4)
	application := s.eligibleApplication(posting, candidate)

	_, _, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	assertCode(t, err, apperror.CodeRoleProfileRequired)
	if n := s.interviewsOfApplication(application); n != 0 {
		t.Fatalf("interview rows = %d, want 0", n)
	}
}

// TestRecruitmentNotEligibleStatusIsRefused proves an Application the Recruiter has not moved to
// interview_eligible cannot start an interview.
func TestRecruitmentNotEligibleStatusIsRefused(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.confirmedProfile(candidate, 1)
	_, posting, _ := s.openPosting(slotOf(2), 4)
	deadline := time.Now().Add(24 * time.Hour)
	application := s.application(posting, candidate, "pending", &deadline)

	_, _, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	assertCode(t, err, apperror.CodeApplicationNotEligible)
	if n := s.interviewsOfApplication(application); n != 0 {
		t.Fatalf("interview rows = %d, want 0", n)
	}
}

// TestRecruitmentExpiredDeadlineIsRefused proves a session cannot start once the interview deadline has
// passed, even for an interview_eligible Application.
func TestRecruitmentExpiredDeadlineIsRefused(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.confirmedProfile(candidate, 1)
	_, posting, _ := s.openPosting(slotOf(2), 4)
	application := s.eligibleApplication(posting, candidate)
	s.setDeadlineInPast(application)

	_, _, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	assertCode(t, err, apperror.CodeApplicationNotEligible)
	if n := s.interviewsOfApplication(application); n != 0 {
		t.Fatalf("interview rows = %d after an expired deadline, want 0", n)
	}
}

// TestRecruitmentCapacityNotFundedIsRefused proves a posting whose interview capacity was never funded
// does not start interviews.
func TestRecruitmentCapacityNotFundedIsRefused(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.confirmedProfile(candidate, 1)
	_, posting, _ := s.openPosting(nil, 4)
	application := s.eligibleApplication(posting, candidate)

	_, _, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	assertCode(t, err, apperror.CodeInterviewCapacityNotFunded)
	if n := s.interviewsOfApplication(application); n != 0 {
		t.Fatalf("interview rows = %d, want 0", n)
	}
}

// TestRecruitmentFullCapacityIsRefused proves the second Candidate is refused when the single funded
// slot is already used by the first one.
func TestRecruitmentFullCapacityIsRefused(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	_, posting, _ := s.openPosting(slotOf(1), 4)

	first := s.user("candidate")
	s.confirmedProfile(first, 1)
	firstApp := s.eligibleApplication(posting, first)
	if _, created, err := svc.Create(ctx, first, "candidate", recruitmentInput(firstApp)); err != nil || !created {
		t.Fatalf("first Create() created=%v error=%v, want a created session", created, err)
	}

	second := s.user("candidate")
	s.confirmedProfile(second, 1)
	secondApp := s.eligibleApplication(posting, second)
	_, _, err := svc.Create(ctx, second, "candidate", recruitmentInput(secondApp))
	assertCode(t, err, apperror.CodeInterviewCapacityFull)
	if n := s.interviewsOfApplication(secondApp); n != 0 {
		t.Fatalf("interview rows for the refused Application = %d, want 0", n)
	}
}

// TestRecruitmentForeignApplicationIsHidden proves an Application of another Candidate is reported as
// the same error as an Application that does not exist.
func TestRecruitmentForeignApplicationIsHidden(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	owner := s.user("candidate")
	s.confirmedProfile(owner, 1)
	_, posting, _ := s.openPosting(slotOf(2), 4)
	foreign := s.eligibleApplication(posting, owner)

	intruder := s.user("candidate")
	s.confirmedProfile(intruder, 1)
	_, _, foreignErr := svc.Create(ctx, intruder, "candidate", recruitmentInput(foreign))
	_, _, missingErr := svc.Create(ctx, intruder, "candidate", recruitmentInput(uuid.New()))

	assertCode(t, foreignErr, apperror.CodeApplicationNotFound)
	if codeOf(missingErr) != codeOf(foreignErr) {
		t.Fatalf("missing Application code %s differs from foreign code %s", codeOf(missingErr), codeOf(foreignErr))
	}
	if n := s.interviewsOfApplication(foreign); n != 0 {
		t.Fatalf("interview rows on the foreign Application = %d, want 0", n)
	}
}

// TestRecruitmentRepeatReturnsSameSession proves a repeated start returns the existing session with no
// second row and no side effect, and that the repeat still works after the deadline has passed, because
// the session already exists.
func TestRecruitmentRepeatReturnsSameSession(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 10)
	s.confirmedProfile(candidate, 1)
	_, posting, _ := s.openPosting(slotOf(2), 4)
	application := s.eligibleApplication(posting, candidate)

	first, created, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	if err != nil || !created {
		t.Fatalf("first Create() created=%v error=%v", created, err)
	}
	again, created, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	if err != nil {
		t.Fatalf("repeat Create() error = %v", err)
	}
	if created || again.ID != first.ID {
		t.Fatalf("repeat = %s created %v, want the existing %s with created false", again.ID, created, first.ID)
	}
	s.setDeadlineInPast(application)
	late, created, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	if err != nil {
		t.Fatalf("repeat after the deadline Create() error = %v, want the existing session", err)
	}
	if created || late.ID != first.ID {
		t.Fatalf("repeat after the deadline = %s created %v, want the existing %s", late.ID, created, first.ID)
	}
	if n := s.interviewsOfApplication(application); n != 1 {
		t.Fatalf("interview rows = %d after repeats, want 1", n)
	}
	if b := s.balance(candidate); b != 10 {
		t.Fatalf("balance = %d after repeats, want 10", b)
	}
}

// TestConcurrentRecruitmentCreatesForSameApplicationYieldOneRow fires eight starts for one Application
// at once. Exactly one call creates the session, the others return it, and there is one interview row.
func TestConcurrentRecruitmentCreatesForSameApplicationYieldOneRow(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.confirmedProfile(candidate, 1)
	_, posting, _ := s.openPosting(slotOf(5), 4)
	application := s.eligibleApplication(posting, candidate)

	const n = 8
	results := runConcurrently(n, func(int) (domain.Session, bool, error) {
		return svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	})
	createdCount := 0
	var sessionID uuid.UUID
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("start %d error = %v, want every start to return the session", i, r.err)
		}
		if r.created {
			createdCount++
		}
		if i == 0 {
			sessionID = r.session.ID
		} else if r.session.ID != sessionID {
			t.Fatalf("start %d returned %s, want the same session %s", i, r.session.ID, sessionID)
		}
	}
	if createdCount != 1 {
		t.Fatalf("created by %d starts, want exactly 1", createdCount)
	}
	if rows := s.interviewsOfApplication(application); rows != 1 {
		t.Fatalf("interview rows = %d, want 1", rows)
	}
}

// TestConcurrentRecruitmentCreatesNeverExceedLastSlot fires twelve starts for twelve different eligible
// Applications on a posting with three funded slots. Exactly three sessions are created, the others are
// refused as full, and the posting never has more interviews than it funded.
func TestConcurrentRecruitmentCreatesNeverExceedLastSlot(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	_, posting, _ := s.openPosting(slotOf(3), 4)

	const candidates = 12
	type entry struct {
		candidate, application uuid.UUID
	}
	entries := make([]entry, candidates)
	for i := range entries {
		c := s.user("candidate")
		s.confirmedProfile(c, 1)
		entries[i] = entry{candidate: c, application: s.eligibleApplication(posting, c)}
	}

	results := runConcurrently(candidates, func(i int) (domain.Session, bool, error) {
		return svc.Create(ctx, entries[i].candidate, "candidate", recruitmentInput(entries[i].application))
	})
	created, full := 0, 0
	for i, r := range results {
		switch {
		case r.err == nil && r.created:
			created++
		case codeOf(r.err) == apperror.CodeInterviewCapacityFull:
			full++
		default:
			t.Fatalf("start %d: created=%v error=%v, want created or capacity full", i, r.created, r.err)
		}
	}
	if created != 3 || full != candidates-3 {
		t.Fatalf("created = %d full = %d, want 3 and %d", created, full, candidates-3)
	}
	if rows := s.interviewsOfPosting(posting); rows != 3 {
		t.Fatalf("interview rows on the posting = %d, want exactly the 3 funded slots", rows)
	}
}

// TestRecruitmentCreateDoesNotWriteWhenRefused proves a refused recruitment start leaves the Application
// status and the candidate wallet alone, because no charge exists for recruitment.
func TestRecruitmentCreateDoesNotWriteWhenRefused(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 7)
	s.confirmedProfile(candidate, 1)
	_, posting, _ := s.openPosting(nil, 4)
	application := s.eligibleApplication(posting, candidate)

	_, _, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	assertCode(t, err, apperror.CodeInterviewCapacityNotFunded)
	if got := s.applicationStatus(application); got != "interview_eligible" {
		t.Fatalf("application status = %s after a refused start, want interview_eligible", got)
	}
	if b := s.balance(candidate); b != 7 {
		t.Fatalf("balance = %d, want 7 unchanged", b)
	}
	if rows := s.coinRows(candidate); len(rows) != 0 {
		t.Fatalf("coin rows = %+v, want none", rows)
	}
}

// startResult is one concurrent start and its outcome.
type startResult struct {
	session domain.Session
	created bool
	err     error
}

// runConcurrently runs n calls at the same time, released together by a barrier, and returns their
// results in index order.
func runConcurrently(n int, call func(i int) (domain.Session, bool, error)) []startResult {
	results := make([]startResult, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			session, created, err := call(i)
			results[i] = startResult{session: session, created: created, err: err}
		}(i)
	}
	close(start)
	wg.Wait()
	return results
}

// recruitmentJSON marshals a session the way the handler would send it, for the redaction checks.
func recruitmentJSON(t *testing.T, session domain.Session) map[string]any {
	t.Helper()
	body, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestRecruitmentSessionJSONIsRedacted proves a recruitment session response has no turns key, no
// record fields and shows only the current question's text, never the rest of the bank.
func TestRecruitmentSessionJSONIsRedacted(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.confirmedProfile(candidate, 1)
	_, posting, bank := s.openPosting(slotOf(2), 4)
	application := s.eligibleApplication(posting, candidate)

	got, _, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	body := recruitmentJSON(t, got)
	for _, key := range []string{"turns", "record_path", "record_status", "core_question_ids", "questions", "upload_url"} {
		if _, ok := body[key]; ok {
			t.Fatalf("recruitment session JSON has key %q, want it absent", key)
		}
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range bank[1:] {
		if content := s.contentOf(id); strings.Contains(string(raw), content) {
			t.Fatalf("recruitment session JSON leaks a bank question that is not current: %q", content)
		}
	}
	if !strings.Contains(string(raw), s.contentOf(bank[0])) {
		t.Fatal("recruitment session JSON does not show the current question")
	}
}

// TestPracticeSessionJSONShowsEmptyTurns proves a practice session always carries a turns array, empty
// before any answer, so the client never has to handle a missing key.
func TestPracticeSessionJSONShowsEmptyTurns(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 5, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 4)

	got, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	body := recruitmentJSON(t, got)
	turns, ok := body["turns"].([]any)
	if !ok || len(turns) != 0 {
		t.Fatalf("practice turns = %#v, want an empty array", body["turns"])
	}
}
