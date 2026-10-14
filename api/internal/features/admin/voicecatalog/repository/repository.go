package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository reads the voices catalog and removes voices. Removal runs in one transaction because
// it checks references and writes based on the result.
type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
}

func (r *Repository) List(ctx context.Context, filter domain.VoiceListFilter) ([]domain.VoiceProfile, int64, error) {
	active, pattern := activeArg(filter.Status), patternArg(filter.Query)
	rows, err := r.queries.ListVoices(ctx, ListVoicesParams{IsActive: active, NamePattern: pattern, PageLimit: filter.Limit, PageOffset: filter.Offset})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	total, err := r.queries.CountVoices(ctx, CountVoicesParams{IsActive: active, NamePattern: pattern})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	out := make([]domain.VoiceProfile, 0, len(rows))
	for _, row := range rows {
		out = append(out, voice(row.ID, row.Name, row.IsActive, row.InterviewCount, row.JobPostingCount))
	}
	return out, total, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domain.VoiceProfile, error) {
	row, err := r.queries.GetVoice(ctx, id)
	if err != nil {
		return domain.VoiceProfile{}, persistenceError(err)
	}
	return voice(row.ID, row.Name, row.IsActive, row.InterviewCount, row.JobPostingCount), nil
}

// Remove deactivates a referenced voice and deletes an unreferenced one. The row lock keeps a new
// interview or posting from referencing the voice between the count and the write; if the delete
// still hits a foreign key, the voice is deactivated instead.
func (r *Repository) Remove(ctx context.Context, id uuid.UUID) (domain.VoiceDeleteOutcome, error) {
	var outcome domain.VoiceDeleteOutcome
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		if _, err := q.LockVoice(ctx, id); err != nil {
			return err
		}
		refs, err := q.CountVoiceReferences(ctx, id)
		if err != nil {
			return err
		}
		if refs > 0 {
			outcome = domain.VoiceDeactivated
			return q.DeactivateVoice(ctx, id)
		}
		outcome = domain.VoiceDeleted
		return q.DeleteVoice(ctx, id)
	})
	// Safety net only: with the row lock held, a referencing insert waits for this transaction,
	// so the count above already sees it and the delete can't hit a foreign key.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		if err := r.queries.DeactivateVoice(ctx, id); err != nil {
			return "", persistenceError(err)
		}
		return domain.VoiceDeactivated, nil
	}
	if err != nil {
		return "", persistenceError(err)
	}
	return outcome, nil
}

// Activate puts a voice back into the catalog. Activating an active voice changes nothing; a
// deleted voice is not found.
func (r *Repository) Activate(ctx context.Context, id uuid.UUID) error {
	rows, err := r.queries.ActivateVoice(ctx, id)
	if err != nil {
		return persistenceError(err)
	}
	if rows == 0 {
		return domain.ErrVoiceNotFound
	}
	return nil
}

func voice(id uuid.UUID, name string, active bool, interviews, postings int64) domain.VoiceProfile {
	return domain.VoiceProfile{ID: id, Name: name, IsActive: active, InterviewCount: interviews, JobPostingCount: postings,
		InUse: interviews > 0 || postings > 0}
}

func activeArg(status domain.VoiceStatusFilter) *bool {
	switch status {
	case domain.VoiceStatusActive:
		v := true
		return &v
	case domain.VoiceStatusInactive:
		v := false
		return &v
	default:
		return nil
	}
}

// patternArg turns the search text into an ILIKE pattern, escaping the wildcard characters so
// "50%" searches for a literal percent sign.
func patternArg(query string) *string {
	if query == "" {
		return nil
	}
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	pattern := "%" + escaped + "%"
	return &pattern
}

func persistenceError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.Wrap(apperror.CodeVoiceNotFound, "voice not found", err)
	}
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.Wrap(apperror.CodeInternal, "voice catalog persistence failed", err)
}
