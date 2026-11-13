//go:build integration

package tests

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/repository"
	rpdomain "github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	rprepository "github.com/cog-forge/rolecue/api/internal/features/roleprofile/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type env struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	roadmaps *repository.Repository
	profiles *rprepository.RoleProfileRepository
}

func newEnv(t *testing.T) env {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("DATABASE_URL required", err)
	}
	t.Cleanup(pool.Close)
	return env{ctx: ctx, pool: pool, roadmaps: repository.NewRepository(pool), profiles: rprepository.NewRoleProfileRepository(pool)}
}

func (e env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(e.ctx, sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func (e env) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(e.ctx, sql, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", sql, err)
	}
	return n
}

// user inserts a fixture user and removes it last (t.Cleanup is LIFO, so cleanups registered later run first).
func (e env) user(t *testing.T, role string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	e.exec(t, `INSERT INTO users(id,name,email,role,email_verified) VALUES($1,'KAN342 fixture',$2,$3,true)`, id, id.String()+"@kan342.invalid", role)
	t.Cleanup(func() { _, _ = e.pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, id) })
	return id
}

// profile creates a Role Profile through the repository, the way the API does.
func (e env) profile(t *testing.T, owner uuid.UUID) rpdomain.RoleProfile {
	t.Helper()
	raw := strings.Repeat("Senior backend engineer using Go, concurrency, caching and PostgreSQL. ", 4)
	profile, err := e.profiles.CreateProfile(e.ctx, owner, raw)
	if err != nil {
		t.Fatal("create profile", err)
	}
	t.Cleanup(func() { e.removeProfile(profile.ID, owner) })
	return profile
}

func (e env) removeProfile(profile, owner uuid.UUID) {
	ctx := context.Background()
	_, _ = e.pool.Exec(ctx, `DELETE FROM roadmap_assessments WHERE interview_id IN (SELECT id FROM interviews WHERE role_profile_id=$1)`, profile)
	_, _ = e.pool.Exec(ctx, `DELETE FROM interview_questions WHERE interview_id IN (SELECT id FROM interviews WHERE role_profile_id=$1)`, profile)
	_, _ = e.pool.Exec(ctx, `DELETE FROM interviews WHERE role_profile_id=$1`, profile)
	_ = e.profiles.DeleteProfile(ctx, owner, profile)
}

func (e env) roadmap(t *testing.T, owner, profile uuid.UUID) domain.Roadmap {
	t.Helper()
	roadmap, err := e.roadmaps.GetByProfile(e.ctx, owner, profile)
	if err != nil {
		t.Fatal("get roadmap", err)
	}
	return roadmap
}

// publish takes the Roadmap through claim and publish with the given topics.
func (e env) publish(t *testing.T, roadmap uuid.UUID, titles ...string) []domain.Item {
	t.Helper()
	fence, err := e.roadmaps.Claim(e.ctx, roadmap, []domain.GenerationStatus{domain.StatusPending, domain.StatusFailed}, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal("claim", err)
	}
	items := make([]domain.NewItem, len(titles))
	for i, title := range titles {
		items[i] = domain.NewItem{Title: title, Description: title + " scope", Position: i}
	}
	stored, err := e.roadmaps.Publish(e.ctx, roadmap, fence, items, nil)
	if err != nil {
		t.Fatal("publish", err)
	}
	return stored
}

// question inserts a bank question of the profile's job description, mapped to item (uuid.Nil: unmapped).
func (e env) question(t *testing.T, jd, item uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var itemArg *uuid.UUID
	if item != uuid.Nil {
		itemArg = &item
	}
	e.exec(t, `INSERT INTO core_questions(id,jd_id,content,roadmap_item_id) VALUES($1,$2,'What is a data race?',$3)`, id, jd, itemArg)
	return id
}

// interview inserts a practice session of the profile with one assigned question at position 1.
func (e env) interview(t *testing.T, profile, question uuid.UUID, focus uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var focusArg *uuid.UUID
	if focus != uuid.Nil {
		focusArg = &focus
	}
	e.exec(t, `INSERT INTO interviews(id,role_profile_id,type,focus_item_id) VALUES($1,$2,'practice',$3)`, id, profile, focusArg)
	e.exec(t, `INSERT INTO interview_questions(interview_id,position,core_question_id) VALUES($1,1,$2)`, id, question)
	return id
}

func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
