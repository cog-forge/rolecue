package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type fakeRepository struct {
	calls          int
	owner, id      uuid.UUID
	limit, offset  int32
	capacity       domain.Capacity
	avatars        []domain.Avatar
	total          int64
	avatar         domain.StoredAvatar
	wallet         *domain.Wallet
	purchaseOwner  uuid.UUID
	purchaseWallet domain.Wallet
	purchaseAmount int32
	err            error
}

func (r *fakeRepository) Summary(_ context.Context, owner uuid.UUID) (domain.Capacity, error) {
	r.calls++
	r.owner = owner
	return r.capacity, r.err
}
func (r *fakeRepository) List(_ context.Context, owner uuid.UUID, limit, offset int32) ([]domain.Avatar, int64, error) {
	r.calls++
	r.owner, r.limit, r.offset = owner, limit, offset
	return r.avatars, r.total, r.err
}
func (r *fakeRepository) Get(_ context.Context, owner, id uuid.UUID) (domain.StoredAvatar, error) {
	r.calls++
	r.owner, r.id = owner, id
	return r.avatar, r.err
}
func (r *fakeRepository) Delete(_ context.Context, owner, id uuid.UUID) error {
	r.calls++
	r.owner, r.id = owner, id
	return r.err
}
func (r *fakeRepository) GetWallet(_ context.Context, owner uuid.UUID) (*domain.Wallet, error) {
	r.calls++
	r.owner = owner
	return r.wallet, r.err
}
func (r *fakeRepository) PurchaseSlot(_ context.Context, owner uuid.UUID, wallet domain.Wallet, amount int32) error {
	r.calls++
	r.purchaseOwner, r.purchaseWallet, r.purchaseAmount = owner, wallet, amount
	return r.err
}

type fakeSigner struct {
	calls int
	key   string
	ttl   time.Duration
	grant storage.SignedRequest
	err   error
}

func (s *fakeSigner) PresignGet(_ context.Context, key string, ttl time.Duration) (storage.SignedRequest, error) {
	s.calls++
	s.key, s.ttl = key, ttl
	return s.grant, s.err
}

func inventoryUser() authdomain.User {
	return authdomain.User{ID: uuid.New(), Role: "candidate", EmailVerified: true}
}

func TestServiceAuthorization(t *testing.T) {
	cases := []struct {
		name    string
		change  func(*authdomain.User)
		allowed bool
	}{
		{"candidate", func(u *authdomain.User) {}, true},
		{"recruiter", func(u *authdomain.User) { u.Role = "recruiter" }, true},
		{"missing ID", func(u *authdomain.User) { u.ID = uuid.Nil }, false},
		{"admin", func(u *authdomain.User) { u.Role = "admin" }, false},
		{"unknown role", func(u *authdomain.User) { u.Role = "guest" }, false},
		{"missing role", func(u *authdomain.User) { u.Role = "" }, false},
		{"unverified", func(u *authdomain.User) { u.EmailVerified = false }, false},
		{"locked", func(u *authdomain.User) { u.IsLocked = true }, false},
	}
	operations := []struct {
		name string
		run  func(*Service, authdomain.User) error
	}{
		{"summary", func(s *Service, u authdomain.User) error { _, err := s.Summary(context.Background(), u); return err }},
		{"list", func(s *Service, u authdomain.User) error {
			_, _, err := s.List(context.Background(), u, 10, 0)
			return err
		}},
		{"get", func(s *Service, u authdomain.User) error {
			_, err := s.Get(context.Background(), u, uuid.New())
			return err
		}},
		{"delete", func(s *Service, u authdomain.User) error { return s.Delete(context.Background(), u, uuid.New()) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, op := range operations {
				t.Run(op.name, func(t *testing.T) {
					user := inventoryUser()
					tc.change(&user)
					repo, signer := &fakeRepository{}, &fakeSigner{}
					err := op.run(NewService(repo, signer), user)
					if tc.allowed {
						if err != nil {
							t.Fatalf("authorized user rejected: %v", err)
						}
						if repo.calls != 1 || repo.owner != user.ID {
							t.Fatalf("repository not scoped to user: %+v", repo)
						}
					} else {
						if !apperror.IsCode(err, apperror.CodeForbidden) {
							t.Fatalf("want FORBIDDEN, got %v", err)
						}
						if repo.calls != 0 || signer.calls != 0 {
							t.Fatal("unauthorized request reached repository or signer")
						}
					}
				})
			}
		})
	}
}

func TestServiceSummaryAndList(t *testing.T) {
	user := inventoryUser()
	repo := &fakeRepository{
		capacity: domain.Capacity{Capacity: 3, Used: 2, Available: 1},
		avatars:  []domain.Avatar{{ID: uuid.New(), Name: "Personal avatar"}}, total: 7,
	}
	s := NewService(repo, nil)
	capacity, err := s.Summary(context.Background(), user)
	if err != nil || capacity != repo.capacity {
		t.Fatalf("summary = %+v, %v", capacity, err)
	}
	avatars, total, err := s.List(context.Background(), user, 2, 4)
	if err != nil || !reflect.DeepEqual(avatars, repo.avatars) || total != 7 {
		t.Fatalf("list = %+v, %d, %v", avatars, total, err)
	}
	if repo.owner != user.ID || repo.limit != 2 || repo.offset != 4 {
		t.Fatalf("wrong owner/pagination: %+v", repo)
	}
}

func TestServiceRepositoryErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		run  func(*Service, authdomain.User, uuid.UUID) error
	}{
		{"summary unavailable", errors.New("database unavailable"), func(s *Service, u authdomain.User, _ uuid.UUID) error {
			_, err := s.Summary(context.Background(), u)
			return err
		}},
		{"list unavailable", errors.New("database unavailable"), func(s *Service, u authdomain.User, _ uuid.UUID) error {
			_, _, err := s.List(context.Background(), u, 10, 0)
			return err
		}},
		{"get not found", apperror.New(apperror.CodeAvatarNotFound, "not found"), func(s *Service, u authdomain.User, id uuid.UUID) error {
			_, err := s.Get(context.Background(), u, id)
			return err
		}},
		{"delete not found", apperror.New(apperror.CodeAvatarNotFound, "not found"), func(s *Service, u authdomain.User, id uuid.UUID) error { return s.Delete(context.Background(), u, id) }},
		{"delete in use", apperror.New(apperror.CodeAvatarInUse, "in use"), func(s *Service, u authdomain.User, id uuid.UUID) error { return s.Delete(context.Background(), u, id) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, signer := &fakeRepository{err: tc.err}, &fakeSigner{}
			user, id := inventoryUser(), uuid.New()
			err := tc.run(NewService(repo, signer), user, id)
			if !errors.Is(err, tc.err) {
				t.Fatalf("repository error lost: %v", err)
			}
			if repo.owner != user.ID {
				t.Fatal("wrong owner")
			}
			if (tc.name == "get not found" || tc.name == "delete not found" || tc.name == "delete in use") && repo.id != id {
				t.Fatal("wrong avatar ID")
			}
			if signer.calls != 0 {
				t.Fatal("repository failure must not issue asset access")
			}
		})
	}
}

func TestServiceGetAssetUnavailable(t *testing.T) {
	cause := errors.New("storage unavailable")
	for _, tc := range []struct {
		name   string
		signer ModelSigner
		cause  error
	}{
		{"missing signer", nil, nil},
		{"presign failure", &fakeSigner{err: cause}, cause},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewService(&fakeRepository{avatar: domain.StoredAvatar{ModelPath: "private/model.vrm"}}, tc.signer)
			detail, err := s.Get(context.Background(), inventoryUser(), uuid.New())
			if !apperror.IsCode(err, apperror.CodeAvatarAssetUnavailable) {
				t.Fatalf("want AVATAR_ASSET_UNAVAILABLE, got %v", err)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("presign cause lost: %v", err)
			}
			if !reflect.DeepEqual(detail, domain.AvatarDetail{}) {
				t.Fatalf("failed request exposed detail: %+v", detail)
			}
		})
	}
}

func TestServiceGetFiveMinuteReadGrant(t *testing.T) {
	for _, headers := range []map[string]string{nil, {"x-model-header": "value"}} {
		name := "headers preserved"
		if headers == nil {
			name = "nil headers normalized"
		}
		t.Run(name, func(t *testing.T) {
			user, id := inventoryUser(), uuid.New()
			avatar := domain.Avatar{ID: id, Name: "Personal avatar", Personality: "Friendly", CreatedAt: time.Unix(100, 0), UpdatedAt: time.Unix(200, 0)}
			repo := &fakeRepository{avatar: domain.StoredAvatar{Avatar: avatar, ModelPath: "private/model.vrm"}}
			signer := &fakeSigner{grant: storage.SignedRequest{URL: "https://storage.example/model?signature=test", Method: "GET", Headers: headers, ExpiresAt: time.Unix(500, 0)}}
			detail, err := NewService(repo, signer).Get(context.Background(), user, id)
			if err != nil {
				t.Fatal(err)
			}
			if repo.owner != user.ID || repo.id != id {
				t.Fatal("wrong owner or avatar ID")
			}
			if signer.calls != 1 || signer.key != repo.avatar.ModelPath || signer.ttl != 5*time.Minute {
				t.Fatalf("wrong grant request: %+v", signer)
			}
			wantHeaders := headers
			if wantHeaders == nil {
				wantHeaders = map[string]string{}
			}
			want := domain.AvatarDetail{Avatar: avatar, ModelAccess: domain.ModelAccess{URL: signer.grant.URL, Method: "GET", Headers: wantHeaders, ExpiresAt: signer.grant.ExpiresAt}}
			if !reflect.DeepEqual(detail, want) {
				t.Fatalf("detail = %+v, want %+v", detail, want)
			}
		})
	}
}

func TestServiceDeleteOwnedAvatar(t *testing.T) {
	repo := &fakeRepository{}
	user, id := inventoryUser(), uuid.New()
	if err := NewService(repo, nil).Delete(context.Background(), user, id); err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 || repo.owner != user.ID || repo.id != id {
		t.Fatalf("wrong deletion scope: %+v", repo)
	}
}

func TestServicePurchaseSlot(t *testing.T) {
	user := inventoryUser()
	wallet := domain.Wallet{ID: uuid.New(), Balance: domain.PricePerSlot * 3}
	repo := &fakeRepository{wallet: &wallet}
	amount := int32(2)

	err := NewService(repo, nil).PurchaseSlot(context.Background(), user, amount)
	if err != nil {
		t.Fatalf("purchase slot: %v", err)
	}
	if repo.calls != 2 || repo.owner != user.ID || repo.purchaseOwner != user.ID {
		t.Fatalf("repository calls were not scoped to user: %+v", repo)
	}
	if repo.purchaseWallet != wallet || repo.purchaseAmount != amount {
		t.Fatalf("purchase args = wallet %+v, amount %d", repo.purchaseWallet, repo.purchaseAmount)
	}
}

func TestServicePurchaseSlotRejectsInvalidRequests(t *testing.T) {
	for _, amount := range []int32{0, -1} {
		t.Run("invalid amount", func(t *testing.T) {
			repo := &fakeRepository{}
			err := NewService(repo, nil).PurchaseSlot(context.Background(), inventoryUser(), amount)
			if !apperror.IsCode(err, apperror.CodeValidation) {
				t.Fatalf("want VALIDATION, got %v", err)
			}
			if repo.calls != 0 {
				t.Fatalf("invalid amount reached repository: %+v", repo)
			}
		})
	}

	t.Run("insufficient balance", func(t *testing.T) {
		user := inventoryUser()
		wallet := domain.Wallet{ID: uuid.New(), Balance: domain.PricePerSlot - 1}
		repo := &fakeRepository{wallet: &wallet}
		err := NewService(repo, nil).PurchaseSlot(context.Background(), user, 1)
		if !apperror.IsCode(err, apperror.CodeInsufficientBalance) {
			t.Fatalf("want INSUFFICIENT_BALANCE, got %v", err)
		}
		if repo.calls != 1 || repo.owner != user.ID {
			t.Fatalf("unexpected repository calls: %+v", repo)
		}
	})

	t.Run("wallet lookup failure", func(t *testing.T) {
		cause := errors.New("wallet unavailable")
		repo := &fakeRepository{err: cause}
		err := NewService(repo, nil).PurchaseSlot(context.Background(), inventoryUser(), 1)
		if !errors.Is(err, cause) {
			t.Fatalf("wallet lookup error lost: %v", err)
		}
		if repo.calls != 1 || repo.purchaseOwner != uuid.Nil || repo.purchaseAmount != 0 {
			t.Fatalf("purchase proceeded after wallet lookup failure: %+v", repo)
		}
	})
}
