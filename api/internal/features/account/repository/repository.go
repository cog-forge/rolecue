// Package repository reads and deletes the data an account owns. The erasure touches most tables of the
// schema and an export joins several of them; the SQL lives in query/*.sql and the generated code is
// *.sql.go (make sqlc).
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/account/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrActiveWork is returned by Erase when the recheck inside the transaction finds new active work.
var ErrActiveWork = errors.New("account has active work")

// Repository takes the pool, not just DBTX, because the erasure runs many statements in one transaction.
type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
}

// Account returns the identity row, or ErrNotFound.
func (r *Repository) Account(ctx context.Context, id uuid.UUID) (domain.Identity, error) {
	row, err := r.queries.GetAccount(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Identity{}, apperror.New(apperror.CodeAccountNotFound, "account not found")
	}
	if err != nil {
		return domain.Identity{}, apperror.Wrap(apperror.CodeInternal, "could not read the account", err)
	}
	return domain.Identity{ID: row.ID, Email: row.Email, Role: row.Role}, nil
}

func activeWork(ctx context.Context, q *Queries, id uuid.UUID) (domain.ErasureBlockers, error) {
	row, err := q.CountActiveWork(ctx, id)
	if err != nil {
		return domain.ErasureBlockers{}, err
	}
	return domain.ErasureBlockers{OpenPostings: row.OpenPostings, ApplicationsInProgress: row.ApplicationsInProgress}, nil
}

// ActiveWork counts the live Job Postings and every Application on a Recruiter's postings, decided or not: erasing
// the account would delete those Candidates' CVs, recordings and history, so it is refused until a person decides.
// (Historical name: ApplicationsInProgress.) A Candidate has none.
func (r *Repository) ActiveWork(ctx context.Context, id uuid.UUID) (domain.ErasureBlockers, error) {
	w, err := activeWork(ctx, r.queries, id)
	if err != nil {
		return domain.ErasureBlockers{}, apperror.Wrap(apperror.CodeInternal, "could not check the account's open work", err)
	}
	return w, nil
}

// StorageKeys lists every object key the account owns: CVs, recordings, avatar models, JD PDFs and the
// keys of issued upload grants (which also covers uploads never attached to anything). Duplicates and
// empty values are removed.
func (r *Repository) StorageKeys(ctx context.Context, id uuid.UUID) ([]string, error) {
	rows, err := r.queries.ListStorageKeys(ctx, id)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "could not list the account's stored files", err)
	}
	var keys []string
	for _, key := range rows {
		if key != nil && *key != "" {
			keys = append(keys, *key)
		}
	}
	return keys, nil
}

// Erase deletes the account and everything it owns in one transaction. The caller has already removed
// the stored objects. It locks the user row and checks again that no work is active, so a posting
// that went live after the first check is refused (ErrActiveWork). An account that is already gone
// is not an error, which makes a retry after a lost response safe.
func (r *Repository) Erase(ctx context.Context, id uuid.UUID) (err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "could not start the erasure", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	q := r.queries.WithTx(tx)
	email, scanErr := q.LockAccountEmail(ctx, id)
	switch {
	case errors.Is(scanErr, pgx.ErrNoRows):
		return tx.Commit(ctx)
	case scanErr != nil:
		return apperror.Wrap(apperror.CodeInternal, "could not lock the account", scanErr)
	}
	work, err := activeWork(ctx, q, id)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "could not check the account's open work", err)
	}
	if !work.None() {
		return ErrActiveWork
	}
	if err = erase(ctx, q, id, email); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "could not erase the account", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "could not erase the account", err)
	}
	return nil
}

// erase runs the statements in the order the foreign keys need; q is bound to the erasure transaction.
func erase(ctx context.Context, q *Queries, id uuid.UUID, email string) error {
	interviews, err := q.ListOwnedInterviewIDs(ctx, id)
	if err != nil {
		return err
	}
	jds, err := q.ListOwnedJobDescriptionIDs(ctx, id)
	if err != nil {
		return err
	}
	tombstone := domain.Tombstone
	steps := []func() error{
		func() error { return q.KeepUsedInterviewSlots(ctx, id) },
		func() error {
			return q.TombstoneConversationTurns(ctx, TombstoneConversationTurnsParams{Tombstone: tombstone, InterviewIds: interviews})
		},
		func() error {
			return q.TombstoneCriterionScores(ctx, TombstoneCriterionScoresParams{Tombstone: tombstone, InterviewIds: interviews})
		},
		func() error { return q.DeleteCriterionScores(ctx, interviews) },
		func() error { return q.DeleteScoreDetails(ctx, interviews) },
		func() error { return q.DeleteRoadmapAssessments(ctx, interviews) },
		func() error { return q.DeleteInterviewQuestions(ctx, interviews) },
		func() error { return q.DeleteConversationTurns(ctx, interviews) },
		func() error { return q.DeleteInterviews(ctx, interviews) },
		func() error { return q.DetachInterviewAvatars(ctx, id) },
		func() error { return q.DetachPostingRequiredAvatars(ctx, id) },
		func() error { return q.DeleteOwnedApplications(ctx, id) },
		func() error { return q.DeleteOwnedPostingMetrics(ctx, id) },
		func() error { return q.DeleteOwnedJobPostings(ctx, id) },
		func() error { return q.DeleteOwnedRoadmaps(ctx, id) },
		func() error { return q.DeleteOwnedRoleProfiles(ctx, id) },
		func() error { return q.DeleteCoreQuestions(ctx, jds) },
		func() error { return q.DeleteRequirements(ctx, jds) },
		func() error { return q.DeleteJobDescriptions(ctx, jds) },
		func() error { return q.DeleteOwnedAvatars(ctx, id) },
		func() error { return q.DeleteOwnedInventories(ctx, id) },
		func() error { return q.DeleteOwnedWallets(ctx, id) },
		func() error {
			return q.DeleteOwnedVerifications(ctx, DeleteOwnedVerificationsParams{ID: &id, Email: email})
		},
		func() error { return q.DeleteAccount(ctx, id) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
