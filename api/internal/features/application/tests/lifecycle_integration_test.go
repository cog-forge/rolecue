//go:build integration

// Lifecycle of a recruitment Application after its interview: the Recruiter's final decision (KAN-115),
// the interview deadline auto-reject (KAN-214), and the interview block of the Recruiter's Application
// Detail and of the Candidate's own Application (KAN-114). They need a migrated, disposable PostgreSQL
// database, like the other tests in this package.

package tests

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// lcAt returns the time at offset from now, truncated to the microsecond Postgres stores.
func lcAt(offset time.Duration) time.Time {
	return time.Now().Add(offset).Truncate(time.Microsecond)
}

func lcTime(value time.Time) *time.Time { return &value }

// lcStatus reads the stored status of an Application.
func lcStatus(s *seed, applicationID uuid.UUID) string {
	return s.stored(applicationID).status
}

// lcCompleteInterview marks an interview completed with a score, as the completion and the evaluation
// write them together.
func lcCompleteInterview(s *seed, interviewID uuid.UUID, score int32) {
	s.t.Helper()
	s.exec(`UPDATE public.interviews SET completed_at = now(), evaluation_status = 'completed', score = $2 WHERE id = $1`,
		interviewID, score)
}

// lcEligibleExpired adds an interview_eligible Application whose deadline has passed and returns it.
func lcEligibleExpired(s *seed, postingID uuid.UUID) uuid.UUID {
	appID := s.application(postingID, s.user("candidate"))
	setStatusAndDeadline(s, appID, "interview_eligible", lcTime(lcAt(-time.Hour)))
	return appID
}

// --- Final decision (KAN-115) ---

func TestFinalDecisionIsOwnerOnly(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, other, candidate := s.user("recruiter"), s.user("recruiter"), s.user("candidate")
	postingID := s.posting(owner, "open", slot(3))
	appID := s.application(postingID, candidate)
	s.setApplicationStatus(appID, "interviewed")

	_, err := svc.Decide(ctx, other, "recruiter", postingID, appID, domain.DecisionApprove, nil)
	assertCode(t, err, apperror.CodeApplicationNotFound)
	_, err = svc.Decide(ctx, owner, "recruiter", uuid.New(), appID, domain.DecisionApprove, nil)
	assertCode(t, err, apperror.CodeApplicationNotFound)
	_, err = svc.Decide(ctx, owner, "recruiter", postingID, uuid.New(), domain.DecisionApprove, nil)
	assertCode(t, err, apperror.CodeApplicationNotFound)
	_, err = svc.Decide(ctx, candidate, "candidate", postingID, appID, domain.DecisionApprove, nil)
	assertCode(t, err, apperror.CodeForbidden)
	if got := lcStatus(s, appID); got != "interviewed" {
		t.Fatalf("status = %s after refused decisions, want interviewed", got)
	}
}

func TestFinalDecisionNeedsAnInterviewedApplication(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	past := lcAt(-time.Hour)
	for _, status := range []string{"pending", "interview_eligible", "approved", "rejected"} {
		appID := s.application(postingID, s.user("candidate"))
		setStatusAndDeadline(s, appID, status, lcTime(past))
		_, err := svc.Decide(ctx, owner, "recruiter", postingID, appID, domain.DecisionApprove, nil)
		assertCode(t, err, apperror.CodeApplicationNotInterviewed)
		if got := lcStatus(s, appID); got != status {
			t.Fatalf("status = %s after a refused decision, want %s", got, status)
		}
	}
}

func TestFinalDecisionGateOpensAfterTheLatestDeadline(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	decided := s.application(postingID, s.user("candidate"))
	later := s.application(postingID, s.user("candidate"))
	setStatusAndDeadline(s, decided, "interviewed", lcTime(lcAt(-time.Hour)))
	// The decided Application's own window is over, but the posting waits for the latest deadline.
	lateDeadline := lcAt(2 * time.Hour)
	setStatusAndDeadline(s, later, "interview_eligible", lcTime(lateDeadline))

	_, err := svc.Decide(ctx, owner, "recruiter", postingID, decided, domain.DecisionApprove, nil)
	assertCode(t, err, apperror.CodeFinalReviewLocked)
	if got := lcStatus(s, decided); got != "interviewed" {
		t.Fatalf("status = %s after a locked decision, want interviewed", got)
	}
	detail, err := svc.GetDetail(ctx, owner, "recruiter", postingID, decided)
	if err != nil {
		t.Fatal(err)
	}
	if detail.FinalReview.OpensAt == nil || !detail.FinalReview.OpensAt.Equal(lateDeadline) || detail.FinalReview.CanDecide {
		t.Fatalf("FinalReview = %+v, want opens_at at the latest deadline and can_decide false", detail.FinalReview)
	}

	// Once the latest window closes the gate opens for every Application of the posting.
	setStatusAndDeadline(s, later, "interview_eligible", lcTime(lcAt(-time.Minute)))
	got, err := svc.Decide(ctx, owner, "recruiter", postingID, decided, domain.DecisionApprove, nil)
	if err != nil {
		t.Fatalf("Decide() after the gate opened: %v", err)
	}
	if got.Status != domain.StatusApproved || lcStatus(s, decided) != "approved" {
		t.Fatalf("Decide() = status %s, stored %s, want approved", got.Status, lcStatus(s, decided))
	}
}

func TestFinalDecisionStoresApproveAndRejectWithTrimmedReason(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	approved := s.application(postingID, s.user("candidate"))
	rejected := s.application(postingID, s.user("candidate"))
	setStatusAndDeadline(s, approved, "interviewed", lcTime(lcAt(-time.Hour)))
	setStatusAndDeadline(s, rejected, "interviewed", lcTime(lcAt(-time.Hour)))

	ignored := "  ignored on approve  "
	if _, err := svc.Decide(ctx, owner, "recruiter", postingID, approved, domain.DecisionApprove, &ignored); err != nil {
		t.Fatal(err)
	}
	if row := s.stored(approved); row.status != "approved" || row.reason != nil {
		t.Fatalf("approved Application = %+v, want approved without a reason", row)
	}

	tooLong := strings.Repeat("r", domain.MaxRejectReasonLength+1)
	_, err := svc.Decide(ctx, owner, "recruiter", postingID, rejected, domain.DecisionReject, &tooLong)
	assertCode(t, err, apperror.CodeValidation)
	if got := lcStatus(s, rejected); got != "interviewed" {
		t.Fatalf("status = %s after an invalid reason, want interviewed", got)
	}

	reason := "  not a fit for the role \n"
	if _, err := svc.Decide(ctx, owner, "recruiter", postingID, rejected, domain.DecisionReject, &reason); err != nil {
		t.Fatal(err)
	}
	row := s.stored(rejected)
	if row.status != "rejected" || row.reason == nil || *row.reason != "not a fit for the role" {
		t.Fatalf("rejected Application = %+v, want rejected with the trimmed reason", row)
	}
}

func TestFinalDecisionRepeatIsRefused(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	appID := s.application(postingID, s.user("candidate"))
	setStatusAndDeadline(s, appID, "interviewed", lcTime(lcAt(-time.Hour)))
	if _, err := svc.Decide(ctx, owner, "recruiter", postingID, appID, domain.DecisionApprove, nil); err != nil {
		t.Fatal(err)
	}

	for _, decision := range []domain.Decision{domain.DecisionApprove, domain.DecisionReject} {
		reason := "changed my mind"
		_, err := svc.Decide(ctx, owner, "recruiter", postingID, appID, decision, &reason)
		assertCode(t, err, apperror.CodeApplicationNotInterviewed)
	}
	if row := s.stored(appID); row.status != "approved" || row.reason != nil {
		t.Fatalf("Application = %+v after repeated decisions, want the first approval kept", row)
	}
}

func TestFinalDecisionClosesPostingAndRefundsOnce(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 100, 3)
	first := s.application(postingID, s.user("candidate"))
	second := s.application(postingID, s.user("candidate"))
	third := s.application(postingID, s.user("candidate"))
	approve(t, ctx, svc, owner, postingID, first)
	approve(t, ctx, svc, owner, postingID, second)
	if _, err := svc.RejectCV(ctx, owner, "recruiter", postingID, third, nil); err != nil {
		t.Fatal(err)
	}
	s.interview(first)
	if _, err := svc.CloseIntake(ctx, owner, "recruiter", postingID); err != nil {
		t.Fatal(err)
	}
	setStatusAndDeadline(s, first, "interviewed", lcTime(lcAt(-time.Hour)))
	setStatusAndDeadline(s, second, "interviewed", lcTime(lcAt(-time.Hour)))

	// The first decision leaves the second Application live, so the posting stays intake_closed and
	// nothing is refunded yet.
	if _, err := svc.Decide(ctx, owner, "recruiter", postingID, first, domain.DecisionApprove, nil); err != nil {
		t.Fatal(err)
	}
	if status, _ := s.postingState(postingID); status != "intake_closed" {
		t.Fatalf("posting status = %s with a live Application, want intake_closed", status)
	}
	if n := len(s.postingRows(postingID)); n != 1 {
		t.Fatalf("coin rows = %d before the last decision, want only the funding debit", n)
	}

	// The last live Application becomes terminal: the posting closes and the unused slots are refunded
	// (3 funded, 1 consumed by the first interview, so 2 back).
	if _, err := svc.Decide(ctx, owner, "recruiter", postingID, second, domain.DecisionReject, nil); err != nil {
		t.Fatal(err)
	}
	if status, _ := s.postingState(postingID); status != "closed" {
		t.Fatalf("posting status = %s after the last decision, want closed", status)
	}
	rows := s.postingRows(postingID)
	if len(rows) != 2 || !hasRow(rows, "system", owner.String(), 2) {
		t.Fatalf("coin rows = %+v, want the debit and one refund of 2", rows)
	}
	if b := s.balance(owner); b != 99 {
		t.Fatalf("balance = %d, want 99", b)
	}
	got, err := svc.GetCapacity(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PostingStatus != "closed" || got.Refunded == nil || *got.Refunded != 2 {
		t.Fatalf("GetCapacity() = %+v, want closed with 2 refunded", got)
	}

	// A repeated decision is refused and must not refund again.
	_, err = svc.Decide(ctx, owner, "recruiter", postingID, second, domain.DecisionReject, nil)
	assertCode(t, err, apperror.CodeApplicationNotInterviewed)
	if n := len(s.postingRows(postingID)); n != 2 {
		t.Fatalf("coin rows = %d after a repeated decision, want 2", n)
	}
	if b := s.balance(owner); b != 99 {
		t.Fatalf("balance = %d after a repeated decision, want 99", b)
	}
}

// --- Auto-reject at the interview deadline (KAN-214) ---

func TestAutoRejectRejectsExpiredWithoutACompletedInterview(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(5))

	noInterview := lcEligibleExpired(s, postingID)
	started := lcEligibleExpired(s, postingID)
	s.interview(started) // started, never completed: the Candidate did not finish in time
	completed := lcEligibleExpired(s, postingID)
	lcCompleteInterview(s, s.interview(completed), 70)
	future := s.application(postingID, s.user("candidate"))
	setStatusAndDeadline(s, future, "interview_eligible", lcTime(lcAt(time.Hour)))
	interviewed := s.application(postingID, s.user("candidate"))
	setStatusAndDeadline(s, interviewed, "interviewed", lcTime(lcAt(-time.Hour)))

	rejected, err := svc.AutoRejectExpired(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if rejected < 2 {
		t.Fatalf("AutoRejectExpired() = %d, want at least the two expired Applications", rejected)
	}
	for _, id := range []uuid.UUID{noInterview, started} {
		row := s.stored(id)
		if row.status != "rejected" || row.reason == nil || *row.reason != domain.ExpiryRejectReason {
			t.Fatalf("expired Application = %+v, want rejected with %q", row, domain.ExpiryRejectReason)
		}
	}
	if row := s.stored(completed); row.status != "interview_eligible" || row.reason != nil {
		t.Fatalf("completed-interview Application = %+v, want it untouched", row)
	}
	if row := s.stored(future); row.status != "interview_eligible" || row.reason != nil {
		t.Fatalf("future-deadline Application = %+v, want it untouched", row)
	}
	if row := s.stored(interviewed); row.status != "interviewed" || row.reason != nil {
		t.Fatalf("interviewed Application = %+v, want it untouched", row)
	}
}

func TestAutoRejectDeadlineBoundaryIsStrict(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(5))
	deadline := lcAt(time.Hour)
	appID := s.application(postingID, s.user("candidate"))
	setStatusAndDeadline(s, appID, "interview_eligible", lcTime(deadline))

	for _, now := range []time.Time{deadline.Add(-time.Hour), deadline} {
		if _, err := svc.AutoRejectExpired(ctx, now); err != nil {
			t.Fatal(err)
		}
		if got := lcStatus(s, appID); got != "interview_eligible" {
			t.Fatalf("status = %s at %v, want interview_eligible before and at the deadline", got, now)
		}
	}
	if _, err := svc.AutoRejectExpired(ctx, deadline.Add(time.Microsecond)); err != nil {
		t.Fatal(err)
	}
	if row := s.stored(appID); row.status != "rejected" {
		t.Fatalf("Application = %+v after the deadline, want rejected", row)
	}
}

func TestAutoRejectSecondRunIsNoOp(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(5))
	appID := lcEligibleExpired(s, postingID)

	if _, err := svc.AutoRejectExpired(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	before := updatedAt(t, s, appID)
	rejected, err := svc.AutoRejectExpired(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if rejected != 0 {
		t.Fatalf("second AutoRejectExpired() = %d, want 0", rejected)
	}
	if !updatedAt(t, s, appID).Equal(before) {
		t.Fatal("a second run changed an already rejected Application")
	}
}

func TestAutoRejectConcurrentRunsRejectEachOnce(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(5))
	expired := []uuid.UUID{lcEligibleExpired(s, postingID), lcEligibleExpired(s, postingID)}

	now := time.Now()
	const runs = 2
	counts := make([]int, runs)
	errs := make([]error, runs)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < runs; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			counts[i], errs[i] = svc.AutoRejectExpired(ctx, now)
		}(i)
	}
	close(start)
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if total := counts[0] + counts[1]; total != len(expired) {
		t.Fatalf("concurrent runs rejected %d (%v), want each of the %d Applications once", total, counts, len(expired))
	}
	for _, id := range expired {
		if row := s.stored(id); row.status != "rejected" || row.reason == nil || *row.reason != domain.ExpiryRejectReason {
			t.Fatalf("Application = %+v, want rejected once with the expiry reason", row)
		}
	}
}

func TestAutoRejectClosesPostingAndRefundsOnce(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 100, 2)
	appID := s.application(postingID, s.user("candidate"))
	approve(t, ctx, svc, owner, postingID, appID)
	if _, err := svc.CloseIntake(ctx, owner, "recruiter", postingID); err != nil {
		t.Fatal(err)
	}
	setStatusAndDeadline(s, appID, "interview_eligible", lcTime(lcAt(-time.Hour)))

	if _, err := svc.AutoRejectExpired(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if row := s.stored(appID); row.status != "rejected" {
		t.Fatalf("Application = %+v, want rejected", row)
	}
	if status, _ := s.postingState(postingID); status != "closed" {
		t.Fatalf("posting status = %s after the last live Application expired, want closed", status)
	}
	rows := s.postingRows(postingID)
	if len(rows) != 2 || !hasRow(rows, "system", owner.String(), 2) {
		t.Fatalf("coin rows = %+v, want the debit and one refund of 2", rows)
	}
	if b := s.balance(owner); b != 100 {
		t.Fatalf("balance = %d, want 100 (2 funded, 2 refunded)", b)
	}

	if rejected, err := svc.AutoRejectExpired(ctx, time.Now()); err != nil || rejected != 0 {
		t.Fatalf("second AutoRejectExpired() = %d, %v, want 0", rejected, err)
	}
	if n := len(s.postingRows(postingID)); n != 2 || s.balance(owner) != 100 {
		t.Fatalf("coin rows = %d and balance = %d after a second run, want no second refund", n, s.balance(owner))
	}
}

// --- Interview block of the Recruiter's detail and the Candidate's own Application (KAN-114) ---

func TestRecruiterDetailShowsInterviewAndFinalReview(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	latest := lcAt(2 * time.Hour)
	withoutInterview := s.application(postingID, s.user("candidate"))
	setStatusAndDeadline(s, withoutInterview, "interview_eligible", lcTime(latest))
	interviewed := s.application(postingID, s.user("candidate"))
	setStatusAndDeadline(s, interviewed, "interviewed", lcTime(lcAt(-time.Hour)))
	interviewID := s.interview(interviewed)

	detail, err := svc.GetDetail(ctx, owner, "recruiter", postingID, withoutInterview)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Interview != nil {
		t.Fatalf("Interview = %+v for an Application without a session, want nil", detail.Interview)
	}
	if detail.FinalReview.OpensAt == nil || !detail.FinalReview.OpensAt.Equal(latest) || detail.FinalReview.CanDecide {
		t.Fatalf("FinalReview = %+v, want opens_at at the latest deadline and can_decide false", detail.FinalReview)
	}

	detail, err = svc.GetDetail(ctx, owner, "recruiter", postingID, interviewed)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Interview == nil || detail.Interview.ID != interviewID || detail.Interview.Status != "in_progress" ||
		detail.Interview.CompletedAt != nil || detail.Interview.Score != nil || detail.Interview.RecordStatus != "none" {
		t.Fatalf("Interview = %+v, want an in-progress session with no score and no recording", detail.Interview)
	}
	if detail.FinalReview.CanDecide {
		t.Fatal("can_decide is true while the posting still waits for a later deadline")
	}

	// Completed but not yet evaluated: the status is pending and the score stays null.
	s.exec(`UPDATE public.interviews SET completed_at = now() WHERE id = $1`, interviewID)
	setStatusAndDeadline(s, withoutInterview, "interview_eligible", lcTime(lcAt(-time.Minute)))
	detail, err = svc.GetDetail(ctx, owner, "recruiter", postingID, interviewed)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Interview == nil || detail.Interview.Status != "pending" || detail.Interview.Score != nil ||
		detail.Interview.CompletedAt == nil {
		t.Fatalf("Interview = %+v after completion, want completed with its evaluation pending", detail.Interview)
	}
	if !detail.FinalReview.CanDecide {
		t.Fatalf("FinalReview = %+v, want can_decide once every deadline has passed", detail.FinalReview)
	}
}

func TestCandidateApplicationInterviewBlockHasOnlyTheScore(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, candidate := s.user("recruiter"), s.user("candidate")
	postingID := s.posting(owner, "open", slot(3))
	appID := s.application(postingID, candidate)
	setStatusAndDeadline(s, appID, "interviewed", lcTime(lcAt(-time.Hour)))
	interviewID := s.interview(appID)
	lcCompleteInterview(s, interviewID, 81)

	got, err := svc.GetMine(ctx, candidate, "candidate", appID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Interview == nil || got.Interview.ID != interviewID || got.Interview.Status != "completed" ||
		got.Interview.Score == nil || *got.Interview.Score != 81 || got.Interview.CompletedAt == nil {
		t.Fatalf("Interview = %+v, want the completed interview with its score", got.Interview)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(marshalJSON(t, got)), &body); err != nil {
		t.Fatal(err)
	}
	var block map[string]json.RawMessage
	if err := json.Unmarshal(body["interview"], &block); err != nil {
		t.Fatalf("interview block = %s, want an object", body["interview"])
	}
	keys := make([]string, 0, len(block))
	for key := range block {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "completed_at,id,score,status" {
		t.Fatalf("interview block keys = %v, want exactly id, completed_at, status, score", keys)
	}

	other := s.user("candidate")
	noSession := s.application(postingID, other)
	empty, err := svc.GetMine(ctx, other, "candidate", noSession)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Interview != nil || !strings.Contains(marshalJSON(t, empty), `"interview":null`) {
		t.Fatalf("interview = %+v for an Application without a session, want null", empty.Interview)
	}
	if _, err := svc.GetMine(ctx, owner, "recruiter", appID); err == nil {
		t.Fatal("a recruiter read a Candidate's own Application")
	}
}
