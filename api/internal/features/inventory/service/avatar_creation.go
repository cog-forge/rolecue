package service

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/converter"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AvatarConverter interface {
	Convert(context.Context, io.ReadSeeker, string) (*converter.Asset, error)
}

type AvatarCreator interface {
	Summary(context.Context, uuid.UUID) (domain.Capacity, error)
	Create(context.Context, uuid.UUID, string, string, string) (domain.Avatar, error)
	CountRecentAvatars(context.Context, uuid.UUID, time.Time) (int64, error)
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
	gate   *conversionGate
}

func NewAvatarConversionService(repo AvatarCreator, store AvatarStore, runner AvatarConverter, cfg config.AvatarConversionConfig) *AvatarConversionService {
	return &AvatarConversionService{repo: repo, store: store, runner: runner, config: cfg, gate: newConversionGate(cfg.UserMaxInflight, cfg.UserDailyQuota)}
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
	// Reserve before the expensive work: the per-user daily quota and in-flight limit are checked
	// and taken together, then the free slot is read again so each running conversion keeps one
	// slot for itself. Without the reservation, parallel uploads from one user all pass the check above.
	var persisted int64
	if s.config.UserDailyQuota > 0 {
		if persisted, err = s.repo.CountRecentAvatars(ctx, user.ID, time.Now().Add(-conversionQuotaWindow)); err != nil {
			return domain.Avatar{}, err
		}
	}
	reserved, err := s.gate.acquire(user.ID, persisted)
	if err != nil {
		return domain.Avatar{}, err
	}
	defer reserved.release()
	if capacity, err = s.repo.Summary(ctx, user.ID); err != nil {
		return domain.Avatar{}, err
	}
	if capacity.IsFull || (reserved.others > 0 && capacity.Available <= int64(reserved.others)) {
		return domain.Avatar{}, apperror.New(apperror.CodeInventoryFull, "avatar inventory is full")
	}
	done := logger.Start(ctx, "inventory.avatar_convert", logger.String("user_id", user.ID.String()), zap.Int64("bytes", size))
	asset, err := s.runner.Convert(ctx, source, metadata.Name)
	done(err)
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
			logger.FromContext(cleanupCtx).Error("unable to delete unreferenced avatar object",
				logger.Op("inventory.avatar_cleanup"),
				logger.String("key", key),
				logger.Cause(cleanupErr),
			)
		}
		return domain.Avatar{}, err
	}
	logger.FromContext(ctx).Info("avatar created",
		logger.Op("inventory.avatar_create"),
		logger.String("avatar_id", avatar.ID.String()),
		logger.String("user_id", user.ID.String()),
		zap.Int64("bytes", size),
	)
	return avatar, nil
}
