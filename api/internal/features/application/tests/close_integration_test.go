//go:build integration

package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/features/application/repository"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// storedPostingStatus reads a posting's status as stored.
func storedPostingStatus(t *testing.T, s *seed, postingID uuid.UUID) string {
	t.Helper()
	var status string
	if err := s.pool.QueryRow(s.ctx, `SELECT status::text FROM public.job_postings WHERE id = $1`, postingID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

// countApplications counts the Applications of a posting in one status.
func countApplications(t *testing.T, s *seed, postingID uuid.UUID, status string) int {
	t.Helper()
	var n int
	if err := s.pool.QueryRow(s.ctx,
		`SELECT count(*) FROM public.applications WHERE job_posting_id = $1 AND status::text = $2`,
		postingID, status).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// updatedAt reads an Application's updated_at, to show that a call changed no row.
func updatedAt(t *testing.T, s *seed, applicationID uuid.UUID) time.Time {
	t.Helper()
	var at time.Time
	if err := s.pool.QueryRow(s.ctx, `SELECT updated_at FROM public.applications WHERE id = $1`, applicationID).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

// transactionRows counts the wallet transactions. Close Intake and approvals must add none.
func transactionRows(t *testing.T, pool *pgxpool.Pool, ctx context.Context) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.transactions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCloseIntakeRejectsPendingAndKeepsEligible(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(5))
	pending1 := s.application(postingID, s.user("candidate"))
	pending2 := s.application(postingID, s.user("candidate"))
	eligible := s.application(postingID, s.user("candidate"))
	if _, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, eligible); err != nil {
		t.Fatal(err)
	}

	closure, err := svc.CloseIntake(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if closure.JobPostingID != postingID || closure.Status != "intake_closed" || closure.RejectedCount != 2 {
		t.Fatalf("CloseIntake() = %+v, want intake_closed with 2 rejections", closure)
	}
	if got := storedPostingStatus(t, s, postingID); got != "intake_closed" {
		t.Fatalf("posting status = %s, want intake_closed", got)
	}
	for _, id := range []uuid.UUID{pending1, pending2} {
		row := s.stored(id)
		if row.status != "rejected" || row.reason == nil || *row.reason != domain.IntakeClosedRejectReason || row.deadline != nil {
			t.Fatalf("pending Application = %+v, want rejected with the intake-closed reason", row)
		}
	}
	if row := s.stored(eligible); row.status != "interview_eligible" || row.deadline == nil {
		t.Fatalf("eligible Application = %+v, want it still eligible", row)
	}
}

func TestCloseIntakeRepeatChangesNothing(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(5))
	pending := s.application(postingID, s.user("candidate"))
	eligible := s.application(postingID, s.user("candidate"))
	if _, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, eligible); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CloseIntake(ctx, owner, "recruiter", postingID); err != nil {
		t.Fatal(err)
	}
	pendingAt, eligibleAt := updatedAt(t, s, pending), updatedAt(t, s, eligible)

	closure, err := svc.CloseIntake(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if closure.Status != "intake_closed" || closure.RejectedCount != 0 {
		t.Fatalf("second CloseIntake() = %+v, want intake_closed with 0 rejections", closure)
	}
	if !updatedAt(t, s, pending).Equal(pendingAt) || !updatedAt(t, s, eligible).Equal(eligibleAt) {
		t.Fatal("a repeat close changed an Application row")
	}
}

func TestCloseIntakeRefusesOtherPostingStatuses(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	for _, status := range []string{"pending", "rejected"} {
		postingID := s.posting(owner, status, slot(3))
		appID := s.application(postingID, s.user("candidate"))
		_, err := svc.CloseIntake(ctx, owner, "recruiter", postingID)
		assertCode(t, err, apperror.CodeIntakeNotOpen)
		if got := storedPostingStatus(t, s, postingID); got != status {
			t.Fatalf("%s posting became %s", status, got)
		}
		if row := s.stored(appID); row.status != "pending" {
			t.Fatalf("%s posting: Application = %+v, want it untouched", status, row)
		}
	}
}

func TestCloseIntakeIsOwnerScoped(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, other := s.user("recruiter"), s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	appID := s.application(postingID, s.user("candidate"))

	_, err := svc.CloseIntake(ctx, other, "recruiter", postingID)
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	_, err = svc.CloseIntake(ctx, owner, "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	_, err = svc.CloseIntake(ctx, owner, "candidate", postingID)
	assertCode(t, err, apperror.CodeForbidden)

	if got := storedPostingStatus(t, s, postingID); got != "open" {
		t.Fatalf("posting status = %s after refused closes, want open", got)
	}
	if row := s.stored(appID); row.status != "pending" {
		t.Fatalf("Application = %+v after refused closes, want pending", row)
	}
}

func TestCloseWithNoApplicationsClosesTerminally(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))

	closure, err := svc.CloseIntake(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if closure.Status != "closed" || closure.RejectedCount != 0 {
		t.Fatalf("CloseIntake() = %+v, want closed with 0 rejections", closure)
	}
	if got := storedPostingStatus(t, s, postingID); got != "closed" {
		t.Fatalf("posting status = %s, want closed (no live Application, so nothing is left to wait for)", got)
	}
}

func TestCloseIntakeRetryAfterTerminalCloseSucceeds(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	appID := s.application(postingID, s.user("candidate"))

	first, err := svc.CloseIntake(ctx, owner, "recruiter", postingID)
	if err != nil || first.Status != "closed" || first.RejectedCount != 1 {
		t.Fatalf("first CloseIntake() = %+v, %v, want closed with 1 rejection", first, err)
	}
	before := s.stored(appID)

	retry, err := svc.CloseIntake(ctx, owner, "recruiter", postingID)
	if err != nil || retry.Status != "closed" || retry.RejectedCount != 0 {
		t.Fatalf("retry CloseIntake() = %+v, %v, want closed with 0 rejections and no error", retry, err)
	}
	after := s.stored(appID)
	if after.status != before.status || after.reason == nil || before.reason == nil || *after.reason != *before.reason {
		t.Fatalf("retry changed the Application: %+v -> %+v", before, after)
	}
}

func TestCloseWithOnlyPendingApplicationsClosesTerminally(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	s.application(postingID, s.user("candidate"))
	s.application(postingID, s.user("candidate"))

	closure, err := svc.CloseIntake(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if closure.Status != "closed" || closure.RejectedCount != 2 {
		t.Fatalf("CloseIntake() = %+v, want closed with 2 rejections", closure)
	}
	if got := storedPostingStatus(t, s, postingID); got != "closed" {
		t.Fatalf("posting status = %s, want closed once every Application is rejected", got)
	}
}

func TestEvaluateTerminalCloseRules(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	owner := s.user("recruiter")

	// An open posting is not evaluated.
	openPosting := s.posting(owner, "open", slot(3))
	s.application(openPosting, s.user("candidate"))
	if got, err := repo.EvaluateTerminalClose(ctx, openPosting); err != nil || got != "open" {
		t.Fatalf("EvaluateTerminalClose(open) = %q, %v, want open", got, err)
	}
	if got := storedPostingStatus(t, s, openPosting); got != "open" {
		t.Fatalf("open posting became %s", got)
	}

	// An intake_closed posting with a live interview_eligible Application stays intake_closed.
	live := s.posting(owner, "intake_closed", slot(3))
	liveApp := s.application(live, s.user("candidate"))
	s.exec(`UPDATE public.applications SET status = 'interview_eligible', interview_deadline = now() + interval '1 day' WHERE id = $1`, liveApp)
	if got, err := repo.EvaluateTerminalClose(ctx, live); err != nil || got != "intake_closed" {
		t.Fatalf("EvaluateTerminalClose(live) = %q, %v, want intake_closed", got, err)
	}

	// Only terminal Applications: the posting closes, and a repeat changes nothing.
	done := s.posting(owner, "intake_closed", slot(3))
	approved := s.application(done, s.user("candidate"))
	rejected := s.application(done, s.user("candidate"))
	s.exec(`UPDATE public.applications SET status = 'approved' WHERE id = $1`, approved)
	s.exec(`UPDATE public.applications SET status = 'rejected' WHERE id = $1`, rejected)
	if got, err := repo.EvaluateTerminalClose(ctx, done); err != nil || got != "closed" {
		t.Fatalf("EvaluateTerminalClose(terminal) = %q, %v, want closed", got, err)
	}
	if got, err := repo.EvaluateTerminalClose(ctx, done); err != nil || got != "closed" {
		t.Fatalf("second EvaluateTerminalClose(closed) = %q, %v, want closed", got, err)
	}
	if got := storedPostingStatus(t, s, done); got != "closed" {
		t.Fatalf("posting status = %s, want closed", got)
	}

	// An intake_closed posting with no Application has nothing left to wait for, so it closes.
	empty := s.posting(owner, "intake_closed", slot(3))
	if got, err := repo.EvaluateTerminalClose(ctx, empty); err != nil || got != "closed" {
		t.Fatalf("EvaluateTerminalClose(empty) = %q, %v, want closed", got, err)
	}

	_, err := repo.EvaluateTerminalClose(ctx, uuid.New())
	assertCode(t, err, apperror.CodeJobPostingNotFound)
}

// Close Intake and approvals write no transaction rows, except the one refund row of the first
// posting: it is funded with 3 slots, none consumed, and closes terminally, so the 3 coins go back.
// The auto-closing approval leaves a live Application and so writes nothing.
func TestIntakeChangesWriteOnlyTheRefund(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	before := transactionRows(t, pool, ctx)

	closing := s.posting(owner, "open", slot(3))
	s.application(closing, s.user("candidate"))
	if _, err := svc.CloseIntake(ctx, owner, "recruiter", closing); err != nil {
		t.Fatal(err)
	}
	auto := s.posting(owner, "open", slot(1))
	first := s.application(auto, s.user("candidate"))
	if _, err := svc.ApproveCV(ctx, owner, "recruiter", auto, first); err != nil {
		t.Fatal(err)
	}

	if after := transactionRows(t, pool, ctx); after != before+1 {
		t.Fatalf("transactions rows went from %d to %d, want exactly one refund row", before, after)
	}
}

func TestApprovalOfTheLastSlotClosesIntake(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(2))
	first := s.application(postingID, s.user("candidate"))
	second := s.application(postingID, s.user("candidate"))
	third := s.application(postingID, s.user("candidate"))

	got, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, first)
	if err != nil {
		t.Fatal(err)
	}
	if got.JobPostingStatus != "open" || storedPostingStatus(t, s, postingID) != "open" {
		t.Fatalf("first of two slots: job_posting_status = %s, want open", got.JobPostingStatus)
	}
	if row := s.stored(third); row.status != "pending" {
		t.Fatalf("third Application = %+v before the limit, want pending", row)
	}

	got, err = svc.ApproveCV(ctx, owner, "recruiter", postingID, second)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusInterviewEligible || got.JobPostingStatus != "intake_closed" {
		t.Fatalf("last slot = %+v, want interview_eligible on an intake_closed posting", got)
	}
	if status := storedPostingStatus(t, s, postingID); status != "intake_closed" {
		t.Fatalf("posting status = %s, want intake_closed", status)
	}
	for _, id := range []uuid.UUID{first, second} {
		if row := s.stored(id); row.status != "interview_eligible" || row.deadline == nil {
			t.Fatalf("approved Application = %+v, want it still eligible", row)
		}
	}
	row := s.stored(third)
	if row.status != "rejected" || row.reason == nil || *row.reason != domain.IntakeClosedRejectReason {
		t.Fatalf("remaining pending Application = %+v, want rejected with the intake-closed reason", row)
	}
	if _, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, third); err == nil {
		t.Fatal("ApproveCV on the rejected Application succeeded")
	} else {
		assertCode(t, err, apperror.CodeApplicationNotPending)
	}
}

func TestApprovalOfTheOnlySlotClosesIntakeAtOnce(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(1))
	only := s.application(postingID, s.user("candidate"))
	late := s.application(postingID, s.user("candidate"))

	got, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, only)
	if err != nil {
		t.Fatal(err)
	}
	if got.JobPostingStatus != "intake_closed" || storedPostingStatus(t, s, postingID) != "intake_closed" {
		t.Fatalf("slot 1 approval: job_posting_status = %s, want intake_closed", got.JobPostingStatus)
	}
	if row := s.stored(late); row.status != "rejected" || row.reason == nil || *row.reason != domain.IntakeClosedRejectReason {
		t.Fatalf("late Application = %+v, want rejected with the intake-closed reason", row)
	}
}

func TestRejectedAfterApprovalStillCountsTowardTheSlot(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(2))
	missed := s.application(postingID, s.user("candidate"))
	next := s.application(postingID, s.user("candidate"))
	late := s.application(postingID, s.user("candidate"))
	if _, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, missed); err != nil {
		t.Fatal(err)
	}
	// The first approval later lapses: rejected, but it still used a slot.
	s.exec(`UPDATE public.applications SET status = 'rejected' WHERE id = $1`, missed)

	got, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, next)
	if err != nil {
		t.Fatal(err)
	}
	if got.JobPostingStatus != "intake_closed" {
		t.Fatalf("second approval: job_posting_status = %s, want intake_closed", got.JobPostingStatus)
	}
	if row := s.stored(late); row.status != "rejected" {
		t.Fatalf("late Application = %+v, want rejected by the close", row)
	}
}

func TestRejectCVNeverClosesIntake(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")

	// One slot free after one approval, then a rejection: intake stays open.
	postingID := s.posting(owner, "open", slot(2))
	approved := s.application(postingID, s.user("candidate"))
	rejected := s.application(postingID, s.user("candidate"))
	s.application(postingID, s.user("candidate"))
	if _, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, approved); err != nil {
		t.Fatal(err)
	}
	got, err := svc.RejectCV(ctx, owner, "recruiter", postingID, rejected, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.JobPostingStatus != "open" || storedPostingStatus(t, s, postingID) != "open" {
		t.Fatalf("RejectCV() job_posting_status = %s, want open", got.JobPostingStatus)
	}

	// Rejecting the only pending Application of a one-slot posting does not close it either.
	oneSlot := s.posting(owner, "open", slot(1))
	only := s.application(oneSlot, s.user("candidate"))
	got, err = svc.RejectCV(ctx, owner, "recruiter", oneSlot, only, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.JobPostingStatus != "open" || storedPostingStatus(t, s, oneSlot) != "open" {
		t.Fatalf("RejectCV() on a one-slot posting: job_posting_status = %s, want open", got.JobPostingStatus)
	}
}

func TestApprovalWithoutFundedSlotNeverCloses(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", nil)
	appID := s.application(postingID, s.user("candidate"))
	other := s.application(postingID, s.user("candidate"))

	_, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, appID)
	assertCode(t, err, apperror.CodeInterviewCapacityNotFunded)
	if got := storedPostingStatus(t, s, postingID); got != "open" {
		t.Fatalf("posting status = %s, want open", got)
	}
	if row := s.stored(other); row.status != "pending" {
		t.Fatalf("other Application = %+v, want pending", row)
	}
}

// Approval and Close Intake started together on one posting end in one of two consistent states:
// the approval came first (intake closed, one eligible Application), or the close came first (every
// Application rejected, the posting closed, and the approval refused as not pending).
func TestApprovalAndCloseIntakeRaceEndsConsistent(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")

	for round := 0; round < 5; round++ {
		postingID := s.posting(owner, "open", slot(2))
		apps := []uuid.UUID{
			s.application(postingID, s.user("candidate")),
			s.application(postingID, s.user("candidate")),
			s.application(postingID, s.user("candidate")),
		}

		var wg sync.WaitGroup
		start := make(chan struct{})
		var approveErr, closeErr error
		var closure domain.IntakeClosure
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, approveErr = svc.ApproveCV(ctx, owner, "recruiter", postingID, apps[0])
		}()
		go func() {
			defer wg.Done()
			<-start
			closure, closeErr = svc.CloseIntake(ctx, owner, "recruiter", postingID)
		}()
		close(start)
		wg.Wait()

		if closeErr != nil {
			t.Fatalf("round %d: CloseIntake() error = %v", round, closeErr)
		}
		if approveErr != nil && codeOf(approveErr) != apperror.CodeApplicationNotPending {
			t.Fatalf("round %d: ApproveCV() error = %v, want nil or APPLICATION_NOT_PENDING", round, approveErr)
		}

		status := storedPostingStatus(t, s, postingID)
		eligible := countApplications(t, s, postingID, "interview_eligible")
		if pending := countApplications(t, s, postingID, "pending"); pending != 0 {
			t.Fatalf("round %d: %d Applications still pending after the close", round, pending)
		}
		if approveErr == nil {
			if status != "intake_closed" || eligible != 1 || closure.Status != "intake_closed" || closure.RejectedCount != 2 {
				t.Fatalf("round %d: approval first but status=%s eligible=%d closure=%+v", round, status, eligible, closure)
			}
		} else {
			if status != "closed" || eligible != 0 || closure.Status != "closed" || closure.RejectedCount != 3 {
				t.Fatalf("round %d: close first but status=%s eligible=%d closure=%+v", round, status, eligible, closure)
			}
		}
	}
}
