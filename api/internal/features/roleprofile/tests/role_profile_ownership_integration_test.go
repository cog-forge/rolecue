//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	qbrepository "github.com/cog-forge/rolecue/api/internal/features/questionbank/repository"
	qbservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/repository"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// roleProfileEnv wires the real repository, schema and question bank, so the
// tests below exercise ownership and constraints in PostgreSQL.
type roleProfileEnv struct {
	ctx  context.Context
	pool *pgxpool.Pool
	repo *repository.RoleProfileRepository
	app  *service.RoleProfileApplication
}

func newRoleProfileEnv(t *testing.T) roleProfileEnv {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("DATABASE_URL required", err)
	}
	t.Cleanup(pool.Close)
	repo := repository.NewRoleProfileRepository(pool)
	extraction, err := service.NewExtractionService(&fixtureAI{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	questionBank, err := qbservice.New(qbrepository.NewRepository(pool), fixtureQuestionGenerator{}, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	app, err := service.NewRoleProfileApplication(extraction, repo, questionBank, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return roleProfileEnv{ctx: ctx, pool: pool, repo: repo, app: app}
}

// insertFixtureUser registers cleanup of the user row; any cleanup registered
// later (profiles, postings) runs first because t.Cleanup is LIFO.
func insertFixtureUser(t *testing.T, env roleProfileEnv, role string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO users(id,name,email,role,email_verified) VALUES($1,'KAN178 fixture',$2,$3,true)`, id, id.String()+"@kan178.invalid", role); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = env.pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, id) })
	return id
}

// createAnalyzedProfile creates a Role Profile owned by owner and runs analysis,
// so it has requirements and reaches the review state.
func createAnalyzedProfile(t *testing.T, env roleProfileEnv, owner uuid.UUID) domain.RoleProfile {
	t.Helper()
	raw := strings.Repeat("Senior backend engineer using Go, concurrency, caching and PostgreSQL. ", 4)
	profile, err := env.app.Create(env.ctx, owner, "candidate", raw)
	if err != nil {
		t.Fatal("create", err)
	}
	t.Cleanup(func() { _ = env.repo.DeleteProfile(context.Background(), owner, profile.ID) })
	profile, err = env.app.Analyze(env.ctx, owner, "candidate", profile.ID)
	if err != nil || profile.Status != domain.Review {
		t.Fatal("analyze", err)
	}
	return profile
}

func wantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func TestRoleProfileCrossUserAccessIsNotFound(t *testing.T) {
	env := newRoleProfileEnv(t)
	owner := insertFixtureUser(t, env, "candidate")
	other := insertFixtureUser(t, env, "candidate")
	profile := createAnalyzedProfile(t, env, owner)

	review := profile.RoleReview
	review.Title = "Hijacked title"
	_, err := env.app.Get(env.ctx, other, "candidate", profile.ID)
	wantCode(t, err, apperror.CodeJDNotFound)
	_, err = env.app.Save(env.ctx, other, "candidate", profile.ID, domain.SaveRoleProfile{RoleReview: review})
	wantCode(t, err, apperror.CodeJDNotFound)
	_, err = env.app.Confirm(env.ctx, other, "candidate", profile.ID)
	wantCode(t, err, apperror.CodeJDNotFound)
	_, err = env.app.Analyze(env.ctx, other, "candidate", profile.ID)
	wantCode(t, err, apperror.CodeJDNotFound)
	wantCode(t, env.app.Delete(env.ctx, other, "candidate", profile.ID), apperror.CodeJDNotFound)

	after, err := env.app.Get(env.ctx, owner, "candidate", profile.ID)
	if err != nil {
		t.Fatal("owner lost the profile", err)
	}
	if after.Title != profile.Title || after.Status != domain.Review || len(after.Requirements) != len(profile.Requirements) {
		t.Fatalf("cross-user writes changed the owner's profile: %+v", after)
	}
}

func TestRoleProfileDeleteIsBlockedWhileJDIsReferencedByJobPosting(t *testing.T) {
	env := newRoleProfileEnv(t)
	owner := insertFixtureUser(t, env, "candidate")
	recruiter := insertFixtureUser(t, env, "recruiter")
	profile := createAnalyzedProfile(t, env, owner)
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO job_postings(jd_id, recruiter_id, title) VALUES($1,$2,'KAN-178 posting')`, profile.JDID, recruiter); err != nil {
		t.Fatal(err)
	}
	// Registered after the profile cleanup, so the posting is removed first.
	t.Cleanup(func() {
		_, _ = env.pool.Exec(context.Background(), `DELETE FROM job_postings WHERE jd_id=$1`, profile.JDID)
	})

	wantCode(t, env.app.Delete(env.ctx, owner, "candidate", profile.ID), apperror.CodeJDInUse)

	// The failed delete rolls back: the profile and its requirements remain.
	if _, err := env.app.Get(env.ctx, owner, "candidate", profile.ID); err != nil {
		t.Fatal("profile gone after conflict", err)
	}
	var requirements int
	if err := env.pool.QueryRow(env.ctx, `SELECT count(*) FROM requirements WHERE jd_id=$1`, profile.JDID).Scan(&requirements); err != nil || requirements != len(profile.Requirements) {
		t.Fatalf("requirements = %d, err = %v, want %d", requirements, err, len(profile.Requirements))
	}
}

func TestRoleProfileCandidateResponsesOmitCoreQuestions(t *testing.T) {
	env := newRoleProfileEnv(t)
	owner := insertFixtureUser(t, env, "candidate")
	profile := createAnalyzedProfile(t, env, owner)
	confirmed, err := env.app.Confirm(env.ctx, owner, "candidate", profile.ID)
	if err != nil || confirmed.Status != domain.Confirmed {
		t.Fatal("confirm", err)
	}
	var questions int
	if err = env.pool.QueryRow(env.ctx, `SELECT count(*) FROM core_questions WHERE jd_id=$1`, profile.JDID).Scan(&questions); err != nil || questions == 0 {
		// Without stored questions the redaction assertions below would pass vacuously.
		t.Fatal("no core questions were generated", questions, err)
	}
	const probe = "How do you use Go concurrency in a production service?"
	read, err := env.app.Get(env.ctx, owner, "candidate", profile.ID)
	if err != nil {
		t.Fatal("get", err)
	}
	for name, value := range map[string]any{"confirm": confirmed, "get": read} {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(body))
		if strings.Contains(string(body), probe) || strings.Contains(lower, "question") {
			t.Fatalf("%s response leaked core-question data: %s", name, body)
		}
	}
}
