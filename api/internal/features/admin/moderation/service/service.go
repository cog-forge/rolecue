package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const adminRole = "admin"

// Repository is the persistence contract consumed by Job Posting moderation.
type Repository interface {
	List(ctx context.Context, filter domain.ListFilter) ([]domain.PostingSummary, int64, error)
	Get(ctx context.Context, id uuid.UUID) (domain.ReviewPosting, error)
	Decide(ctx context.Context, id uuid.UUID, status domain.PostingStatus, rejectReason *string) error
}

type Service struct {
	repository Repository
	log        *logger.Logger
}

// New builds the service. Decisions are logged because the brain asks for an admin audit trail
// while the frozen schema has no audit table.
func New(repository Repository, log *logger.Logger) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("moderation repository is required")
	}
	if log == nil {
		log = &logger.Logger{Logger: zap.NewNop()}
	}
	return &Service{repository: repository, log: log}, nil
}

func requireAdmin(role string) error {
	if role != adminRole {
		return apperror.New(apperror.CodeForbidden, "only administrators can moderate job postings")
	}
	return nil
}

func (s *Service) List(ctx context.Context, role string, filter domain.ListFilter) ([]domain.PostingSummary, int64, error) {
	if err := requireAdmin(role); err != nil {
		return nil, 0, err
	}
	if err := validateFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.repository.List(ctx, filter)
}

func (s *Service) Get(ctx context.Context, role string, id uuid.UUID) (domain.ReviewPosting, error) {
	if err := requireAdmin(role); err != nil {
		return domain.ReviewPosting{}, err
	}
	return s.repository.Get(ctx, id)
}

func (s *Service) Approve(ctx context.Context, adminID uuid.UUID, role string, id uuid.UUID) (domain.ReviewPosting, error) {
	if err := requireAdmin(role); err != nil {
		return domain.ReviewPosting{}, err
	}
	return s.decide(ctx, adminID, id, domain.PostingStatusOpen, nil)
}

func (s *Service) Reject(ctx context.Context, adminID uuid.UUID, role string, id uuid.UUID, reason string) (domain.ReviewPosting, error) {
	if err := requireAdmin(role); err != nil {
		return domain.ReviewPosting{}, err
	}
	reason, err := normalizeReason(reason)
	if err != nil {
		return domain.ReviewPosting{}, err
	}
	return s.decide(ctx, adminID, id, domain.PostingStatusRejected, &reason)
}

// decide applies the transition. When the posting is no longer pending, repeating the decision it
// already has succeeds with the current state, so a client retry after a lost response is safe.
func (s *Service) decide(ctx context.Context, adminID, id uuid.UUID, status domain.PostingStatus, reason *string) (domain.ReviewPosting, error) {
	err := s.repository.Decide(ctx, id, status, reason)
	if err == nil {
		// Log before re-reading: the decision is already committed, and a failed read followed by an
		// idempotent retry would otherwise leave it with no audit line at all.
		s.log.WithContext(ctx).Info("job posting moderated",
			logger.String("admin_id", adminID.String()),
			logger.String("job_posting_id", id.String()),
			logger.String("status", string(status)),
		)
		return s.repository.Get(ctx, id)
	}
	if !errors.Is(err, domain.ErrNotPending) {
		return domain.ReviewPosting{}, err
	}
	posting, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.ReviewPosting{}, err
	}
	switch {
	case posting.Status == status:
		return posting, nil
	case posting.Status == domain.PostingStatusPending && status == domain.PostingStatusOpen:
		// Still pending, so the only guard that failed is the non-empty question bank.
		return domain.ReviewPosting{}, domain.ErrQuestionBankNotReady
	default:
		return domain.ReviewPosting{}, domain.ErrNotPending
	}
}
