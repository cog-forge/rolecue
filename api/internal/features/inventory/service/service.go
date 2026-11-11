package service

import (
	"context"
	"errors"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type Repository interface {
	Summary(context.Context, uuid.UUID) (domain.Capacity, error)
	List(context.Context, uuid.UUID, int32, int32) ([]domain.Avatar, int64, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (domain.StoredAvatar, error)
	PurchaseSlot(ctx context.Context, owner uuid.UUID, amount int32, idempotencyKey string) (domain.SlotPurchase, error)
	// Delete removes the owner's avatar row and calls purge with its stored model key before the
	// transaction commits; a purge error keeps the row.
	Delete(ctx context.Context, owner, id uuid.UUID, purge func(context.Context, string) error) error
}

// ModelStore is the object storage of the avatar models: signed reads and deletion. storage.Store satisfies it.
type ModelStore interface {
	PresignGet(context.Context, string, time.Duration) (storage.SignedRequest, error)
	Delete(context.Context, string) error
}

const modelDeleteTimeout = 10 * time.Second

type Service struct {
	repository Repository
	signer     ModelStore
	// retryDelays are the waits between attempts to delete a model object: one more attempt than delays.
	retryDelays []time.Duration
}

func NewService(repository Repository, signer ModelStore) *Service {
	return &Service{repository: repository, signer: signer, retryDelays: []time.Duration{200 * time.Millisecond, 600 * time.Millisecond}}
}

func authorize(user authdomain.User) error {
	if user.ID == uuid.Nil || (user.Role != "candidate" && user.Role != "recruiter") || !user.EmailVerified || user.IsLocked {
		return apperror.New(apperror.CodeForbidden, "account is not permitted to access a personal avatar inventory")
	}
	return nil
}

func (s *Service) Summary(ctx context.Context, user authdomain.User) (domain.Capacity, error) {
	if err := authorize(user); err != nil {
		return domain.Capacity{}, err
	}
	return s.repository.Summary(ctx, user.ID)
}

func (s *Service) List(ctx context.Context, user authdomain.User, limit, offset int32) ([]domain.Avatar, int64, error) {
	if err := authorize(user); err != nil {
		return nil, 0, err
	}
	return s.repository.List(ctx, user.ID, limit, offset)
}

func (s *Service) Get(ctx context.Context, user authdomain.User, id uuid.UUID) (domain.AvatarDetail, error) {
	if err := authorize(user); err != nil {
		return domain.AvatarDetail{}, err
	}
	avatar, err := s.repository.Get(ctx, user.ID, id)
	if err != nil {
		return domain.AvatarDetail{}, err
	}
	if s.signer == nil {
		return domain.AvatarDetail{}, apperror.New(apperror.CodeAvatarAssetUnavailable, "avatar asset is temporarily unavailable")
	}
	grant, err := s.signer.PresignGet(ctx, avatar.ModelPath, 5*time.Minute)
	if err != nil {
		return domain.AvatarDetail{}, apperror.Wrap(apperror.CodeAvatarAssetUnavailable, "avatar asset is temporarily unavailable", err)
	}
	if grant.Headers == nil {
		grant.Headers = map[string]string{}
	}
	return domain.AvatarDetail{Avatar: avatar.Avatar, ModelAccess: domain.ModelAccess{
		URL: grant.URL, Method: grant.Method, Headers: grant.Headers, ExpiresAt: grant.ExpiresAt,
	}}, nil
}

// PurchaseSlot buys 1 to MaxSlotsPerPurchase slots at PricePerSlot each. The repository checks the
// balance, debits it and adds the slots in one transaction, so a low balance or a missing wallet is
// INSUFFICIENT_BALANCE and writes nothing.
func (s *Service) PurchaseSlot(ctx context.Context, user authdomain.User, amount int32, idempotencyKey string) (domain.SlotPurchase, error) {
	if err := authorize(user); err != nil {
		return domain.SlotPurchase{}, err
	}
	if amount < 1 || amount > domain.MaxSlotsPerPurchase {
		return domain.SlotPurchase{}, apperror.New(apperror.CodeValidation, "amount must be between 1 and 10")
	}
	purchase, err := s.repository.PurchaseSlot(ctx, user.ID, amount, idempotencyKey)
	if err != nil {
		return domain.SlotPurchase{}, err
	}
	logger.FromContext(ctx).Info("inventory slots purchased",
		logger.Op("inventory.slot_purchase"),
		logger.String("user_id", user.ID.String()),
		logger.Int("count", int(amount)),
		logger.Int("capacity", int(purchase.Capacity.Capacity)),
		logger.Int("balance", int(purchase.Balance)),
	)
	return purchase, nil
}

func (s *Service) Delete(ctx context.Context, user authdomain.User, id uuid.UUID) error {
	if err := authorize(user); err != nil {
		return err
	}
	// The object goes in the same transaction as the row: when storage keeps failing the avatar stays and
	// the owner can delete again, so a row and its personal-data object never disagree silently.
	if err := s.repository.Delete(ctx, user.ID, id, s.deleteModel); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("avatar deleted",
		logger.Op("inventory.avatar_delete"),
		logger.String("avatar_id", id.String()),
		logger.String("user_id", user.ID.String()),
	)
	return nil
}

// deleteModel removes the stored model object, trying again after each of retryDelays. A missing object
// counts as deleted. Logs never carry the object key.
func (s *Service) deleteModel(ctx context.Context, key string) error {
	if s.signer == nil {
		return apperror.New(apperror.CodeAvatarAssetUnavailable, "avatar storage is temporarily unavailable")
	}
	var last error
	for attempt := 0; ; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, modelDeleteTimeout)
		last = s.signer.Delete(attemptCtx, key)
		cancel()
		if last == nil || errors.Is(last, storage.ErrNotFound) {
			return nil
		}
		if attempt >= len(s.retryDelays) || ctx.Err() != nil {
			break
		}
		logger.FromContext(ctx).Warn("avatar model delete failed; retrying",
			logger.Op("inventory.avatar_model_delete"), logger.Int("attempt", attempt+1), logger.Cause(last))
		timer := time.NewTimer(s.retryDelays[attempt])
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	return apperror.Wrap(apperror.CodeAvatarAssetUnavailable, "the avatar model could not be deleted; the avatar was kept, try again", last)
}
