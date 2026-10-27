package service

import (
	"context"
	"fmt"
	"math"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// HistoryRepository is the read contract of the Candidate's session history. The interview Repository
// implements it in its own file, so the session contract stays as it is.
type HistoryRepository interface {
	ListHistory(ctx context.Context, candidateID uuid.UUID, filter domain.HistoryFilter) ([]domain.HistoryItem, int64, error)
}

// historyRepository returns the history contract of the configured repository.
func (s *Service) historyRepository() (HistoryRepository, error) {
	repo, ok := s.repository.(HistoryRepository)
	if !ok {
		return nil, apperror.New(apperror.CodeInternal, "interview history is not available")
	}
	return repo, nil
}

// ListHistory returns one page of the Candidate's own sessions, newest first. Only list fields are
// returned; the repository never reads questions, turns, feedback or the recording path into the result.
func (s *Service) ListHistory(ctx context.Context, candidateID uuid.UUID, role string, filter domain.HistoryFilter) (domain.HistoryPage, error) {
	if role != candidateRole {
		return domain.HistoryPage{}, apperror.New(apperror.CodeForbidden, "only candidates can read interview history")
	}
	if err := validateHistoryFilter(filter); err != nil {
		return domain.HistoryPage{}, err
	}
	repo, err := s.historyRepository()
	if err != nil {
		return domain.HistoryPage{}, err
	}
	items, total, err := repo.ListHistory(ctx, candidateID, filter)
	if err != nil {
		return domain.HistoryPage{}, err
	}
	// An empty page is an empty array, not null.
	if items == nil {
		items = []domain.HistoryItem{}
	}
	return domain.HistoryPage{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

// validateHistoryFilter checks the type, the page and the page size. The offset is passed to SQL as a
// 32-bit integer, so a page beyond that range is refused here.
func validateHistoryFilter(filter domain.HistoryFilter) error {
	switch filter.Type {
	case "", domain.TypePractice, domain.TypeRecruitment:
	default:
		return apperror.New(apperror.CodeValidation, "type must be practice or recruitment")
	}
	if filter.Page < 1 {
		return apperror.New(apperror.CodeValidation, "page must be at least 1")
	}
	if filter.PageSize < 1 || filter.PageSize > domain.MaxHistoryPageSize {
		return apperror.New(apperror.CodeValidation, fmt.Sprintf("page_size must be between 1 and %d", domain.MaxHistoryPageSize))
	}
	if int64(filter.Page-1)*int64(filter.PageSize) > math.MaxInt32 {
		return apperror.New(apperror.CodeValidation, "page is out of range")
	}
	return nil
}
