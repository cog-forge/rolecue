package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository reads the metrics catalog and reads, replaces or clears a Recruiter's posting weights.
type Repository struct {
	pool          *pgxpool.Pool
	queries       *Queries
	questionLimit int
}

// Option configures a Repository.
type Option func(*Repository)

// WithQuestionLimit sets how many core questions one interview session asks, the limit the coverage view of
// a weight change uses. It must be the value the interview start uses (the default applies when it is not
// set or not positive).
func WithQuestionLimit(limit int) Option {
	return func(r *Repository) { r.questionLimit = domain.ResolveQuestionLimit(limit) }
}

func NewRepository(pool *pgxpool.Pool, options ...Option) *Repository {
	r := &Repository{pool: pool, queries: New(pool), questionLimit: domain.DefaultQuestionLimit}
	for _, option := range options {
		option(r)
	}
	return r
}

// Coverage reports whether the bank of the posting can cover every metric active under its saved weights
// within the session question limit (KAN-377). The caller has already checked the posting is the
// Recruiter's own.
func (r *Repository) Coverage(ctx context.Context, postingID uuid.UUID) (domain.CoverageReport, error) {
	return PostingCoverage(ctx, r.pool, postingID, r.questionLimit)
}

func (r *Repository) ListMetrics(ctx context.Context) ([]domain.Metric, error) {
	rows, err := r.queries.ListMetrics(ctx)
	if err != nil {
		return nil, persistenceError(err)
	}
	out := make([]domain.Metric, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Metric{ID: row.ID, Name: row.Name})
	}
	return out, nil
}

// PostingStatus returns the status of a posting the Recruiter owns. Someone else's posting is
// reported as not found.
func (r *Repository) PostingStatus(ctx context.Context, recruiterID, postingID uuid.UUID) (string, error) {
	status, err := r.queries.GetRecruiterPostingStatus(ctx, GetRecruiterPostingStatusParams{ID: postingID, RecruiterID: recruiterID})
	if err != nil {
		return "", persistenceError(err)
	}
	return status, nil
}

func (r *Repository) ListWeights(ctx context.Context, postingID uuid.UUID) ([]domain.Weight, error) {
	return listWeights(ctx, r.queries, postingID)
}

// ReplaceWeights swaps the whole weight set in one transaction, so a failure leaves the previous
// set untouched. It also reports whether the change took the posting out of the Admin queue: when the posting
// is pending and submitted and the new weights leave its bank unable to cover every active metric, the
// submission is withdrawn in the same transaction, under the same posting lock (KAN-377).
func (r *Repository) ReplaceWeights(ctx context.Context, recruiterID, postingID uuid.UUID, weights []domain.Weight) ([]domain.Weight, bool, error) {
	var saved []domain.Weight
	withdrawn := false
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		locked, err := q.LockEditablePosting(ctx, LockEditablePostingParams{ID: postingID, RecruiterID: recruiterID})
		if err != nil {
			return persistenceError(err)
		}
		if !domain.EditableStatus(locked.Status) {
			return domain.ErrWeightsLocked
		}
		if err := q.DeletePostingWeights(ctx, postingID); err != nil {
			return persistenceError(err)
		}
		for _, weight := range weights {
			if err := q.InsertPostingWeight(ctx, InsertPostingWeightParams{
				JobPostingID: postingID, MetricID: weight.MetricID, Percentage: weight.Percentage,
			}); err != nil {
				return persistenceError(err)
			}
		}
		if withdrawn, err = r.withdrawIfUncovered(ctx, tx, q, postingID, locked); err != nil {
			return err
		}
		saved, err = listWeights(ctx, q, postingID)
		return err
	})
	if err != nil {
		// Errors from inside are already AppErrors; this maps a failed begin or commit.
		return nil, false, persistenceError(err)
	}
	return saved, withdrawn, nil
}

// ClearWeights deletes the posting's whole weight set under the posting's row lock, so the posting
// falls back to the equal-split defaults. Like ReplaceWeights it reports whether the posting left the Admin
// queue because the whole catalog is active again and the bank no longer covers it.
func (r *Repository) ClearWeights(ctx context.Context, recruiterID, postingID uuid.UUID) (bool, error) {
	withdrawn := false
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		locked, err := q.LockEditablePosting(ctx, LockEditablePostingParams{ID: postingID, RecruiterID: recruiterID})
		if err != nil {
			return persistenceError(err)
		}
		if !domain.EditableStatus(locked.Status) {
			return domain.ErrWeightsLocked
		}
		if err := q.DeletePostingWeights(ctx, postingID); err != nil {
			return persistenceError(err)
		}
		withdrawn, err = r.withdrawIfUncovered(ctx, tx, q, postingID, locked)
		return err
	})
	if err != nil {
		// Errors from inside are already AppErrors; this maps a failed begin or commit.
		return false, persistenceError(err)
	}
	return withdrawn, nil
}

// withdrawIfUncovered takes a submitted pending posting out of the Admin queue when its bank no longer covers
// every metric active under the weights just written. A bank that still covers keeps its submission: not
// every weight change is a change of what the Admin reviewed. Called under the posting lock, after the
// weights were written, in the transaction of the change.
func (r *Repository) withdrawIfUncovered(ctx context.Context, tx pgx.Tx, q *Queries, postingID uuid.UUID, locked LockEditablePostingRow) (bool, error) {
	if locked.Status != "pending" || !locked.ReviewSubmitted {
		return false, nil
	}
	report, err := PostingCoverage(ctx, tx, postingID, r.questionLimit)
	if err != nil {
		return false, err
	}
	if report.Passes {
		return false, nil
	}
	n, err := q.WithdrawSubmission(ctx, postingID)
	if err != nil {
		return false, persistenceError(err)
	}
	return n > 0, nil
}

func listWeights(ctx context.Context, q *Queries, postingID uuid.UUID) ([]domain.Weight, error) {
	rows, err := q.ListPostingWeights(ctx, postingID)
	if err != nil {
		return nil, persistenceError(err)
	}
	out := make([]domain.Weight, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Weight{MetricID: row.MetricID, Name: row.Name, Percentage: row.Percentage})
	}
	return out, nil
}

// persistenceError maps a missing or foreign posting to not found and a metric deleted between
// validation and insert to an unknown metric.
func persistenceError(err error) error {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.Wrap(apperror.CodeJobPostingNotFound, "job posting not found", err)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return apperror.Wrap(apperror.CodeEvaluationMetricUnknown, "unknown evaluation metric", err)
	}
	return apperror.Wrap(apperror.CodeInternal, "evaluation weights persistence failed", err)
}
