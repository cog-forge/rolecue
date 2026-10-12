package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestApproveOpensPendingPosting(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	got, err := newService(t, repo).Approve(context.Background(), uuid.New(), "admin", repo.posting.ID)
	if err != nil || got.Status != domain.PostingStatusOpen {
		t.Fatalf("status=%s err=%v", got.Status, err)
	}
	if repo.decidedStatus != domain.PostingStatusOpen || repo.decidedReason != nil {
		t.Fatalf("decided %s with reason %v", repo.decidedStatus, repo.decidedReason)
	}
}

func TestRejectStoresTrimmedReason(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	got, err := newService(t, repo).Reject(context.Background(), uuid.New(), "admin", repo.posting.ID, "  Salary range is missing  ")
	if err != nil || got.Status != domain.PostingStatusRejected {
		t.Fatalf("status=%s err=%v", got.Status, err)
	}
	if repo.decidedReason == nil || *repo.decidedReason != "Salary range is missing" {
		t.Fatalf("reason = %v", repo.decidedReason)
	}
}

// A retry after a lost response repeats the decision the posting already has; it must succeed.
func TestRepeatedDecisionReturnsCurrentState(t *testing.T) {
	ctx := context.Background()
	repo := postingIn(domain.PostingStatusOpen)
	if got, err := newService(t, repo).Approve(ctx, uuid.New(), "admin", repo.posting.ID); err != nil || got.Status != domain.PostingStatusOpen {
		t.Fatalf("approve again: status=%s err=%v", got.Status, err)
	}
	reason := "Duplicate posting"
	repo = &fakeRepository{posting: &domain.ReviewPosting{ID: uuid.New(), Status: domain.PostingStatusRejected, RejectReason: &reason}}
	got, err := newService(t, repo).Reject(ctx, uuid.New(), "admin", repo.posting.ID, "Another reason")
	if err != nil || got.Status != domain.PostingStatusRejected || *got.RejectReason != reason {
		t.Fatalf("reject again: status=%s reason=%v err=%v", got.Status, got.RejectReason, err)
	}
	if repo.decidedReason != nil {
		t.Fatal("a repeated reject overwrote the stored reason")
	}
}

func TestApproveTwiceIsIdempotent(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	s, logs := newObservedService(t, repo)
	for i := 0; i < 2; i++ {
		got, err := s.Approve(context.Background(), uuid.New(), "admin", repo.posting.ID)
		if err != nil || got.Status != domain.PostingStatusOpen {
			t.Fatalf("attempt %d: status=%s err=%v", i+1, got.Status, err)
		}
	}
	if logs.Len() != 1 {
		t.Fatalf("audit lines = %d, want 1 (the retry changed nothing)", logs.Len())
	}
}

// The decision is committed before the re-read, so it must be logged even when the read fails and
// the client's retry then takes the idempotent path.
func TestDecisionIsLoggedEvenWhenReadFails(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	repo.getErr = errors.New("pool timeout")
	s, logs := newObservedService(t, repo)
	adminID := uuid.New()
	if _, err := s.Approve(context.Background(), adminID, "admin", repo.posting.ID); !errors.Is(err, repo.getErr) {
		t.Fatalf("error = %v", err)
	}
	if logs.Len() != 1 {
		t.Fatalf("audit lines = %d, want 1", logs.Len())
	}
	fields := logs.All()[0].ContextMap()
	if fields["admin_id"] != adminID.String() || fields["job_posting_id"] != repo.posting.ID.String() || fields["status"] != "open" {
		t.Fatalf("audit fields = %v", fields)
	}
	repo.getErr = nil
	if got, err := s.Approve(context.Background(), adminID, "admin", repo.posting.ID); err != nil || got.Status != domain.PostingStatusOpen {
		t.Fatalf("retry: status=%s err=%v", got.Status, err)
	}
	if logs.Len() != 1 {
		t.Fatal("the idempotent retry wrote a second audit line")
	}
}

func TestConflictingDecisionIsNotPending(t *testing.T) {
	for _, tt := range []struct {
		name    string
		current domain.PostingStatus
		approve bool
	}{
		{"approve rejected", domain.PostingStatusRejected, true},
		{"reject open", domain.PostingStatusOpen, false},
		{"approve closed", domain.PostingStatusClosed, true},
		{"reject intake closed", domain.PostingStatusIntakeClosed, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := postingIn(tt.current)
			s := newService(t, repo)
			var err error
			if tt.approve {
				_, err = s.Approve(context.Background(), uuid.New(), "admin", repo.posting.ID)
			} else {
				_, err = s.Reject(context.Background(), uuid.New(), "admin", repo.posting.ID, "Not suitable")
			}
			assertCode(t, err, apperror.CodeJobPostingNotPending)
			if repo.posting.Status != tt.current {
				t.Fatal("a non-pending posting changed state")
			}
		})
	}
}

func TestDecideMissingPostingIsNotFound(t *testing.T) {
	_, err := newService(t, &fakeRepository{}).Approve(context.Background(), uuid.New(), "admin", uuid.New())
	assertCode(t, err, apperror.CodeJobPostingNotFound)
}

func TestDecidePassesRepositoryErrorsThrough(t *testing.T) {
	sentinel := errors.New("db down")
	repo := postingIn(domain.PostingStatusPending)
	repo.decideErr = sentinel
	if _, err := newService(t, repo).Approve(context.Background(), uuid.New(), "admin", repo.posting.ID); !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
	if repo.getCalls != 0 {
		t.Fatal("a failed decision was followed by a read")
	}
}

func TestRejectReasonValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		reason string
	}{
		{"blank", "   "},
		{"too long", strings.Repeat("界", domain.MaxRejectReasonRunes+1)},
		{"nul", "bad\x00reason"},
		{"invalid utf-8", "bad\xffreason"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := postingIn(domain.PostingStatusPending)
			_, err := newService(t, repo).Reject(context.Background(), uuid.New(), "admin", repo.posting.ID, tt.reason)
			assertCode(t, err, apperror.CodeValidation)
			if repo.decideCalls != 0 {
				t.Fatal("an invalid reason reached the repository")
			}
		})
	}
	repo := postingIn(domain.PostingStatusPending)
	if _, err := newService(t, repo).Reject(context.Background(), uuid.New(), "admin", repo.posting.ID, strings.Repeat("界", domain.MaxRejectReasonRunes)); err != nil {
		t.Fatalf("reason at the limit rejected: %v", err)
	}
}
