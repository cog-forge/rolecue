package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/profile/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ queries *Queries }

func NewRepository(db DBTX) *Repository { return &Repository{queries: New(db)} }

func toProfile(row GetProfileRow) domain.Profile {
	p := domain.Profile{ID: row.ID, FullName: row.Name, Email: row.Email, Image: row.Image,
		Role: row.Role, EmailVerified: row.EmailVerified,
		CompanyName: row.CompanyName, CompanyWebsite: row.CompanyWebsite,
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
	if p.Role != "recruiter" {
		p.CompanyName = nil
		p.CompanyWebsite = nil
	}
	return p
}
func profileError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.New(apperror.CodeAccountNotFound, "profile no longer exists")
	}
	return apperror.Wrap(apperror.CodeInternal, "unable to load or save profile", err)
}
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domain.Profile, error) {
	row, err := r.queries.GetProfile(ctx, id)
	if err != nil {
		return domain.Profile{}, profileError(err)
	}
	return toProfile(row), nil
}
func (r *Repository) Update(ctx context.Context, id uuid.UUID, role string, patch domain.Patch) (domain.Profile, error) {
	// A single UPDATE prevents partial writes and rechecks role/lock/verification
	// alongside the write, including changes since middleware authenticated.
	row, err := r.queries.UpdateProfile(ctx, UpdateProfileParams{
		ID: id, Role: role,
		SetName: patch.FullName.Present, Name: patch.FullName.Value,
		SetImage: patch.Image.Present, Image: patch.Image.Value,
		SetCompanyName: patch.CompanyName.Present, CompanyName: patch.CompanyName.Value,
		SetCompanyWebsite: patch.CompanyWebsite.Present, CompanyWebsite: patch.CompanyWebsite.Value,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		exists, checkErr := r.queries.ProfileExists(ctx, id)
		if checkErr != nil {
			return domain.Profile{}, profileError(checkErr)
		}
		if !exists {
			return domain.Profile{}, apperror.New(apperror.CodeAccountNotFound, "profile no longer exists")
		}
		return domain.Profile{}, apperror.New(apperror.CodeForbidden, "account is no longer permitted to update this profile")
	}
	if err != nil {
		return domain.Profile{}, profileError(err)
	}
	return toProfile(GetProfileRow(row)), nil
}
