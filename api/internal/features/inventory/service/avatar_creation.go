package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/converter"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type AvatarConverter interface {
	Convert(context.Context, io.ReadSeeker, string) (*converter.Asset, error)
}

type AvatarCreator interface {
	Summary(context.Context, uuid.UUID) (domain.Capacity, error)
	Create(context.Context, uuid.UUID, string, string, string) (domain.Avatar, error)
}

type AvatarStore interface {
	Upload(context.Context, io.ReadSeeker, string) (string, error)
	Delete(context.Context, string) error
}

type AvatarConversionService struct {
	repo   AvatarCreator
	store  AvatarStore
	runner AvatarConverter
	config config.AvatarConversionConfig
}

func NewAvatarConversionService(repo AvatarCreator, store AvatarStore, runner AvatarConverter, cfg config.AvatarConversionConfig) *AvatarConversionService {
	return &AvatarConversionService{repo: repo, store: store, runner: runner, config: cfg}
}

func (s *AvatarConversionService) Create(ctx context.Context, user authdomain.User, source io.ReadSeeker, size int64, metadata domain.CreateAvatarInput) (domain.Avatar, error) {
	if err := authorize(user); err != nil {
		return domain.Avatar{}, err
	}
	if err := metadata.Validate(); err != nil {
		return domain.Avatar{}, err
	}
	if !s.config.Enabled {
		return domain.Avatar{}, apperror.New(apperror.CodeAvatarConverterUnavailable, "avatar conversion is disabled")
	}
	if size > s.config.MaxInputBytes {
		return domain.Avatar{}, apperror.New(apperror.CodeAvatarSourceTooLarge, "avatar GLB exceeds the upload limit")
	}
	if _, err := converter.Validate(source, size, false); err != nil {
		return domain.Avatar{}, apperror.New(apperror.CodeAvatarSourceUnsupported, "a supported Avaturn GLB with lip-sync is required")
	}
	capacity, err := s.repo.Summary(ctx, user.ID)
	if err != nil {
		return domain.Avatar{}, err
	}
	if capacity.IsFull {
		return domain.Avatar{}, apperror.New(apperror.CodeInventoryFull, "avatar inventory is full")
	}
	asset, err := s.runner.Convert(ctx, source, metadata.Name)
	if err != nil {
		if errors.Is(err, converter.ErrUnavailable) {
			return domain.Avatar{}, apperror.Wrap(apperror.CodeAvatarConverterUnavailable, "avatar converter is busy", err)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return domain.Avatar{}, apperror.Wrap(apperror.CodeAvatarConversionTimeout, "avatar conversion timed out", err)
		}
		return domain.Avatar{}, apperror.Wrap(apperror.CodeAvatarConversionFailed, "avatar conversion failed", err)
	}
	defer func() { _ = asset.Close() }()
	key, err := s.store.Upload(ctx, asset.File, "model/gltf-binary")
	if err != nil {
		return domain.Avatar{}, apperror.Wrap(apperror.CodeAvatarPersistenceFailed, "unable to store converted avatar", err)
	}
	avatar, err := s.repo.Create(ctx, user.ID, metadata.Name, metadata.Personality, key)
	if err != nil {
		// The request may already be cancelled; cleanup still gets a short deadline.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if cleanupErr := s.store.Delete(cleanupCtx, key); cleanupErr != nil {
			slog.ErrorContext(cleanupCtx, "unable to delete unreferenced avatar object", "key", key, "error", cleanupErr)
		}
		return domain.Avatar{}, err
	}
	return avatar, nil
}
