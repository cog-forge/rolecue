package service

import (
	"context"
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/features/voice/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// Repository is the persistence contract consumed by the voice list.
type Repository interface {
	ListActive(ctx context.Context, filter domain.ListFilter) ([]domain.Voice, int64, error)
}

type Service struct{ repository Repository }

func New(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("voice repository is required")
	}
	return &Service{repository: repository}, nil
}

// ListActive returns the voices a Recruiter can require for a Job Posting or a Candidate can pick
// for practice. Admins manage voices through the Admin catalog instead.
func (s *Service) ListActive(ctx context.Context, role string, filter domain.ListFilter) ([]domain.Voice, int64, error) {
	if role != "candidate" && role != "recruiter" {
		return nil, 0, apperror.New(apperror.CodeForbidden, "only candidates and recruiters can choose a voice")
	}
	if filter.Limit < 1 || filter.Limit > 100 || filter.Offset < 0 {
		return nil, 0, apperror.New(apperror.CodeValidation, "invalid pagination parameters")
	}
	return s.repository.ListActive(ctx, filter)
}
