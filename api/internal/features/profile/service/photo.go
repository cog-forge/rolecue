package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/profile/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

const (
	maxPhotoBytes     = 5 << 20
	maxPhotoDimension = 4096
	photoUploadTTL    = 5 * time.Minute
)

type photoStore interface {
	PresignPut(context.Context, string, int64, time.Duration) (storage.UploadGrant, error)
	Download(context.Context, string) (storage.Object, error)
	Upload(context.Context, io.ReadSeeker, string) (string, error)
	Delete(context.Context, string) error
}

type photoRepository interface {
	RecordPhotoUpload(context.Context, uuid.UUID, uuid.UUID) error
	HasPhotoUpload(context.Context, uuid.UUID, uuid.UUID, time.Time) (bool, error)
	ListExpiredPhotoUploads(context.Context, time.Time) ([]uuid.UUID, error)
	RemovePhotoUpload(context.Context, uuid.UUID) error
	OwnsProfilePhoto(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	UpdatePhoto(context.Context, uuid.UUID, string, domain.Patch, uuid.UUID, time.Time) (domain.Profile, uuid.UUID, error)
}

func WithPhotoUploads(repo photoRepository, store photoStore) Option {
	return func(s *Service) {
		s.photos = repo
		s.photoStore = store
	}
}

func validPhotoType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func invalidPhoto() error {
	return apperror.New(apperror.CodeInvalidProfilePhoto, "Choose a valid JPEG, PNG, or WebP up to 5 MiB and 4096 × 4096 pixels.")
}

func photoStorageError(err error) error {
	return apperror.Wrap(apperror.CodeInternal, "Could not access profile photo storage.", err)
}

func (s *Service) CreatePhotoUpload(ctx context.Context, owner uuid.UUID, input domain.PhotoUploadRequest) (domain.PhotoUpload, error) {
	if !validPhotoType(input.ContentType) || input.SizeBytes <= 0 || input.SizeBytes > maxPhotoBytes {
		return domain.PhotoUpload{}, invalidPhoto()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	s.removeExpiredPhotoUploads(ctx)
	grant, err := s.photoStore.PresignPut(ctx, input.ContentType, input.SizeBytes, photoUploadTTL)
	if err != nil {
		return domain.PhotoUpload{}, photoStorageError(err)
	}
	key, err := uuid.Parse(grant.Key)
	if err != nil {
		return domain.PhotoUpload{}, photoStorageError(err)
	}
	if err := s.photos.RecordPhotoUpload(ctx, owner, key); err != nil {
		return domain.PhotoUpload{}, err
	}
	return domain.PhotoUpload{UploadID: key, Request: grant.Request}, nil
}

func (s *Service) attachPhoto(ctx context.Context, owner uuid.UUID, role string, patch domain.Patch) (domain.Profile, error) {
	if patch.PhotoUploadID.Value == nil || s.photos == nil {
		return domain.Profile{}, domain.ErrPhotoUploadUnavailable
	}
	grant, err := uuid.Parse(*patch.PhotoUploadID.Value)
	if err != nil || grant == uuid.Nil {
		return domain.Profile{}, domain.ErrPhotoUploadUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	issuedAfter := time.Now().Add(-photoUploadTTL)
	found, err := s.photos.HasPhotoUpload(ctx, owner, grant, issuedAfter)
	if err != nil {
		return domain.Profile{}, err
	}
	if !found {
		return domain.Profile{}, domain.ErrPhotoUploadUnavailable
	}
	photo, err := s.photoStore.Download(ctx, grant.String())
	if errors.Is(err, storage.ErrNotFound) {
		return domain.Profile{}, domain.ErrPhotoUploadUnavailable
	}
	if err != nil {
		return domain.Profile{}, photoStorageError(err)
	}
	data, err := verifiedPhoto(photo)
	if err != nil {
		return domain.Profile{}, err
	}
	// The client can still overwrite the staging object while its PUT is valid.
	// Publish the verified bytes under a fresh server-only key.
	stored, err := s.photoStore.Upload(ctx, bytes.NewReader(data), photo.ContentType)
	if err != nil {
		return domain.Profile{}, photoStorageError(err)
	}
	key, err := uuid.Parse(stored)
	if err != nil {
		return domain.Profile{}, photoStorageError(err)
	}
	path := domain.PhotoPath(owner, key)
	patch.Image = domain.StringChange{Present: true, Value: &path}
	patch.PhotoObjectKey = key
	return s.replacePhoto(ctx, owner, role, patch, grant, key, issuedAfter)
}

func verifiedPhoto(photo storage.Object) ([]byte, error) {
	defer func() { _ = photo.Body.Close() }()
	if !validPhotoType(photo.ContentType) || photo.Size <= 0 || photo.Size > maxPhotoBytes {
		return nil, invalidPhoto()
	}
	data, err := io.ReadAll(io.LimitReader(photo.Body, maxPhotoBytes+1))
	if err != nil {
		return nil, photoStorageError(err)
	}
	if int64(len(data)) != photo.Size || http.DetectContentType(data) != photo.ContentType {
		return nil, invalidPhoto()
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > maxPhotoDimension || config.Height > maxPhotoDimension {
		return nil, invalidPhoto()
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return nil, invalidPhoto()
	}
	return data, nil
}

func (s *Service) replacePhoto(ctx context.Context, owner uuid.UUID, role string, patch domain.Patch, grant, key uuid.UUID, issuedAfter time.Time) (domain.Profile, error) {
	updated, previous, err := s.photos.UpdatePhoto(ctx, owner, role, patch, grant, issuedAfter)
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err != nil {
		if updated.ID == uuid.Nil {
			s.deletePhoto(cleanup, key)
		}
		return domain.Profile{}, err
	}
	s.deletePhoto(cleanup, previous)
	s.deletePhoto(cleanup, grant)
	return updated, nil
}

func (s *Service) ReadPhoto(ctx context.Context, owner, key uuid.UUID) (storage.Object, error) {
	found, err := s.photos.OwnsProfilePhoto(ctx, owner, key)
	if err != nil {
		return storage.Object{}, err
	}
	notFound := apperror.New(apperror.CodeAccountNotFound, "profile photo not found")
	if !found {
		return storage.Object{}, notFound
	}
	object, err := s.photoStore.Download(ctx, key.String())
	if errors.Is(err, storage.ErrNotFound) {
		return storage.Object{}, notFound
	}
	if err != nil {
		return storage.Object{}, photoStorageError(err)
	}
	if !validPhotoType(object.ContentType) || object.Size <= 0 || object.Size > maxPhotoBytes {
		_ = object.Body.Close()
		return storage.Object{}, notFound
	}
	return object, nil
}

func (s *Service) removeExpiredPhotoUploads(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	// Keep grants past their PUT expiry so completed or abandoned staging objects can be swept.
	keys, err := s.photos.ListExpiredPhotoUploads(ctx, time.Now().Add(-3*photoUploadTTL))
	if err != nil {
		logger.FromContext(ctx).Warn("could not list expired profile photo uploads", logger.Error(err))
		return
	}
	for _, key := range keys {
		if s.deletePhoto(ctx, key) {
			if err := s.photos.RemovePhotoUpload(ctx, key); err != nil {
				logger.FromContext(ctx).Warn("could not remove expired profile photo grant", logger.Error(err))
			}
		}
	}
}

func (s *Service) deletePhoto(ctx context.Context, key uuid.UUID) bool {
	if key == uuid.Nil {
		return true
	}
	if err := s.photoStore.Delete(ctx, key.String()); err != nil && !errors.Is(err, storage.ErrNotFound) {
		logger.FromContext(ctx).Warn("could not delete profile photo object", logger.String("key", key.String()), logger.Error(err))
		return false
	}
	return true
}
