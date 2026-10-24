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
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
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
// set untouched.
func (r *Repository) ReplaceWeights(ctx context.Context, recruiterID, postingID uuid.UUID, weights []domain.Weight) ([]domain.Weight, error) {
	var saved []domain.Weight
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		status, err := q.LockEditablePosting(ctx, LockEditablePostingParams{ID: postingID, RecruiterID: recruiterID})
		if err != nil {
			return persistenceError(err)
		}
		if !domain.EditableStatus(status) {
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
		saved, err = listWeights(ctx, q, postingID)
		return err
	})
	if err != nil {
		// Errors from inside are already AppErrors; this maps a failed begin or commit.
		return nil, persistenceError(err)
	}
	return saved, nil
}

// ClearWeights deletes the posting's whole weight set under the posting's row lock, so the posting
// falls back to the equal-split defaults.
func (r *Repository) ClearWeights(ctx context.Context, recruiterID, postingID uuid.UUID) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		status, err := q.LockEditablePosting(ctx, LockEditablePostingParams{ID: postingID, RecruiterID: recruiterID})
		if err != nil {
			return persistenceError(err)
		}
		if !domain.EditableStatus(status) {
			return domain.ErrWeightsLocked
		}
		if err := q.DeletePostingWeights(ctx, postingID); err != nil {
			return persistenceError(err)
		}
		return nil
	})
	if err != nil {
		// Errors from inside are already AppErrors; this maps a failed begin or commit.
		return persistenceError(err)
	}
	return nil
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
