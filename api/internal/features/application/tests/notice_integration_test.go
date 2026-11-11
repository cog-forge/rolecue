//go:build integration

package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/features/application/repository"
	"github.com/google/uuid"
)

// storedEmail reads a user's email as stored.
func storedEmail(t *testing.T, s *seed, userID uuid.UUID) string {
	t.Helper()
	var email string
	if err := s.pool.QueryRow(s.ctx, `SELECT email FROM public.users WHERE id = $1`, userID).Scan(&email); err != nil {
		t.Fatal(err)
	}
	return email
}

// idIn reports whether id is one of ids. The order of returned ids is not part of the contract.
func idIn(ids []uuid.UUID, id uuid.UUID) bool {
	for _, got := range ids {
		if got == id {
			return true
		}
	}
	return false
}

func TestNoticeContextsCarryRecipientsTitleAndDeadline(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(5))
	candidateEmail := fmt.Sprintf("notice-%s@example.test", uuid.New())
	candidate := s.namedCandidate("Notice Candidate", candidateEmail)
	eligibleID := s.application(postingID, candidate)
	deadline := lcAt(time.Hour)
	setStatusAndDeadline(s, eligibleID, "interview_eligible", lcTime(deadline))
	pendingID := s.application(postingID, s.user("candidate"))

	got, err := repo.NoticeContexts(ctx, []uuid.UUID{eligibleID, pendingID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("NoticeContexts() returned %d contexts, want 2", len(got))
	}
	byID := make(map[uuid.UUID]domain.NoticeContext, len(got))
	for _, c := range got {
		byID[c.ApplicationID] = c
	}

	eligible, ok := byID[eligibleID]
	if !ok {
		t.Fatalf("NoticeContexts() has no context for the eligible Application: %+v", got)
	}
	if eligible.JobPostingID != postingID || eligible.PostingTitle != "app posting" {
		t.Fatalf("posting = %s %q, want %s %q", eligible.JobPostingID, eligible.PostingTitle, postingID, "app posting")
	}
	if eligible.CandidateName != "Notice Candidate" || eligible.CandidateEmail != candidateEmail {
		t.Fatalf("candidate = %q %q, want %q %q", eligible.CandidateName, eligible.CandidateEmail, "Notice Candidate", candidateEmail)
	}
	if eligible.RecruiterEmail != storedEmail(t, s, owner) {
		t.Fatalf("recruiter email = %q, want the posting owner's email", eligible.RecruiterEmail)
	}
	if eligible.InterviewDeadline == nil || !eligible.InterviewDeadline.Equal(deadline) {
		t.Fatalf("interview deadline = %v, want %v", eligible.InterviewDeadline, deadline)
	}

	pending, ok := byID[pendingID]
	if !ok {
		t.Fatalf("NoticeContexts() has no context for the pending Application: %+v", got)
	}
	if pending.InterviewDeadline != nil {
		t.Fatalf("pending interview deadline = %v, want nil", pending.InterviewDeadline)
	}
	if pending.RecruiterEmail != storedEmail(t, s, owner) {
		t.Fatalf("pending recruiter email = %q, want the posting owner's email", pending.RecruiterEmail)
	}

	none, err := repo.NoticeContexts(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if none != nil {
		t.Fatalf("NoticeContexts(nil) = %+v, want nil", none)
	}
}

func TestCloseIntakeReturnsRejectedIDs(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(5))
	pending1 := s.application(postingID, s.user("candidate"))
	pending2 := s.application(postingID, s.user("candidate"))
	eligible := s.application(postingID, s.user("candidate"))
	if _, err := repo.ApproveCV(ctx, owner, postingID, eligible); err != nil {
		t.Fatal(err)
	}

	closure, err := repo.CloseIntake(ctx, owner, postingID)
	if err != nil {
		t.Fatal(err)
	}
	if closure.RejectedCount != 2 || len(closure.RejectedIDs) != 2 {
		t.Fatalf("CloseIntake() = %+v, want 2 rejected ids", closure)
	}
	if !idIn(closure.RejectedIDs, pending1) || !idIn(closure.RejectedIDs, pending2) || idIn(closure.RejectedIDs, eligible) {
		t.Fatalf("RejectedIDs = %v, want the two pending Applications only", closure.RejectedIDs)
	}

	again, err := repo.CloseIntake(ctx, owner, postingID)
	if err != nil {
		t.Fatal(err)
	}
	if again.RejectedCount != 0 || len(again.RejectedIDs) != 0 {
		t.Fatalf("repeat CloseIntake() = %+v, want no rejected ids", again)
	}
}

func TestLastSlotApprovalReturnsRejectedIDs(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(1))
	pending1 := s.application(postingID, s.user("candidate"))
	pending2 := s.application(postingID, s.user("candidate"))
	last := s.application(postingID, s.user("candidate"))

	screening, err := repo.ApproveCV(ctx, owner, postingID, last)
	if err != nil {
		t.Fatal(err)
	}
	if screening.JobPostingStatus != "intake_closed" {
		t.Fatalf("JobPostingStatus = %q, want intake_closed", screening.JobPostingStatus)
	}
	if len(screening.RejectedIDs) != 2 || !idIn(screening.RejectedIDs, pending1) || !idIn(screening.RejectedIDs, pending2) {
		t.Fatalf("RejectedIDs = %v, want the two pending Applications", screening.RejectedIDs)
	}
	if idIn(screening.RejectedIDs, last) {
		t.Fatal("RejectedIDs contains the approved Application")
	}
}

func TestAutoRejectExpiredIDsReturnsExpiredIDs(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(5))
	expired := lcEligibleExpired(s, postingID)
	expiredStarted := lcEligibleExpired(s, postingID)
	s.interview(expiredStarted) // started, never completed: still expired
	future := s.application(postingID, s.user("candidate"))
	setStatusAndDeadline(s, future, "interview_eligible", lcTime(lcEpochAt(time.Hour)))

	got, err := repo.AutoRejectExpiredIDs(ctx, lcEpoch)
	if err != nil {
		t.Fatal(err)
	}
	if !idIn(got, expired) || !idIn(got, expiredStarted) {
		t.Fatalf("AutoRejectExpiredIDs() = %v, want the two expired Applications", got)
	}
	if idIn(got, future) {
		t.Fatal("AutoRejectExpiredIDs() contains an Application whose deadline has not passed")
	}
	if status := lcStatus(s, expired); status != "rejected" {
		t.Fatalf("expired Application status = %s, want rejected", status)
	}
}
