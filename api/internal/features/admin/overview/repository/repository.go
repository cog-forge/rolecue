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
func (r *Repository) Accounts(ctx context.Context, window, previous domain.OverviewWindow) (domain.OverviewAccounts, error) {
	row, err := r.queries.CountOverviewAccounts(ctx, CountOverviewAccountsParams{
		WindowFrom:   timestamp(window.From),
		WindowTo:     timestamp(window.To),
		PreviousFrom: timestamp(previous.From),
	})
	if err != nil {
		return domain.OverviewAccounts{}, persistenceError(err)
	}
	return domain.OverviewAccounts{
		Candidates:          row.Candidates,
		Recruiters:          row.Recruiters,
		Locked:              row.Locked,
		NewInWindow:         row.NewInWindow,
		NewInPreviousWindow: row.NewInPreviousWindow,
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
func (r *Repository) Applications(ctx context.Context, window, previous domain.OverviewWindow) (domain.OverviewApplications, error) {
	row, err := r.queries.CountOverviewApplications(ctx, CountOverviewApplicationsParams{
		WindowFrom:   timestamp(window.From),
		WindowTo:     timestamp(window.To),
		PreviousFrom: timestamp(previous.From),
	})
	if err != nil {
		return domain.OverviewApplications{}, persistenceError(err)
	}
	return domain.OverviewApplications{
		Pending:                   row.Pending,
		InterviewEligible:         row.InterviewEligible,
		Interviewed:               row.Interviewed,
		Approved:                  row.Approved,
		Rejected:                  row.Rejected,
		SubmittedInWindow:         row.SubmittedInWindow,
		SubmittedInPreviousWindow: row.SubmittedInPreviousWindow,
	}, nil
}

// Interviews counts interview sessions started and completed in the window, and failed evaluations.
func (r *Repository) Interviews(ctx context.Context, window, previous domain.OverviewWindow) (domain.OverviewInterviews, domain.OverviewInterviewActivity, error) {
	row, err := r.queries.CountOverviewInterviews(ctx, CountOverviewInterviewsParams{
		WindowFrom: timestamp(window.From),
		WindowTo:   timestamp(window.To),
	})
	if err != nil {
		return domain.OverviewInterviews{}, domain.OverviewInterviewActivity{}, persistenceError(err)
	}
	bucketDays := 1
	if window.Days == 90 {
		bucketDays = 7
	}
	starts, err := r.queries.CountOverviewInterviewActivity(ctx, CountOverviewInterviewActivityParams{
		BucketSeconds: int32(bucketDays * 24 * 60 * 60),
		WindowFrom:    timestamp(window.From),
		WindowTo:      timestamp(window.To),
		PreviousFrom:  timestamp(previous.From),
	})
	if err != nil {
		return domain.OverviewInterviews{}, domain.OverviewInterviewActivity{}, persistenceError(err)
	}

	summary := domain.OverviewInterviews{CompletedInWindow: row.CompletedInWindow, EvaluationFailed: row.EvaluationFailed}
	activity := domain.OverviewInterviewActivity{
		BucketDays: bucketDays,
		Buckets:    make([]domain.OverviewInterviewBucket, (window.Days+bucketDays-1)/bucketDays),
	}
	for _, row := range starts {
		activity.Buckets[row.BucketIndex] = domain.OverviewInterviewBucket{
			Practice:            row.Practice,
			Recruitment:         row.Recruitment,
			PreviousPractice:    row.PreviousPractice,
			PreviousRecruitment: row.PreviousRecruitment,
		}
		summary.PracticeStartedInWindow += row.Practice
		summary.RecruitmentStartedInWindow += row.Recruitment
	}
	return summary, activity, nil
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
