//go:build integration

package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/repository"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// modelStore is the object storage of the delete tests: it records the deleted keys and fails the first
// failures deletes.
type modelStore struct {
	failures int
	deleted  []string
}

func (m *modelStore) PresignGet(context.Context, string, time.Duration) (storage.SignedRequest, error) {
	return storage.SignedRequest{}, errors.New("not used")
}

func (m *modelStore) Delete(_ context.Context, key string) error {
	if m.failures > 0 {
		m.failures--
		return errors.New("storage unavailable")
	}
	m.deleted = append(m.deleted, key)
	return nil
}

func deleteServiceFor(pool *pgxpool.Pool, store service.ModelStore) *service.Service {
	return service.NewService(repository.NewRepository(pool), store)
}

// avatar seeds an avatar in the owner's inventory (created on first use) and returns its id.
func (s *seed) avatar(owner authdomain.User, modelKey string) uuid.UUID {
	s.t.Helper()
	avatar := uuid.New()
	var inventory uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `INSERT INTO public.inventories (user_id, slot) VALUES ($1, 3)
		ON CONFLICT (user_id) DO UPDATE SET slot = public.inventories.slot RETURNING id`, owner.ID).Scan(&inventory); err != nil {
		s.t.Fatal(err)
	}
	s.exec(`INSERT INTO public.avatars (id, inventory_id, name, model_path, personality) VALUES ($1, $2, 'Delete me', $3, 'calm')`,
		avatar, inventory, modelKey)
	s.t.Cleanup(func() {
		// Runs before the seed cleanup (cleanups are last in, first out) because avatars reference inventories.
		_, _ = s.pool.Exec(context.Background(), `DELETE FROM public.interviews WHERE avatar_id = $1`, avatar)
		_, _ = s.pool.Exec(context.Background(), `DELETE FROM public.role_profiles WHERE candidate_id = $1`, owner.ID)
		_, _ = s.pool.Exec(context.Background(), `DELETE FROM public.avatars WHERE id = $1`, avatar)
	})
	return avatar
}

func (s *seed) avatarExists(id uuid.UUID) bool {
	s.t.Helper()
	var n int
	if err := s.pool.QueryRow(s.ctx, `SELECT count(*) FROM public.avatars WHERE id = $1`, id).Scan(&n); err != nil {
		s.t.Fatal(err)
	}
	return n == 1
}

func wantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

// TestDeleteAvatarRemovesTheRowAndTheStoredModel proves the object goes with the row, and that a second
// delete is a plain 404 that does not touch storage again.
func TestDeleteAvatarRemovesTheRowAndTheStoredModel(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner := s.candidate(nil)
	avatar := s.avatar(owner, "avatars/"+uuid.NewString()+".vrm")
	store := &modelStore{}
	svc := deleteServiceFor(pool, store)

	if err := svc.Delete(ctx, owner, avatar); err != nil {
		t.Fatal(err)
	}
	if s.avatarExists(avatar) || len(store.deleted) != 1 {
		t.Fatalf("row exists = %v, object deletes = %v, want the row and one object gone", s.avatarExists(avatar), store.deleted)
	}
	wantCode(t, svc.Delete(ctx, owner, avatar), apperror.CodeAvatarNotFound)
	if len(store.deleted) != 1 {
		t.Fatalf("a repeated delete touched storage: %v", store.deleted)
	}
}

// TestDeleteAvatarRetriesAndThenKeepsTheRow proves a short storage outage is retried inside the request,
// and a long one keeps the row so the owner can delete again once storage is back.
func TestDeleteAvatarRetriesAndThenKeepsTheRow(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner := s.candidate(nil)
	avatar := s.avatar(owner, "avatars/"+uuid.NewString()+".vrm")

	blip := &modelStore{failures: 1}
	if err := deleteServiceFor(pool, blip).Delete(ctx, owner, avatar); err != nil {
		t.Fatalf("one failed attempt is retried, got %v", err)
	}
	if s.avatarExists(avatar) || len(blip.deleted) != 1 {
		t.Fatalf("row exists = %v, deleted = %v after the retry", s.avatarExists(avatar), blip.deleted)
	}

	avatar = s.avatar(owner, "avatars/"+uuid.NewString()+".vrm")
	down := &modelStore{failures: 100}
	wantCode(t, deleteServiceFor(pool, down).Delete(ctx, owner, avatar), apperror.CodeAvatarAssetUnavailable)
	if !s.avatarExists(avatar) {
		t.Fatal("the row must stay while the object could not be deleted")
	}
	back := &modelStore{}
	if err := deleteServiceFor(pool, back).Delete(ctx, owner, avatar); err != nil {
		t.Fatal(err)
	}
	if s.avatarExists(avatar) || len(back.deleted) != 1 {
		t.Fatalf("row exists = %v, deleted = %v after storage came back", s.avatarExists(avatar), back.deleted)
	}
}

// TestDeleteAvatarOfSomeoneElseTouchesNothing proves another account gets a 404, and the owner's row and
// object stay.
func TestDeleteAvatarOfSomeoneElseTouchesNothing(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner, other := s.candidate(nil), s.candidate(nil)
	other.Role = "recruiter" // the other account may be either role the inventory serves
	avatar := s.avatar(owner, "avatars/"+uuid.NewString()+".vrm")
	store := &modelStore{}

	wantCode(t, deleteServiceFor(pool, store).Delete(ctx, other, avatar), apperror.CodeAvatarNotFound)
	if !s.avatarExists(avatar) || len(store.deleted) != 0 {
		t.Fatalf("row exists = %v, object deletes = %v, want the avatar and its object untouched", s.avatarExists(avatar), store.deleted)
	}
}

// TestDeleteAvatarInUseKeepsTheObject proves an avatar an interview still references is refused before
// the object is deleted.
func TestDeleteAvatarInUseKeepsTheObject(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner := s.candidate(nil)
	avatar := s.avatar(owner, "avatars/"+uuid.NewString()+".vrm")
	jd, profile := uuid.New(), uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, content_path) VALUES ($1, 'jd.pdf')`, jd)
	s.t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM public.job_descriptions WHERE id = $1`, jd)
	})
	s.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title) VALUES ($1, $2, $3, 'In use')`, profile, jd, owner.ID)
	s.exec(`INSERT INTO public.interviews (type, role_profile_id, avatar_id) VALUES ('practice', $1, $2)`, profile, avatar)
	store := &modelStore{}

	wantCode(t, deleteServiceFor(pool, store).Delete(ctx, owner, avatar), apperror.CodeAvatarInUse)
	if !s.avatarExists(avatar) || len(store.deleted) != 0 {
		t.Fatalf("row exists = %v, object deletes = %v, want the avatar and its object kept", s.avatarExists(avatar), store.deleted)
	}
}
