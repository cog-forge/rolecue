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
	"github.com/jackc/pgx/v5/pgconn"
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

// eraseAttempts bounds the reruns of the erasure transaction after a deadlock or serialization failure. A
// seller's erasure can cross a purchase of one of the seller's listings (checkout locks the listing and then
// the seller's wallet; the erasure deletes the listing under the user row lock), and the database picks one
// victim. Nothing outside the transaction has happened yet, so a rerun is safe.
const eraseAttempts = 3

// Erase deletes the account and everything it owns in one transaction and returns the current photo and orphaned
// avatar model keys for post-commit removal. The caller checks model references before deleting those objects.
// It locks the user row and checks again that no work is active, so a posting
// that went live after the first check is refused (ErrActiveWork). An account that is already gone is not an
// error, which makes a retry after a lost response safe.
func (r *Repository) Erase(ctx context.Context, id uuid.UUID) ([]string, error) {
	for attempt := 1; ; attempt++ {
		keys, err := r.eraseOnce(ctx, id)
		if err == nil || attempt >= eraseAttempts || !retryable(err) {
			return keys, err
		}
	}
}

// retryable reports a PostgreSQL deadlock (40P01) or serialization failure (40001) anywhere in the chain.
func retryable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "40P01" || pgErr.Code == "40001")
}

func (r *Repository) eraseOnce(ctx context.Context, id uuid.UUID) (keys []string, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "could not start the erasure", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	q := r.queries.WithTx(tx)
	account, scanErr := q.LockAccountForErasure(ctx, id)
	switch {
	case errors.Is(scanErr, pgx.ErrNoRows):
		return nil, tx.Commit(ctx)
	case scanErr != nil:
		return nil, apperror.Wrap(apperror.CodeInternal, "could not lock the account", scanErr)
	}
	work, err := activeWork(ctx, q, id)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "could not check the account's open work", err)
	}
	if !work.None() {
		return nil, ErrActiveWork
	}
	if keys, err = erase(ctx, q, id, account.Email); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "could not erase the account", err)
	}
	if account.ProfilePhotoKey != nil {
		keys = append(keys, account.ProfilePhotoKey.String())
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "could not erase the account", err)
	}
	return keys, nil
}

// ModelKeyInUse reports whether any asset still stores the object key. The service asks it after the erasure
// committed, right before it deletes an avatar model object, so an object that a purchase or an upload
// referenced in the meantime is kept.
func (r *Repository) ModelKeyInUse(ctx context.Context, key string) (bool, error) {
	used, err := r.queries.ModelKeyInUse(ctx, key)
	if err != nil {
		return false, apperror.Wrap(apperror.CodeInternal, "could not check the avatar model", err)
	}
	return used, nil
}

// erase runs the statements in the order the foreign keys need; q is bound to the erasure transaction. It
// returns the model keys of the asset rows it deleted.
func erase(ctx context.Context, q *Queries, id uuid.UUID, email string) (modelKeys []string, err error) {
	interviews, err := q.ListOwnedInterviewIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	jds, err := q.ListOwnedJobDescriptionIDs(ctx, id)
	if err != nil {
		return nil, err
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
		func() error {
			// Listings, then avatars, then the assets (the order checkout takes its locks in). Which assets are
			// orphans is decided here, under the asset locks, and only from the rows this transaction deleted.
			listed, err := q.DeleteOwnedListings(ctx, id)
			if err != nil {
				return err
			}
			held, err := q.DeleteOwnedAvatars(ctx, id)
			if err != nil {
				return err
			}
			candidates := append(listed, held...)
			if err := q.LockCandidateAssets(ctx, LockCandidateAssetsParams{AssetIds: candidates, ID: &id}); err != nil {
				return err
			}
			modelKeys, err = q.DeleteOrphanAssets(ctx, DeleteOrphanAssetsParams{AssetIds: candidates, ID: &id})
			return err
		},
		func() error { return q.DeleteOwnedInventories(ctx, id) },
		func() error { return q.DeleteOwnedWallets(ctx, id) },
		func() error {
			return q.DeleteOwnedVerifications(ctx, DeleteOwnedVerificationsParams{ID: &id, Email: email})
		},
		func() error { return q.DeleteAccount(ctx, id) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}
	return modelKeys, nil
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
