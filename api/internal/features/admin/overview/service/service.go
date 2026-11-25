package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/overview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

const adminRole = "admin"

// Repository is the persistence contract consumed by the Admin overview. Every method is read-only.
type Repository interface {
	Accounts(ctx context.Context, window, previous domain.OverviewWindow) (domain.OverviewAccounts, error)
	Postings(ctx context.Context) (domain.OverviewPostings, error)
	Applications(ctx context.Context, window, previous domain.OverviewWindow) (domain.OverviewApplications, error)
	Interviews(ctx context.Context, window, previous domain.OverviewWindow) (domain.OverviewInterviews, domain.OverviewInterviewActivity, error)
}

// Finance supplies the wallet billing summary for the window.
type Finance interface {
	Summary(ctx context.Context, from, to time.Time) (domain.OverviewFinance, error)
}

type Service struct {
	repository Repository
	finance    Finance
	now        func() time.Time
}

func New(repository Repository, finance Finance) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("admin overview repository is required")
	}
	if finance == nil {
		return nil, fmt.Errorf("admin overview finance is required")
	}
	return &Service{repository: repository, finance: finance, now: time.Now}, nil
}

// Get returns the Admin overview for a window of days (7, 30 or 90). Zero means the default window.
// The role is checked before anything is queried.
func (s *Service) Get(ctx context.Context, role string, days int) (domain.Overview, error) {
	if role != adminRole {
		return domain.Overview{}, apperror.New(apperror.CodeForbidden, "only administrators can view the admin overview")
	}
	windowDays, err := resolveDays(days)
	if err != nil {
		return domain.Overview{}, err
	}
	// Use one microsecond-precision cutoff for PostgreSQL and the serialized window bounds.
	now := s.now().Truncate(time.Microsecond)
	duration := time.Duration(windowDays) * 24 * time.Hour
	window := domain.OverviewWindow{Days: windowDays, From: now.Add(-duration), To: now}
	previous := domain.OverviewWindow{Days: windowDays, From: window.From.Add(-duration), To: window.From}

	accounts, err := s.repository.Accounts(ctx, window, previous)
	if err != nil {
		return domain.Overview{}, err
	}
	postings, err := s.repository.Postings(ctx)
	if err != nil {
		return domain.Overview{}, err
	}
	applications, err := s.repository.Applications(ctx, window, previous)
	if err != nil {
		return domain.Overview{}, err
	}
	interviews, activity, err := s.repository.Interviews(ctx, window, previous)
	if err != nil {
		return domain.Overview{}, err
	}
	finance, err := s.finance.Summary(ctx, window.From, window.To)
	if err != nil {
		return domain.Overview{}, err
	}
	previousFinance, err := s.finance.Summary(ctx, previous.From, previous.To)
	if err != nil {
		return domain.Overview{}, err
	}
	finance.PreviousRevenueVND = previousFinance.RevenueVND
	return domain.Overview{
		GeneratedAt:       now,
		Window:            window,
		PreviousWindow:    previous,
		Accounts:          accounts,
		JobPostings:       postings,
		Applications:      applications,
		Interviews:        interviews,
		InterviewActivity: activity,
		Finance:           finance,
	}, nil
}

func resolveDays(days int) (int, error) {
	if days == 0 {
		return domain.DefaultWindowDays, nil
	}
	if slices.Contains(domain.WindowDays, days) {
		return days, nil
	}
	return 0, apperror.New(apperror.CodeValidation, "days must be 7, 30 or 90")
}
