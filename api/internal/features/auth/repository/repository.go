package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ queries *Queries }

func NewRepository(db DBTX) *Repository { return &Repository{queries: New(db)} }

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.queries.GetAuthUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrInvalidSession
	}
	if err != nil {
		return domain.User{}, domain.ErrUnavailable
	}
	return domain.User{ID: row.ID, Email: row.Email, FullName: row.Name, Role: row.Role,
		EmailVerified: row.EmailVerified, IsLocked: row.IsLocked, Image: row.Image}, nil
}
