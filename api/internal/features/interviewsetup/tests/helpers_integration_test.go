//go:build integration

// Package tests: pre-interview setup integration tests against a real Postgres (KAN-231). The repository and
// the service run unmodified. Every test seeds its own users, postings and profiles and removes exactly those
// rows afterwards, so the tests stay independent of each other and of the rest of the database.
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// openPool connects to the disposable database named by DATABASE_URL. The test is skipped when the variable
// is unset or the database is unreachable.
func openPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	return ctx, pool
}

// seed tracks every row a test creates; the cleanup removes them in foreign-key order, scoped to the ids the
// test made.
type seed struct {
	t        *testing.T
	ctx      context.Context
	pool     *pgxpool.Pool
	svc      *service.Service
	users    []string
	jds      []string
	postings []string
	voices   []string
}

func newSeed(t *testing.T) *seed {
	t.Helper()
	ctx, pool := openPool(t)
	s := &seed{t: t, ctx: ctx, pool: pool, svc: service.New(repository.NewRepository(pool))}
	t.Cleanup(s.cleanup)
	return s
}

func (s *seed) cleanup() {
	ctx := context.Background()
	steps := []struct {
		name string
		sql  string
		args []any
	}{
		{"interview_setups", `DELETE FROM public.interview_setups WHERE role_profile_id IN
			(SELECT id FROM public.role_profiles WHERE candidate_id = ANY($1::text[]::uuid[]))
			OR application_id IN (SELECT id FROM public.applications WHERE candidate_id = ANY($1::text[]::uuid[]))`, []any{s.users}},
		{"interviews", `DELETE FROM public.interviews WHERE application_id IN
			(SELECT id FROM public.applications WHERE candidate_id = ANY($1::text[]::uuid[]))`, []any{s.users}},
		{"applications", `DELETE FROM public.applications WHERE candidate_id = ANY($1::text[]::uuid[])
			OR job_posting_id = ANY($2::text[]::uuid[])`, []any{s.users, s.postings}},
		{"job_postings", `DELETE FROM public.job_postings WHERE id = ANY($1::text[]::uuid[])`, []any{s.postings}},
		{"avatars", `DELETE FROM public.avatars WHERE inventory_id IN
			(SELECT id FROM public.inventories WHERE user_id = ANY($1::text[]::uuid[]))`, []any{s.users}},
		{"avatar assets", `DELETE FROM public.avatar_assets s WHERE s.created_by = ANY($1::text[]::uuid[])
			AND NOT EXISTS (SELECT 1 FROM public.avatars a WHERE a.asset_id = s.id)`, []any{s.users}},
		{"inventories", `DELETE FROM public.inventories WHERE user_id = ANY($1::text[]::uuid[])`, []any{s.users}},
		{"voices", `DELETE FROM public.voices WHERE id = ANY($1::text[]::uuid[])`, []any{s.voices}},
		{"core_questions", `DELETE FROM public.core_questions WHERE jd_id = ANY($1::text[]::uuid[])`, []any{s.jds}},
		{"role_profiles", `DELETE FROM public.role_profiles WHERE candidate_id = ANY($1::text[]::uuid[])`, []any{s.users}},
		{"job_descriptions", `DELETE FROM public.job_descriptions WHERE id = ANY($1::text[]::uuid[])`, []any{s.jds}},
		{"transactions", `DELETE FROM public.transactions WHERE "from" = ANY($1::text[]) OR "to" = ANY($1::text[])`, []any{s.users}},
		{"wallets", `DELETE FROM public.wallets WHERE user_id = ANY($1::text[]::uuid[])`, []any{s.users}},
		{"users", `DELETE FROM public.users WHERE id = ANY($1::text[]::uuid[])`, []any{s.users}},
	}
	for _, step := range steps {
		if _, err := s.pool.Exec(ctx, step.sql, step.args...); err != nil {
			s.t.Errorf("cleanup %s: %v", step.name, err)
		}
	}
}

func (s *seed) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.pool.Exec(s.ctx, sql, args...); err != nil {
		s.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (s *seed) count(sql string, args ...any) int {
	s.t.Helper()
	var n int
	if err := s.pool.QueryRow(s.ctx, sql, args...).Scan(&n); err != nil {
		s.t.Fatalf("count %q: %v", sql, err)
	}
	return n
}

// user adds a user of the role with a funded wallet (so a coin debit would be visible).
func (s *seed) user(role string) uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	var company *string
	if role == "recruiter" {
		name := "Setup Test Co"
		company = &name
	}
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified, company_name)
		VALUES ($1, $2, $3, $4, true, $5)`, id, "setup "+role, fmt.Sprintf("setup-%s@example.test", id), role, company)
	s.users = append(s.users, id.String())
	s.exec(`INSERT INTO public.wallets (user_id, balance) VALUES ($1, 500)`, id)
	return id
}

// profile gives the candidate a Role Profile of the status with a bank of questions core questions.
func (s *seed) profile(candidate uuid.UUID, status string, questions int) uuid.UUID {
	s.t.Helper()
	jd := uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, content_path) VALUES ($1, $2)`, jd, "job-descriptions/"+jd.String()+".pdf")
	s.jds = append(s.jds, jd.String())
	for i := 0; i < questions; i++ {
		s.exec(`INSERT INTO public.core_questions (jd_id, content) VALUES ($1, $2)`, jd, fmt.Sprintf("setup bank question %d", i))
	}
	id := uuid.New()
	s.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title, status) VALUES ($1, $2, $3, 'Setup role', $4)`, id, jd, candidate, status)
	return id
}

func (s *seed) confirmedProfile(candidate uuid.UUID) uuid.UUID {
	return s.profile(candidate, "confirmed", 3)
}

// avatarOf gives the user an inventory and one avatar in it.
func (s *seed) avatarOf(user uuid.UUID, name string) uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	s.exec(`INSERT INTO public.inventories (user_id, slot) VALUES ($1, 1) ON CONFLICT (user_id) DO NOTHING`, user)
	s.exec(`WITH s AS (INSERT INTO public.avatar_assets (created_by, model_path) VALUES ($1, 'models/setup-test.vrm') RETURNING id)
		INSERT INTO public.avatars (id, inventory_id, name, asset_id, personality)
		SELECT $2, inv.id, $3, s.id, 'calm' FROM public.inventories inv, s WHERE inv.user_id = $1`, user, id, name)
	return id
}

func (s *seed) voice(active bool) uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	s.exec(`INSERT INTO public.voices (id, name, is_active) VALUES ($1, 'Setup voice', $2)`, id, active)
	s.voices = append(s.voices, id.String())
	return id
}

// posting adds a recruiter's open posting funded with one slot, locking the avatar and voice.
func (s *seed) posting(recruiter, avatar, voice uuid.UUID) uuid.UUID {
	s.t.Helper()
	jd := uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, content_path) VALUES ($1, $2)`, jd, "job-descriptions/"+jd.String()+".pdf")
	s.jds = append(s.jds, jd.String())
	id := uuid.New()
	s.exec(`INSERT INTO public.job_postings (id, status, jd_id, recruiter_id, title, interview_slot, required_avatar_id, required_voice_id)
		VALUES ($1, 'open', $2, $3, 'Setup posting', 1, $4, $5)`, id, jd, recruiter, avatar, voice)
	s.postings = append(s.postings, id.String())
	return id
}

func (s *seed) application(posting, candidate uuid.UUID, status string, deadline *time.Time) uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	s.exec(`INSERT INTO public.applications (id, job_posting_id, candidate_id, cv_path, status, interview_deadline)
		VALUES ($1, $2, $3, 'cvs/setup-test.pdf', $4::public.application_status, $5)`, id, posting, candidate, status, deadline)
	return id
}

// eligibleApplication is a CV-approved Application with a deadline a day ahead.
func (s *seed) eligibleApplication(posting, candidate uuid.UUID) uuid.UUID {
	deadline := time.Now().Add(24 * time.Hour)
	return s.application(posting, candidate, "interview_eligible", &deadline)
}

// recruitmentFixture is a recruiter with an avatar, a voice and an open posting, and a candidate with an eligible
// Application on it.
type recruitmentFixture struct {
	recruiter, candidate, avatar, voice, posting, application uuid.UUID
}

func (s *seed) recruitment() recruitmentFixture {
	f := recruitmentFixture{recruiter: s.user("recruiter"), candidate: s.user("candidate")}
	f.avatar = s.avatarOf(f.recruiter, "Company interviewer")
	f.voice = s.voice(true)
	f.posting = s.posting(f.recruiter, f.avatar, f.voice)
	f.application = s.eligibleApplication(f.posting, f.candidate)
	return f
}

func codeOf(err error) apperror.Code {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	if got := codeOf(err); got != want {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

func blockerCodes(setup domain.Setup) []domain.BlockerCode {
	out := []domain.BlockerCode{}
	for _, b := range setup.Blockers {
		out = append(out, b.Code)
	}
	return out
}

func str(v string) *string { return &v }

// stamp is what a setup row looks like from outside: enough to prove a read or a refused save changed nothing.
type stamp struct {
	rows      int
	updatedAt time.Time
}

func (s *seed) stamp(profile uuid.UUID) stamp {
	s.t.Helper()
	var st stamp
	st.rows = s.count(`SELECT count(*) FROM public.interview_setups WHERE role_profile_id = $1`, profile)
	_ = s.pool.QueryRow(s.ctx, `SELECT updated_at FROM public.interview_setups WHERE role_profile_id = $1`, profile).Scan(&st.updatedAt)
	return st
}
