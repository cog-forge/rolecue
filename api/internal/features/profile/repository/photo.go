package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/profile/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/uploadgrant"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repository) RecordPhotoUpload(ctx context.Context, owner, key uuid.UUID) error {
	if err := uploadgrant.Record(ctx, r.db, owner, key, uploadgrant.PurposeProfilePhoto); err != nil {
		return profileError(err)
	}
	return nil
}

func (r *Repository) HasPhotoUpload(ctx context.Context, owner, key uuid.UUID, issuedAfter time.Time) (bool, error) {
	found, err := r.queries.HasPhotoUpload(ctx, HasPhotoUploadParams{
		OwnerID: owner, Key: key, IssuedAfter: pgtype.Timestamptz{Time: issuedAfter, Valid: true},
	})
	if err != nil {
		return false, profileError(err)
	}
	return found, nil
}

func (r *Repository) ListExpiredPhotoUploads(ctx context.Context, issuedBefore time.Time) ([]uuid.UUID, error) {
	return r.queries.ListExpiredPhotoUploads(ctx, pgtype.Timestamptz{Time: issuedBefore, Valid: true})
}

func (r *Repository) RemovePhotoUpload(ctx context.Context, key uuid.UUID) error {
	return r.queries.RemovePhotoUpload(ctx, key)
}

func (r *Repository) OwnsProfilePhoto(ctx context.Context, owner, key uuid.UUID) (bool, error) {
	found, err := r.queries.OwnsProfilePhoto(ctx, OwnsProfilePhotoParams{OwnerID: owner, Key: key})
	if err != nil {
		return false, profileError(err)
	}
	return found, nil
}

// UpdatePhoto serializes replacements and consumes the grant in the same transaction as the profile PATCH.
// If the commit result is uncertain, updated is populated so the caller will keep the new object.
func (r *Repository) UpdatePhoto(ctx context.Context, owner uuid.UUID, role string, patch domain.Patch, grant uuid.UUID, issuedAfter time.Time) (updated domain.Profile, previous uuid.UUID, err error) {
	beginner, ok := r.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return updated, previous, apperror.New(apperror.CodeInternal, "profile storage cannot start a transaction")
	}
	err = pgx.BeginFunc(ctx, beginner, func(tx pgx.Tx) error {
		repo := NewRepository(tx)
		current, err := repo.queries.LockProfilePhoto(ctx, owner)
		if err != nil {
			return profileError(err)
		}
		if grant != uuid.Nil {
			count, err := repo.queries.ConsumePhotoUpload(ctx, ConsumePhotoUploadParams{
				OwnerID: owner, Key: grant, IssuedAfter: pgtype.Timestamptz{Time: issuedAfter, Valid: true},
			})
			if err != nil {
				return profileError(err)
			}
			if count == 0 {
				return domain.ErrPhotoUploadUnavailable
			}
		}
		updated, err = repo.Update(ctx, owner, role, patch)
		if current != nil {
			previous = *current
		}
		return err
	})
	var appErr *apperror.AppError
	if err != nil && !errors.As(err, &appErr) {
		err = profileError(err)
	}
	return updated, previous, err
}
