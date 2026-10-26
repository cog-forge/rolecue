package service

import (
	"context"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type Repository interface {
	Summary(context.Context, uuid.UUID) (domain.Capacity, error)
	List(context.Context, uuid.UUID, int32, int32) ([]domain.Avatar, int64, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (domain.StoredAvatar, error)
	GetWallet(ctx context.Context, owner uuid.UUID) (*domain.Wallet, error)
	PurchaseSlot(ctx context.Context, owner uuid.UUID, wallet domain.Wallet, amount int32) error
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

type ModelSigner interface {
	PresignGet(context.Context, string, time.Duration) (storage.SignedRequest, error)
}

type Service struct {
	repository Repository
	signer     ModelSigner
}

func NewService(repository Repository, signer ModelSigner) *Service {
	return &Service{repository: repository, signer: signer}
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

func (s *Service) PurchaseSlot(ctx context.Context, user authdomain.User, amount int32) error {
	if err := authorize(user); err != nil {
		return err
	}
	if amount <= 0 {
		return apperror.New(apperror.CodeValidation, "slot purchase amount must be positive")
	}

	wallet, err := s.repository.GetWallet(ctx, user.ID)
	if err != nil {
		return err
	}
	if wallet.Balance < amount*domain.PricePerSlot {
		return apperror.New(apperror.CodeInsufficientBalance, "not enough coins to purchase inventory slots")
	}

	return s.repository.PurchaseSlot(ctx, user.ID, *wallet, amount)
}

func (s *Service) Delete(ctx context.Context, user authdomain.User, id uuid.UUID) error {
	if err := authorize(user); err != nil {
		return err
	}
	return s.repository.Delete(ctx, user.ID, id)
}
