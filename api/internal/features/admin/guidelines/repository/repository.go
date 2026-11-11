package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/admin/guidelines/domain"
	interviewdomain "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository stores the append-only versions of the Interview Behavior Guidelines.
type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
}

// Current returns the highest version, or false when nothing was saved yet.
func (r *Repository) Current(ctx context.Context) (domain.Stored, bool, error) {
	row, err := r.queries.GetCurrentGuideline(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Stored{}, false, nil
	}
	if err != nil {
		return domain.Stored{}, false, persistenceError(err)
	}
	return domain.Stored{
		ID: row.ID, Version: row.Version, UpdatedBy: row.UpdatedBy, UpdatedByName: row.UpdatedByName, UpdatedAt: row.CreatedAt.Time,
		Values: valuesOf(row.Tone, row.QuestionLength, row.FollowUpMax, row.FollowUpFocus, row.AvoidTopics),
	}, true, nil
}

// Save appends a version after expectedVersion. Saves are serialised by a transaction-scoped lock, so the
// version read under it is the one the insert follows; a different current version is a conflict and writes
// nothing. The first save expects version 0.
func (r *Repository) Save(ctx context.Context, adminID uuid.UUID, expectedVersion int32, values interviewdomain.BehaviorGuideline) (domain.Stored, error) {
	var saved domain.Stored
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		if err := q.LockGuidelineSaves(ctx); err != nil {
			return err
		}
		current, err := q.CurrentGuidelineVersion(ctx)
		if err != nil {
			return err
		}
		if current != expectedVersion {
			return apperror.New(apperror.CodeGuidelineVersionConflict, "the guidelines changed since they were loaded; reload and try again")
		}
		var max *int16
		if values.FollowUpMax != nil {
			v := int16(*values.FollowUpMax)
			max = &v
		}
		row, err := q.InsertGuideline(ctx, InsertGuidelineParams{
			Version: current + 1, Tone: values.Tone, QuestionLength: values.QuestionLength, FollowUpMax: max,
			FollowUpFocus: values.FollowUpFocus, AvoidTopics: values.AvoidTopics, UpdatedBy: adminID,
		})
		if err != nil {
			return err
		}
		name, err := q.GetGuidelineAuthorName(ctx, adminID)
		if err != nil {
			return err
		}
		saved = domain.Stored{
			UpdatedByName: name, ID: row.ID, Version: row.Version, UpdatedBy: row.UpdatedBy, UpdatedAt: row.CreatedAt.Time,
			Values: valuesOf(row.Tone, row.QuestionLength, row.FollowUpMax, row.FollowUpFocus, row.AvoidTopics),
		}
		return nil
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// The unique version index is the backstop of the lock: a concurrent save won.
			return domain.Stored{}, apperror.New(apperror.CodeGuidelineVersionConflict, "the guidelines changed since they were loaded; reload and try again")
		}
		return domain.Stored{}, persistenceError(err)
	}
	return saved, nil
}

func valuesOf(tone, length string, max *int16, focus string, topics []string) interviewdomain.BehaviorGuideline {
	values := interviewdomain.BehaviorGuideline{Tone: tone, QuestionLength: length, FollowUpFocus: focus, AvoidTopics: topics}
	if values.AvoidTopics == nil {
		values.AvoidTopics = []string{}
	}
	if max != nil {
		v := int32(*max)
		values.FollowUpMax = &v
	}
	return values
}

func persistenceError(err error) error {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.Wrap(apperror.CodeInternal, "interview guidelines persistence failed", err)
}
