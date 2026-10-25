//go:build integration

// Package tests: interview capacity lifecycle (KAN-116, 118, 119, 120). Funding, refund and the
// approval rule run in SQL under the posting row lock, so only a real database proves them. Every
// test seeds its own Recruiter, Candidates and posting, and checks per-entity rows, never table
// totals, so the tests stay independent of each other.
package tests

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/features/application/repository"
	"github.com/cog-forge/rolecue/api/internal/features/application/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// submit sends a CV for the Candidate and checks the Application starts pending.
func submit(t *testing.T, ctx context.Context, svc *service.Service, candidateID, postingID uuid.UUID) uuid.UUID {
	t.Helper()
	got, err := svc.Apply(ctx, candidateID, "candidate", postingID, validCV)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got.Status != domain.StatusPending || got.JobPostingID != postingID {
		t.Fatalf("Apply() = %+v, want a pending Application on the posting", got)
	}
	return got.ID
}

// approve grants the CV approval as the Recruiter and fails the test on error.
func approve(t *testing.T, ctx context.Context, svc *service.Service, recruiterID, postingID, applicationID uuid.UUID) domain.Screening {
	t.Helper()
	got, err := svc.ApproveCV(ctx, recruiterID, "recruiter", postingID, applicationID)
	if err != nil {
		t.Fatalf("ApproveCV() error = %v", err)
	}
	return got
}

// fundedPosting returns a Recruiter with the balance and an open posting funded with slots.
func fundedPosting(t *testing.T, ctx context.Context, s *seed, svc *service.Service, balance, slots int32) (uuid.UUID, uuid.UUID) {
	t.Helper()
	owner := s.user("recruiter")
	s.wallet(owner, balance)
	postingID := s.posting(owner, "open", nil)
	if _, err := svc.FundCapacity(ctx, owner, "recruiter", postingID, slots); err != nil {
		t.Fatalf("FundCapacity(%d) error = %v", slots, err)
	}
	return owner, postingID
}

// --- Funding (R2, R3) ---

func TestCapacityFundingDebitsWalletAndSetsSlot(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	s.wallet(owner, 100)
	postingID := s.posting(owner, "open", nil)

	got, err := svc.FundCapacity(ctx, owner, "recruiter", postingID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Funded || got.InterviewSlot == nil || *got.InterviewSlot != 5 || got.PricePerSlot != 1 ||
		got.AvailableToApprove != 5 || got.Refundable != 5 || got.Refunded != nil {
		t.Fatalf("FundCapacity() = %+v", got)
	}
	if b := s.balance(owner); b != 95 {
		t.Fatalf("balance = %d, want 95", b)
	}
	rows := s.postingRows(postingID)
	if len(rows) != 1 || rows[0].amount != 5 || rows[0].from != owner.String() || rows[0].to != "system" ||
		rows[0].currency != "COIN" || rows[0].status != "success" {
		t.Fatalf("coin rows = %+v, want one COIN debit of 5 from the Recruiter to system", rows)
	}
	status, slot := s.postingState(postingID)
	if status != "open" || slot == nil || *slot != 5 {
		t.Fatalf("posting = %s slot %v, want open with slot 5", status, slot)
	}
}

func TestCapacityFundingRetryWithSameCountChargesNothing(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 100, 5)

	again, err := svc.FundCapacity(ctx, owner, "recruiter", postingID, 5)
	if err != nil {
		t.Fatalf("retry FundCapacity(5) error = %v, want the current state", err)
	}
	if !again.Funded || again.InterviewSlot == nil || *again.InterviewSlot != 5 {
		t.Fatalf("retry = %+v, want the funded state", again)
	}
	if b := s.balance(owner); b != 95 {
		t.Fatalf("balance = %d after retry, want 95", b)
	}
	if rows := s.postingRows(postingID); len(rows) != 1 {
		t.Fatalf("coin rows = %d after retry, want 1", len(rows))
	}
}

func TestCapacityFundingDifferentCountIsAlreadyFunded(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 100, 5)

	_, err := svc.FundCapacity(ctx, owner, "recruiter", postingID, 7)
	assertCode(t, err, apperror.CodeInterviewCapacityAlreadyFunded)
	if b := s.balance(owner); b != 95 {
		t.Fatalf("balance = %d, want 95 unchanged", b)
	}
	if status, slot := s.postingState(postingID); status != "open" || slot == nil || *slot != 5 {
		t.Fatalf("posting = %s slot %v, want slot still 5", status, slot)
	}
}

func TestCapacityFundingWithLowBalanceLeavesSlotNull(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	s.wallet(owner, 3)
	postingID := s.posting(owner, "open", nil)

	_, err := svc.FundCapacity(ctx, owner, "recruiter", postingID, 5)
	assertCode(t, err, apperror.CodeInsufficientBalance)
	if _, slot := s.postingState(postingID); slot != nil {
		t.Fatalf("interview_slot = %d, want NULL after a failed funding", *slot)
	}
	if b := s.balance(owner); b != 3 {
		t.Fatalf("balance = %d, want 3 unchanged", b)
	}
	if rows := s.postingRows(postingID); len(rows) != 0 {
		t.Fatalf("coin rows = %+v, want none", rows)
	}
}

func TestCapacityFundingWithoutWalletIsInsufficientBalance(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", nil)

	_, err := svc.FundCapacity(ctx, owner, "recruiter", postingID, 1)
	assertCode(t, err, apperror.CodeInsufficientBalance)
	if _, slot := s.postingState(postingID); slot != nil {
		t.Fatalf("interview_slot = %d, want NULL", *slot)
	}
	if n := s.walletRows(owner); n != 0 {
		t.Fatalf("wallet rows = %d, want funding not to create one", n)
	}
}

func TestCapacityConcurrentFundingChargesOnce(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	s.wallet(owner, 100)
	postingID := s.posting(owner, "open", nil)

	const attempts = 8
	errs := make([]error, attempts)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = svc.FundCapacity(ctx, owner, "recruiter", postingID, 5)
		}(i)
	}
	close(start)
	wg.Wait()

	// The posting lock serialises the calls: the first funds, the rest see the same count.
	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d error = %v, want every repeat with the same count to succeed", i, err)
		}
	}
	if b := s.balance(owner); b != 95 {
		t.Fatalf("balance = %d, want 95 (one purchase of 5)", b)
	}
	if rows := s.postingRows(postingID); len(rows) != 1 {
		t.Fatalf("coin rows = %d, want exactly one debit", len(rows))
	}
	if status, slot := s.postingState(postingID); status != "open" || slot == nil || *slot != 5 {
		t.Fatalf("posting = %s slot %v, want slot 5", status, slot)
	}
}

func TestCapacityFundingNeedsAnOpenPosting(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	s.wallet(owner, 100)

	for _, status := range []string{"pending", "intake_closed", "closed", "rejected"} {
		postingID := s.posting(owner, status, nil)
		_, err := svc.FundCapacity(ctx, owner, "recruiter", postingID, 5)
		assertCode(t, err, apperror.CodeIntakeNotOpen)
		if _, slot := s.postingState(postingID); slot != nil {
			t.Fatalf("%s posting: interview_slot = %d, want NULL", status, *slot)
		}
	}
	if b := s.balance(owner); b != 100 {
		t.Fatalf("balance = %d, want 100 unchanged", b)
	}
}

func TestCapacityFundingSlotBounds(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	s.wallet(owner, 10000)
	postingID := s.posting(owner, "open", nil)

	for _, slots := range []int32{0, -1, 10001} {
		_, err := svc.FundCapacity(ctx, owner, "recruiter", postingID, slots)
		assertCode(t, err, apperror.CodeValidation)
	}
	if b := s.balance(owner); b != 10000 {
		t.Fatalf("balance = %d after rejected counts, want 10000", b)
	}

	// The upper bound itself is valid: 10000 slots at one coin each use the whole balance.
	got, err := svc.FundCapacity(ctx, owner, "recruiter", postingID, 10000)
	if err != nil || got.InterviewSlot == nil || *got.InterviewSlot != 10000 {
		t.Fatalf("FundCapacity(10000) = %+v, %v", got, err)
	}
	if b := s.balance(owner); b != 0 {
		t.Fatalf("balance = %d, want 0", b)
	}
}

// --- State (R4) ---

func TestCapacityStateReadsAreOwnerOnly(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, other := s.user("recruiter"), s.user("recruiter")
	candidate := s.user("candidate")
	unfunded := s.posting(owner, "open", nil)

	got, err := svc.GetCapacity(ctx, owner, "recruiter", unfunded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Funded || got.InterviewSlot != nil || got.AvailableToApprove != 0 || got.Refundable != 0 ||
		got.Refunded != nil || got.PostingStatus != "open" || got.PricePerSlot != 1 {
		t.Fatalf("unfunded GetCapacity() = %+v", got)
	}

	_, funded := fundedPosting(t, ctx, s, svc, 10, 5)
	_, err = svc.GetCapacity(ctx, candidate, "candidate", funded)
	assertCode(t, err, apperror.CodeForbidden)
	_, err = svc.FundCapacity(ctx, candidate, "candidate", funded, 1)
	assertCode(t, err, apperror.CodeForbidden)
	_, err = svc.GetCapacity(ctx, other, "recruiter", funded)
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	_, err = svc.FundCapacity(ctx, other, "recruiter", funded, 1)
	assertCode(t, err, apperror.CodeJobPostingNotFound)
}

func TestCapacitySubmittedCVsNeverCountAsCapacity(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 10, 5)
	for i := 0; i < 50; i++ {
		s.application(postingID, s.user("candidate"))
	}

	got, err := svc.GetCapacity(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CVApprovals != 0 || got.AvailableToApprove != 5 || got.Refundable != 5 || got.Consumed != 0 {
		t.Fatalf("GetCapacity() = %+v, want 50 submissions to leave the 5 slots open", got)
	}
}

func TestCapacityStateCountsApprovalsAndInterviews(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 10, 5)
	first := s.application(postingID, s.user("candidate"))
	second := s.application(postingID, s.user("candidate"))
	s.application(postingID, s.user("candidate"))
	approve(t, ctx, svc, owner, postingID, first)
	approve(t, ctx, svc, owner, postingID, second)
	s.interview(first)

	got, err := svc.GetCapacity(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	// Two approvals take two slots; one interview row consumes one slot, so refundable is slot-1.
	if got.CVApprovals != 2 || got.Consumed != 1 || got.AvailableToApprove != 3 || got.Refundable != 4 {
		t.Fatalf("GetCapacity() = %+v, want approvals 2, consumed 1, available 3, refundable 4", got)
	}
}

func TestCapacityStateFloorsAtZero(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 1, 1)
	apps := []uuid.UUID{
		s.application(postingID, s.user("candidate")),
		s.application(postingID, s.user("candidate")),
		s.application(postingID, s.user("candidate")),
	}
	// Three approvals and three interviews against one slot: the counts exceed the slot, which the
	// state must report as zero open slots and zero refundable slots, never negative.
	s.exec(`UPDATE public.applications SET interview_deadline = now() + interval '1 day' WHERE job_posting_id = $1`, postingID)
	for _, id := range apps {
		s.interview(id)
	}

	got, err := svc.GetCapacity(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CVApprovals != 3 || got.Consumed != 3 || got.AvailableToApprove != 0 || got.Refundable != 0 {
		t.Fatalf("GetCapacity() = %+v, want counts 3 and 3 with zero available and refundable", got)
	}
}

// --- Refund (R5) ---

func TestCapacityRefundOnTerminalCloseCreditsUnusedSlots(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 100, 5)
	s.application(postingID, s.user("candidate"))

	closure, err := svc.CloseIntake(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if closure.Status != "closed" || closure.RejectedCount != 1 {
		t.Fatalf("CloseIntake() = %+v, want closed with one rejection", closure)
	}
	if b := s.balance(owner); b != 100 {
		t.Fatalf("balance = %d, want 100 after the refund", b)
	}
	rows := s.postingRows(postingID)
	if len(rows) != 2 || !hasRow(rows, "system", owner.String(), 5) {
		t.Fatalf("coin rows = %+v, want the debit and one refund of 5 from system", rows)
	}
	got, err := svc.GetCapacity(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PostingStatus != "closed" || got.Refunded == nil || *got.Refunded != 5 {
		t.Fatalf("GetCapacity() after close = %+v, want refunded 5", got)
	}
}

func TestCapacityRefundSubtractsConsumedInterviews(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 100, 5)
	first := s.application(postingID, s.user("candidate"))
	second := s.application(postingID, s.user("candidate"))
	approve(t, ctx, svc, owner, postingID, first)
	approve(t, ctx, svc, owner, postingID, second)
	s.interview(first)

	if _, err := svc.CloseIntake(ctx, owner, "recruiter", postingID); err != nil {
		t.Fatal(err)
	}
	// The outcome decides both Applications; the one that interviewed stays out of the refund.
	s.setApplicationStatus(first, "approved")
	s.setApplicationStatus(second, "rejected")
	status, err := repository.NewRepository(pool).EvaluateTerminalClose(ctx, postingID)
	if err != nil || status != "closed" {
		t.Fatalf("EvaluateTerminalClose() = %q, %v, want closed", status, err)
	}

	if !hasRow(s.postingRows(postingID), "system", owner.String(), 4) {
		t.Fatalf("coin rows = %+v, want a refund of 4 (5 funded, 1 consumed)", s.postingRows(postingID))
	}
	if b := s.balance(owner); b != 99 {
		t.Fatalf("balance = %d, want 99", b)
	}
}

func TestCapacityFullyConsumedRefundsNothing(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 100, 1)
	app := s.application(postingID, s.user("candidate"))
	approve(t, ctx, svc, owner, postingID, app)
	s.interview(app)
	s.setApplicationStatus(app, "approved")

	if _, err := svc.CloseIntake(ctx, owner, "recruiter", postingID); err != nil {
		t.Fatal(err)
	}
	status, err := repository.NewRepository(pool).EvaluateTerminalClose(ctx, postingID)
	if err != nil || status != "closed" {
		t.Fatalf("EvaluateTerminalClose() = %q, %v, want closed", status, err)
	}
	if rows := s.postingRows(postingID); len(rows) != 1 {
		t.Fatalf("coin rows = %+v, want only the debit", rows)
	}
	if b := s.balance(owner); b != 99 {
		t.Fatalf("balance = %d, want 99", b)
	}
	got, err := svc.GetCapacity(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Refunded == nil || *got.Refunded != 0 {
		t.Fatalf("refunded = %v, want 0", got.Refunded)
	}
}

func TestCapacityIntakeCloseWithEligibleNeverRefunds(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 100, 3)
	approved := s.application(postingID, s.user("candidate"))
	s.application(postingID, s.user("candidate"))
	approve(t, ctx, svc, owner, postingID, approved)

	closure, err := svc.CloseIntake(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if closure.Status != "intake_closed" || closure.RejectedCount != 1 {
		t.Fatalf("CloseIntake() = %+v, want intake_closed with one rejection", closure)
	}
	if rows := s.postingRows(postingID); len(rows) != 1 {
		t.Fatalf("coin rows = %+v, want only the debit", rows)
	}
	if b := s.balance(owner); b != 97 {
		t.Fatalf("balance = %d, want 97 (no refund at intake close)", b)
	}
	got, err := svc.GetCapacity(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PostingStatus != "intake_closed" || got.Refunded != nil {
		t.Fatalf("GetCapacity() = %+v, want intake_closed with no refunded amount", got)
	}
}

func TestCapacityUnfundedCloseRefundsNothing(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", nil)

	closure, err := svc.CloseIntake(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if closure.Status != "closed" {
		t.Fatalf("CloseIntake() = %+v, want closed", closure)
	}
	if n := s.walletRows(owner); n != 0 {
		t.Fatalf("wallet rows = %d, want the refund not to create one", n)
	}
	if rows := s.postingRows(postingID); len(rows) != 0 {
		t.Fatalf("coin rows = %+v, want none", rows)
	}
	got, err := svc.GetCapacity(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Refunded == nil || *got.Refunded != 0 {
		t.Fatalf("refunded = %v, want 0 on a closed unfunded posting", got.Refunded)
	}
}

func TestCapacityEvaluateTerminalCloseCreditsOnce(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 100, 3)
	app := s.application(postingID, s.user("candidate"))
	approve(t, ctx, svc, owner, postingID, app)
	if _, err := svc.CloseIntake(ctx, owner, "recruiter", postingID); err != nil {
		t.Fatal(err)
	}
	// The outcome leaves no live Application, so the posting is ready for its terminal close.
	s.setApplicationStatus(app, "approved")

	repo := repository.NewRepository(pool)
	const attempts = 8
	statuses := make([]string, attempts)
	errs := make([]error, attempts)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range statuses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			statuses[i], errs[i] = repo.EvaluateTerminalClose(ctx, postingID)
		}(i)
	}
	close(start)
	wg.Wait()

	for i := range errs {
		if errs[i] != nil || statuses[i] != "closed" {
			t.Fatalf("call %d = %q, %v, want closed with no error", i, statuses[i], errs[i])
		}
	}
	if b := s.balance(owner); b != 100 {
		t.Fatalf("balance = %d, want 100: the refund of 3 credited once", b)
	}
	refunds := 0
	for _, r := range s.postingRows(postingID) {
		if r.from == "system" && r.to == owner.String() {
			refunds++
		}
	}
	if refunds != 1 {
		t.Fatalf("refund rows = %d, want exactly one", refunds)
	}
}

func TestCapacityRefundFailureRollsBackTheClose(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 100, 3)
	app := s.application(postingID, s.user("candidate"))
	approve(t, ctx, svc, owner, postingID, app)
	if _, err := svc.CloseIntake(ctx, owner, "recruiter", postingID); err != nil {
		t.Fatal(err)
	}
	s.setApplicationStatus(app, "approved")
	// The refund of 3 would pass MaxInt32, so the credit fails inside the close transaction.
	s.setWalletBalance(owner, 2147483646)

	_, err := repository.NewRepository(pool).EvaluateTerminalClose(ctx, postingID)
	assertCode(t, err, apperror.CodeInternal)
	if status, _ := s.postingState(postingID); status != "intake_closed" {
		t.Fatalf("posting = %s after a failed refund, want intake_closed", status)
	}
	if b := s.balance(owner); b != 2147483646 {
		t.Fatalf("balance = %d after a failed refund, want unchanged", b)
	}
	if rows := s.postingRows(postingID); len(rows) != 1 {
		t.Fatalf("coin rows = %+v after a failed refund, want only the debit", rows)
	}
}

// --- Lifecycle (T1): fund, apply, approve, close, refund across B1 and B2 ---

// TestLifecycleFundApplyApproveAndAutoClose covers the full path: fund 3, five Candidates apply, three
// approvals, the approval that takes the last slot closes intake and rejects the rest, and the
// wallet is untouched by approvals.
func TestLifecycleFundApplyApproveAndAutoClose(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newServiceWithCVStore(t, pool)
	owner := s.user("recruiter")
	s.wallet(owner, 100)
	postingID := s.posting(owner, "open", nil)
	if _, err := svc.FundCapacity(ctx, owner, "recruiter", postingID, 3); err != nil {
		t.Fatal(err)
	}

	candidates := make([]uuid.UUID, 5)
	apps := make([]uuid.UUID, 5)
	for i := range apps {
		candidates[i] = s.user("candidate")
		apps[i] = submit(t, ctx, svc, candidates[i], postingID)
	}
	// Submissions are not capped by the slot count: five pending, none approved.
	before, err := svc.GetCapacity(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if before.CVApprovals != 0 || before.AvailableToApprove != 3 {
		t.Fatalf("before approvals = %+v, want 0 approvals and 3 available", before)
	}

	for _, id := range apps[:2] {
		if got := approve(t, ctx, svc, owner, postingID, id); got.JobPostingStatus != "open" {
			t.Fatalf("approval before the last slot reported posting %q, want open", got.JobPostingStatus)
		}
	}
	last := approve(t, ctx, svc, owner, postingID, apps[2])
	if last.JobPostingStatus != "intake_closed" || last.Status != domain.StatusInterviewEligible {
		t.Fatalf("last slot approval = %+v, want interview_eligible on an intake_closed posting", last)
	}

	// The fourth and fifth Candidates were rejected by the auto-close, so their approval is refused
	// as not pending. CAPACITY_FULL needs an open posting with every slot taken; see the next test.
	for _, id := range apps[3:] {
		_, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, id)
		assertCode(t, err, apperror.CodeApplicationNotPending)
		if st := s.stored(id).status; st != "rejected" {
			t.Fatalf("auto-closed Application status = %s, want rejected", st)
		}
	}
	if status, slot := s.postingState(postingID); status != "intake_closed" || slot == nil || *slot != 3 {
		t.Fatalf("posting = %s slot %v, want intake_closed with slot 3", status, slot)
	}
	if b := s.balance(owner); b != 97 {
		t.Fatalf("balance = %d after approvals, want 97 (approvals never touch the wallet)", b)
	}
	if rows := s.postingRows(postingID); len(rows) != 1 {
		t.Fatalf("coin rows = %+v after approvals, want only the funding debit", rows)
	}
	if n := s.walletRows(candidates...); n != 0 {
		t.Fatalf("Candidate wallet rows = %d, want none at any step", n)
	}
}

// TestLifecycleFourthApprovalAtFullCapacityIsCapacityFull reaches the state where every slot is
// approved while the posting is still open. The normal flow never gets there, because the approval
// that fills the last slot closes intake; the state is set directly, as the existing tests do.
func TestLifecycleFourthApprovalAtFullCapacityIsCapacityFull(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 100, 3)
	apps := make([]uuid.UUID, 4)
	for i := range apps {
		apps[i] = s.application(postingID, s.user("candidate"))
	}
	for _, id := range apps[:3] {
		approve(t, ctx, svc, owner, postingID, id)
	}
	s.exec(`UPDATE public.job_postings SET status = 'open' WHERE id = $1`, postingID)
	s.exec(`UPDATE public.applications SET status = 'pending', reject_reason = NULL WHERE id = $1`, apps[3])

	_, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, apps[3])
	assertCode(t, err, apperror.CodeInterviewCapacityFull)
}

// TestLifecycleApprovedCandidateStaysEligibleAfterIntakeCloses checks that Close Intake leaves the
// approved Candidate eligible and the posting intake_closed, and that the refund appears only after
// the outcome closes the posting.
func TestLifecycleApprovedCandidateStaysEligibleAfterIntakeCloses(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newServiceWithCVStore(t, pool)
	owner := s.user("recruiter")
	s.wallet(owner, 100)
	postingID := s.posting(owner, "open", nil)
	if _, err := svc.FundCapacity(ctx, owner, "recruiter", postingID, 2); err != nil {
		t.Fatal(err)
	}
	candidate := s.user("candidate")
	approved := submit(t, ctx, svc, candidate, postingID)
	pending := submit(t, ctx, svc, s.user("candidate"), postingID)
	approve(t, ctx, svc, owner, postingID, approved)

	closure, err := svc.CloseIntake(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if closure.Status != "intake_closed" || closure.RejectedCount != 1 {
		t.Fatalf("CloseIntake() = %+v, want intake_closed with the pending one rejected", closure)
	}
	if st := s.stored(pending).status; st != "rejected" {
		t.Fatalf("pending Application = %s, want rejected", st)
	}
	if st := s.stored(approved).status; st != "interview_eligible" {
		t.Fatalf("approved Application = %s after intake close, want interview_eligible", st)
	}
	if _, err := svc.InterviewEligibility(ctx, candidate, "candidate", approved); err != nil {
		t.Fatalf("InterviewEligibility() after intake close = %v, want eligible", err)
	}
	if rows := s.postingRows(postingID); len(rows) != 1 {
		t.Fatalf("coin rows = %+v after intake close, want no refund", rows)
	}

	// The outcome is the only thing that can close the posting and refund it.
	s.setApplicationStatus(approved, "approved")
	status, err := repository.NewRepository(pool).EvaluateTerminalClose(ctx, postingID)
	if err != nil || status != "closed" {
		t.Fatalf("EvaluateTerminalClose() = %q, %v, want closed", status, err)
	}
	if b := s.balance(owner); b != 100 {
		t.Fatalf("balance = %d after the outcome, want 100 (2 funded, 2 refunded)", b)
	}
	if n := s.walletRows(candidate); n != 0 {
		t.Fatalf("Candidate wallet rows = %d, want none", n)
	}
}

// TestLifecycleConsumedInterviewsReduceRefund funds 4, consumes one with an interview, and checks the
// refund is 3 and the wallet change equals the ledger net.
func TestLifecycleConsumedInterviewsReduceRefund(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, postingID := fundedPosting(t, ctx, s, svc, 100, 4)
	first := s.application(postingID, s.user("candidate"))
	second := s.application(postingID, s.user("candidate"))
	approve(t, ctx, svc, owner, postingID, first)
	approve(t, ctx, svc, owner, postingID, second)
	s.interview(first)
	if _, err := svc.CloseIntake(ctx, owner, "recruiter", postingID); err != nil {
		t.Fatal(err)
	}
	s.setApplicationStatus(first, "approved")
	s.setApplicationStatus(second, "rejected")
	if _, err := repository.NewRepository(pool).EvaluateTerminalClose(ctx, postingID); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetCapacity(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Refunded == nil || *got.Refunded != 3 || got.Consumed != 1 || got.Refundable != 3 {
		t.Fatalf("GetCapacity() = %+v, want refunded 3 with 1 consumed of 4", got)
	}
	if b := s.balance(owner); b != 99 {
		t.Fatalf("balance = %d, want 100 - 4 + 3 = 99", b)
	}
	if net := s.ledgerNet(owner); net != int64(s.balance(owner))-100 {
		t.Fatalf("ledger net = %d, want the balance change %d", net, int64(s.balance(owner))-100)
	}
}

// TestLifecycleNoReopenOrEndRecruitmentRoute checks the route table: capacity has the fund and read
// routes and the existing Close Intake, and no route reopens a posting or ends recruitment. It reads
// the router source, so it is a static check of the registered paths.
func TestLifecycleNoReopenOrEndRecruitmentRoute(t *testing.T) {
	src, err := os.ReadFile("../../../router/router.go")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(src))
	for _, want := range []string{"/interview-capacity", "/close-intake"} {
		if !strings.Contains(text, want) {
			t.Fatalf("router has no %s route", want)
		}
	}
	for _, banned := range []string{"reopen", "end-recruitment", "end_recruitment", "endrecruitment"} {
		if strings.Contains(text, banned) {
			t.Fatalf("router mentions %q, want no route to reopen or end recruitment", banned)
		}
	}
}
