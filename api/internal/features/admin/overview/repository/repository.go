package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/overview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository reads the Admin overview counts. It only reads.
type Repository struct {
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{queries: New(pool)}
}

// NewRepositoryTx reads through one transaction, so a test sees its own uncommitted rows and a fixed
// snapshot of everyone else's.
func NewRepositoryTx(tx pgx.Tx) *Repository {
	return &Repository{queries: New(tx)}
}

// Accounts counts Candidate and Recruiter accounts, the active locks and the accounts created in the window.
func (r *Repository) Accounts(ctx context.Context, from, to time.Time) (domain.OverviewAccounts, error) {
	row, err := r.queries.CountOverviewAccounts(ctx, CountOverviewAccountsParams{
		WindowFrom: timestamp(from),
		WindowTo:   timestamp(to),
	})
	if err != nil {
		return domain.OverviewAccounts{}, persistenceError(err)
	}
	return domain.OverviewAccounts{
		Candidates:  row.Candidates,
		Recruiters:  row.Recruiters,
		Locked:      row.Locked,
		NewInWindow: row.NewInWindow,
	}, nil
}

// Postings counts Job Postings by status, all-time.
func (r *Repository) Postings(ctx context.Context) (domain.OverviewPostings, error) {
	row, err := r.queries.CountOverviewJobPostings(ctx)
	if err != nil {
		return domain.OverviewPostings{}, persistenceError(err)
	}
	return domain.OverviewPostings{
		Pending:      row.Pending,
		Open:         row.Open,
		IntakeClosed: row.IntakeClosed,
		Closed:       row.Closed,
		Rejected:     row.Rejected,
	}, nil
}

// Applications counts Applications by status, all-time, and the ones submitted in the window.
func (r *Repository) Applications(ctx context.Context, from, to time.Time) (domain.OverviewApplications, error) {
	row, err := r.queries.CountOverviewApplications(ctx, CountOverviewApplicationsParams{
		WindowFrom: timestamp(from),
		WindowTo:   timestamp(to),
	})
	if err != nil {
		return domain.OverviewApplications{}, persistenceError(err)
	}
	return domain.OverviewApplications{
		Pending:           row.Pending,
		InterviewEligible: row.InterviewEligible,
		Interviewed:       row.Interviewed,
		Approved:          row.Approved,
		Rejected:          row.Rejected,
		SubmittedInWindow: row.SubmittedInWindow,
	}, nil
}

// Interviews counts interview sessions started and completed in the window, and failed evaluations.
func (r *Repository) Interviews(ctx context.Context, from, to time.Time) (domain.OverviewInterviews, error) {
	row, err := r.queries.CountOverviewInterviews(ctx, CountOverviewInterviewsParams{
		WindowFrom: timestamp(from),
		WindowTo:   timestamp(to),
	})
	if err != nil {
		return domain.OverviewInterviews{}, persistenceError(err)
	}
	return domain.OverviewInterviews{
		PracticeStartedInWindow:    row.PracticeStartedInWindow,
		RecruitmentStartedInWindow: row.RecruitmentStartedInWindow,
		CompletedInWindow:          row.CompletedInWindow,
		EvaluationFailed:           row.EvaluationFailed,
	}, nil
}

func timestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func persistenceError(err error) error {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.Wrap(apperror.CodeInternal, "admin overview persistence failed", err)
}
