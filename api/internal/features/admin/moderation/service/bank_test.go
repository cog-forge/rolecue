package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// The bank is locked once a posting leaves pending (KAN-79), so approving an empty one would leave
// it with nothing to interview from.
func TestApproveNeedsCoreQuestions(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	repo.posting.CoreQuestionCount = 0
	s, logs := newObservedService(t, repo)
	_, err := s.Approve(context.Background(), uuid.New(), "admin", repo.posting.ID)
	assertCode(t, err, apperror.CodeQuestionBankNotReady)
	if repo.posting.Status != domain.PostingStatusPending {
		t.Fatal("a posting with an empty bank was approved")
	}
	if logs.Len() != 0 {
		t.Fatal("a refused approval wrote an audit line")
	}
}

func TestApproveWithOneCoreQuestion(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	repo.posting.CoreQuestionCount = 1
	got, err := newService(t, repo).Approve(context.Background(), uuid.New(), "admin", repo.posting.ID)
	if err != nil || got.Status != domain.PostingStatusOpen {
		t.Fatalf("status=%s err=%v", got.Status, err)
	}
}

func TestRejectAllowedWithEmptyBank(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	repo.posting.CoreQuestionCount = 0
	got, err := newService(t, repo).Reject(context.Background(), uuid.New(), "admin", repo.posting.ID, "No questions yet")
	if err != nil || got.Status != domain.PostingStatusRejected {
		t.Fatalf("status=%s err=%v", got.Status, err)
	}
}

// An empty bank only matters while the posting is pending; a repeated approval of an open posting
// still returns its current state.
func TestEmptyBankDoesNotBreakRepeatedApproval(t *testing.T) {
	repo := postingIn(domain.PostingStatusOpen)
	repo.posting.CoreQuestionCount = 0
	if got, err := newService(t, repo).Approve(context.Background(), uuid.New(), "admin", repo.posting.ID); err != nil || got.Status != domain.PostingStatusOpen {
		t.Fatalf("status=%s err=%v", got.Status, err)
	}
}
