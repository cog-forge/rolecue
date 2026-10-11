//go:build integration

package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// openPool connects to the migrated database from DATABASE_URL, or skips the test.
func openPool(t *testing.T, timeout time.Duration) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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

// seed inserts rows tagged with a random suffix and deletes exactly those rows afterwards.
type seed struct {
	t    *testing.T
	pool *pgxpool.Pool
	tag  string
	ctx  context.Context
	// cleanup statements, executed in reverse order
	undo []string
	args [][]any
}

func newSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *seed {
	return &seed{t: t, pool: pool, tag: uuid.NewString()[:8], ctx: ctx}
}

func (s *seed) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.pool.Exec(s.ctx, sql, args...); err != nil {
		s.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (s *seed) later(sql string, args ...any) {
	s.undo = append(s.undo, sql)
	s.args = append(s.args, args)
}

func (s *seed) cleanup() {
	for i := len(s.undo) - 1; i >= 0; i-- {
		if _, err := s.pool.Exec(context.Background(), s.undo[i], s.args[i]...); err != nil {
			s.t.Errorf("cleanup %q: %v", s.undo[i], err)
		}
	}
}

func (s *seed) level(name, description string) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.levels (id, name, description) VALUES ($1, $2, $3)`, id, name, description)
	s.later(`DELETE FROM public.levels WHERE id = $1`, id)
	return id
}

func (s *seed) user(role string) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, $2, $3, $4, true)`,
		id, "qb "+role, fmt.Sprintf("qb-%s-%s@example.test", s.tag, id), role)
	s.later(`DELETE FROM public.users WHERE id = $1`, id)
	return id
}

func (s *seed) jd(levelID uuid.UUID, note string) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, refinement_note, content_path, level_id) VALUES ($1, $2, $3, $4)`,
		id, note, "qb/"+s.tag, levelID)
	s.later(`DELETE FROM public.job_descriptions WHERE id = $1`, id)
	return id
}

func (s *seed) posting(jdID, recruiterID uuid.UUID) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.job_postings (id, jd_id, recruiter_id, title) VALUES ($1, $2, $3, 'qb posting')`, id, jdID, recruiterID)
	s.later(`DELETE FROM public.job_postings WHERE id = $1`, id)
	return id
}

func (s *seed) roleProfile(jdID, candidateID uuid.UUID) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title) VALUES ($1, $2, $3, 'qb profile')`, id, jdID, candidateID)
	s.later(`DELETE FROM public.role_profiles WHERE id = $1`, id)
	return id
}

// jdWithPosting seeds a JD with a pending posting owned by the recruiter and, optionally, six
// confirmed requirements. Cleanup runs requirements, questions, posting, then the JD.
func (s *seed) jdWithPosting(levelID, recruiterID uuid.UUID, withRequirements bool) (jdID, postingID uuid.UUID) {
	jdID = s.jd(levelID, "Do not ask about Spring Boot.")
	postingID = s.posting(jdID, recruiterID)
	s.later(`DELETE FROM public.core_questions WHERE jd_id = $1`, jdID)
	if withRequirements {
		s.exec(`INSERT INTO public.requirements (jd_id, name, target, type) VALUES
			($1,'Go','backend services','required'), ($1,'PostgreSQL','data storage','required'),
			($1,'Kafka','event streaming','required'), ($1,'API design','REST APIs','required'),
			($1,'Automated testing','quality','required'), ($1,'Docker','deployment','preferred')`, jdID)
		s.later(`DELETE FROM public.requirements WHERE jd_id = $1`, jdID)
	}
	return jdID, postingID
}

func codeOf(err error) apperror.Code {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) && appErr != nil {
		return appErr.Code
	}
	return ""
}

func wantCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	if got := codeOf(err); got != want {
		t.Fatalf("error = %v (code %q), want code %q", err, got, want)
	}
}
