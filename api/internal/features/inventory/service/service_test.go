package service

import (
	"context"
	"errors"
	"fmt"
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
	purchase       domain.SlotPurchase
	purchaseOwner  uuid.UUID
	purchaseAmount int32
	purchaseKey    string
	deleted        bool
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

// Delete mirrors the transaction: a repository error means nothing was removed, otherwise the stored
// model key goes to purge, and a purge error rolls the removal back.
func (r *fakeRepository) Delete(ctx context.Context, owner, id uuid.UUID, purge func(context.Context, string) error) error {
	r.calls++
	r.owner, r.id = owner, id
	if r.err != nil {
		return r.err
	}
	if err := purge(ctx, r.avatar.ModelPath); err != nil {
		return err
	}
	r.deleted = true
	return nil
}
func (r *fakeRepository) PurchaseSlot(_ context.Context, owner uuid.UUID, amount int32, key string) (domain.SlotPurchase, error) {
	r.purchaseKey = key
	r.calls++
	r.owner, r.purchaseOwner, r.purchaseAmount = owner, owner, amount
	return r.purchase, r.err
}

type fakeSigner struct {
	deleteCalls  int
	deleteKeys   []string
	deleteErrors []error // consumed one per Delete call; an empty list means success
	calls        int
	key          string
	ttl          time.Duration
	grant        storage.SignedRequest
	err          error
}

func (s *fakeSigner) PresignGet(_ context.Context, key string, ttl time.Duration) (storage.SignedRequest, error) {
	s.calls++
	s.key, s.ttl = key, ttl
	return s.grant, s.err
}

func (s *fakeSigner) Delete(_ context.Context, key string) error {
	s.deleteCalls++
	s.deleteKeys = append(s.deleteKeys, key)
	if len(s.deleteErrors) == 0 {
		return nil
	}
	err := s.deleteErrors[0]
	s.deleteErrors = s.deleteErrors[1:]
	return err
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
		{"purchase", func(s *Service, u authdomain.User) error {
			_, err := s.PurchaseSlot(context.Background(), u, 1, "")
			return err
		}},
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
		signer ModelStore
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

func deleteService(repo *fakeRepository, store *fakeSigner) *Service {
	s := NewService(repo, store)
	s.retryDelays = []time.Duration{0, 0} // three attempts without waiting
	return s
}

func TestServiceDeleteRemovesTheStoredModel(t *testing.T) {
	repo := &fakeRepository{avatar: domain.StoredAvatar{ModelPath: "avatars/model.vrm"}}
	store := &fakeSigner{}
	user, id := inventoryUser(), uuid.New()
	if err := deleteService(repo, store).Delete(context.Background(), user, id); err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 || repo.owner != user.ID || repo.id != id || !repo.deleted {
		t.Fatalf("wrong deletion scope: %+v", repo)
	}
	if store.deleteCalls != 1 || store.deleteKeys[0] != "avatars/model.vrm" {
		t.Fatalf("object deletes = %v", store.deleteKeys)
	}
}

func TestServiceDeleteToleratesAMissingObject(t *testing.T) {
	repo := &fakeRepository{avatar: domain.StoredAvatar{ModelPath: "avatars/gone.vrm"}}
	store := &fakeSigner{deleteErrors: []error{fmt.Errorf("delete object: %w", storage.ErrNotFound)}}
	if err := deleteService(repo, store).Delete(context.Background(), inventoryUser(), uuid.New()); err != nil {
		t.Fatalf("a missing object is already deleted, got %v", err)
	}
	if !repo.deleted || store.deleteCalls != 1 {
		t.Fatalf("deleted = %v after %d object deletes, want the row removed after one", repo.deleted, store.deleteCalls)
	}
}

func TestServiceDeleteRetriesAStorageFailure(t *testing.T) {
	repo := &fakeRepository{avatar: domain.StoredAvatar{ModelPath: "avatars/model.vrm"}}
	store := &fakeSigner{deleteErrors: []error{errors.New("storage unavailable"), errors.New("storage unavailable")}}
	if err := deleteService(repo, store).Delete(context.Background(), inventoryUser(), uuid.New()); err != nil {
		t.Fatalf("the third attempt succeeds, got %v", err)
	}
	if !repo.deleted || store.deleteCalls != 3 {
		t.Fatalf("deleted = %v after %d object deletes, want true after 3", repo.deleted, store.deleteCalls)
	}
}

func TestServiceDeleteKeepsTheAvatarWhenStorageStaysDown(t *testing.T) {
	repo := &fakeRepository{avatar: domain.StoredAvatar{ModelPath: "avatars/model.vrm"}}
	down := errors.New("storage unavailable")
	store := &fakeSigner{deleteErrors: []error{down, down, down, down}}
	err := deleteService(repo, store).Delete(context.Background(), inventoryUser(), uuid.New())
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeAvatarAssetUnavailable {
		t.Fatalf("err = %v, want AVATAR_ASSET_UNAVAILABLE", err)
	}
	if repo.deleted {
		t.Fatal("the row must stay when the object could not be deleted")
	}
	if store.deleteCalls != 3 {
		t.Fatalf("object deletes = %d, want 3 attempts", store.deleteCalls)
	}
}

func TestServiceDeleteStopsRetryingWhenTheRequestIsCancelled(t *testing.T) {
	repo := &fakeRepository{avatar: domain.StoredAvatar{ModelPath: "avatars/model.vrm"}}
	store := &fakeSigner{deleteErrors: []error{errors.New("storage unavailable"), errors.New("storage unavailable")}}
	s := NewService(repo, store)
	s.retryDelays = []time.Duration{time.Hour, time.Hour}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Delete(ctx, inventoryUser(), uuid.New()); err == nil || repo.deleted {
		t.Fatalf("err = %v, deleted = %v, want a failure that keeps the row", err, repo.deleted)
	}
	if store.deleteCalls > 1 {
		t.Fatalf("object deletes = %d, want no retry after cancellation", store.deleteCalls)
	}
}

func TestServiceDeleteWithoutStorageKeepsTheAvatar(t *testing.T) {
	repo := &fakeRepository{}
	err := NewService(repo, nil).Delete(context.Background(), inventoryUser(), uuid.New())
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeAvatarAssetUnavailable || repo.deleted {
		t.Fatalf("err = %v, deleted = %v, want AVATAR_ASSET_UNAVAILABLE and the row kept", err, repo.deleted)
	}
}

func TestServiceDeleteDoesNotTouchStorageWhenTheAvatarIsNotTheCallers(t *testing.T) {
	for _, code := range []apperror.Code{apperror.CodeAvatarNotFound, apperror.CodeAvatarInUse} {
		repo := &fakeRepository{err: apperror.New(code, "refused")}
		store := &fakeSigner{}
		err := deleteService(repo, store).Delete(context.Background(), inventoryUser(), uuid.New())
		var appErr *apperror.AppError
		if !errors.As(err, &appErr) || appErr.Code != code {
			t.Fatalf("err = %v, want %s", err, code)
		}
		if store.deleteCalls != 0 {
			t.Fatalf("%s: object deletes = %d, want 0", code, store.deleteCalls)
		}
	}
}

func TestServicePurchaseSlot(t *testing.T) {
	for _, amount := range []int32{1, domain.MaxSlotsPerPurchase} {
		user := inventoryUser()
		want := domain.SlotPurchase{Capacity: domain.Capacity{Capacity: 3 + amount, Used: 1, Available: int64(2 + amount)}, Balance: 7}
		repo := &fakeRepository{purchase: want}

		got, err := NewService(repo, nil).PurchaseSlot(context.Background(), user, amount, "")
		if err != nil {
			t.Fatalf("purchase %d slots: %v", amount, err)
		}
		if got != want {
			t.Fatalf("purchase = %+v, want the repository result %+v", got, want)
		}
		if repo.calls != 1 || repo.purchaseOwner != user.ID || repo.purchaseAmount != amount {
			t.Fatalf("purchase was not one call scoped to the user for %d slots: %+v", amount, repo)
		}
	}
}

func TestServicePurchaseSlotPassesTheIdempotencyKey(t *testing.T) {
	repo := &fakeRepository{}
	if _, err := NewService(repo, nil).PurchaseSlot(context.Background(), inventoryUser(), 2, "attempt-1"); err != nil {
		t.Fatalf("PurchaseSlot() error = %v", err)
	}
	if repo.purchaseKey != "attempt-1" {
		t.Fatalf("repository key = %q, want attempt-1", repo.purchaseKey)
	}
}

func TestServicePurchaseSlotRejectsInvalidAmounts(t *testing.T) {
	for _, amount := range []int32{0, -1, domain.MaxSlotsPerPurchase + 1, 1 << 30} {
		repo := &fakeRepository{}
		got, err := NewService(repo, nil).PurchaseSlot(context.Background(), inventoryUser(), amount, "")
		if !apperror.IsCode(err, apperror.CodeValidation) {
			t.Fatalf("amount %d: want VALIDATION_ERROR, got %v", amount, err)
		}
		if repo.calls != 0 || got != (domain.SlotPurchase{}) {
			t.Fatalf("amount %d reached the repository or returned a result: %+v %+v", amount, repo, got)
		}
	}
}

func TestServicePurchaseSlotPassesRepositoryErrorsThrough(t *testing.T) {
	for _, cause := range []error{
		apperror.New(apperror.CodeInsufficientBalance, "insufficient coin balance"),
		errors.New("database unavailable"),
	} {
		repo := &fakeRepository{err: cause}
		got, err := NewService(repo, nil).PurchaseSlot(context.Background(), inventoryUser(), 2, "")
		if !errors.Is(err, cause) {
			t.Fatalf("repository error lost: %v", err)
		}
		if got != (domain.SlotPurchase{}) {
			t.Fatalf("failed purchase returned a result: %+v", got)
		}
	}
}
