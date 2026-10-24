package service

import (
	"context"
	"fmt"
	"math"

	"github.com/cog-forge/rolecue/api/internal/features/admin/interviews/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

const adminRole = "admin"

// Repository is the persistence contract consumed by the Admin interview oversight.
type Repository interface {
	List(ctx context.Context, filter domain.ListFilter) ([]domain.InterviewRow, int64, error)
}

type Service struct {
	repository Repository
}

func New(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("admin interview repository is required")
	}
	return &Service{repository: repository}, nil
}

func requireAdmin(role string) error {
	if role != adminRole {
		return apperror.New(apperror.CodeForbidden, "only administrators can view interview sessions")
	}
	return nil
}

func invalid(message string) error { return apperror.New(apperror.CodeValidation, message) }

// List returns one page of interviews. Only operational metadata comes back; the repository never
// reads recording paths, transcripts or answers into the result.
func (s *Service) List(ctx context.Context, role string, filter domain.ListFilter) (domain.InterviewPage, error) {
	if err := requireAdmin(role); err != nil {
		return domain.InterviewPage{}, err
	}
	if err := validateFilter(filter); err != nil {
		return domain.InterviewPage{}, err
	}
	items, total, err := s.repository.List(ctx, filter)
	if err != nil {
		return domain.InterviewPage{}, err
	}
	if items == nil {
		items = []domain.InterviewRow{}
	}
	return domain.InterviewPage{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func validateFilter(filter domain.ListFilter) error {
	switch filter.Type {
	case "", domain.TypePractice, domain.TypeRecruitment:
	default:
		return invalid("invalid interview type filter")
	}
	if filter.Page < 1 {
		return invalid("page must be at least 1")
	}
	if filter.PageSize < 1 || filter.PageSize > domain.MaxPageSize {
		return invalid(fmt.Sprintf("page_size must be between 1 and %d", domain.MaxPageSize))
	}
	// The offset is passed to SQL as a 32-bit integer.
	if int64(filter.Page-1)*int64(filter.PageSize) > math.MaxInt32 {
		return invalid("page is out of range")
	}
	return nil
}
