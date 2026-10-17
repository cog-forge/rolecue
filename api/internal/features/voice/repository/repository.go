package repository

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/voice/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

type Repository struct{ queries *Queries }

func NewRepository(db DBTX) *Repository {
	return &Repository{queries: New(db)}
}

func (r *Repository) ListActive(ctx context.Context, filter domain.ListFilter) ([]domain.Voice, int64, error) {
	rows, err := r.queries.ListActiveVoices(ctx, ListActiveVoicesParams{PageLimit: filter.Limit, PageOffset: filter.Offset})
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeInternal, "voice persistence failed", err)
	}
	total, err := r.queries.CountActiveVoices(ctx)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeInternal, "voice persistence failed", err)
	}
	out := make([]domain.Voice, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Voice{ID: row.ID, Name: row.Name})
	}
	return out, total, nil
}
