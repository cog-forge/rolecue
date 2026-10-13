//go:build integration

package tests

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// openPool connects to the migrated database from DATABASE_URL, or skips the test.
func openPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

// seed inserts rows and removes exactly those rows when the test ends.
type seed struct {
	t     *testing.T
	ctx   context.Context
	pool  *pgxpool.Pool
	tag   string
	users []uuid.UUID
	undo  []func()
}

// newSeed registers the cleanup before the test writes anything. t.Cleanup runs last-in first-out,
// so dropJDs (registered second) runs before the users and levels are removed. It finds every
// JD this test touched, including half-written ones a rollback bug would leave behind: by owner,
// and by content_path, which always starts with the test's tag.
func newSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *seed {
	s := &seed{t: t, ctx: ctx, pool: pool, tag: "jdtest-" + uuid.NewString()[:8]}
	t.Cleanup(func() {
		for i := len(s.undo) - 1; i >= 0; i-- {
			s.undo[i]()
		}
	})
	t.Cleanup(s.dropJDs)
	return s
}

// key is a content_path owned by this test.
func (s *seed) key(name string) string { return s.tag + "/" + name }

// dropJDs deletes children before parents: interviews, core questions, requirements, role
// profiles, then the JDs.
func (s *seed) dropJDs() {
	ctx := context.Background()
	ids := `SELECT id FROM public.job_descriptions WHERE content_path LIKE $1 || '/%'
		UNION SELECT jd_id FROM public.role_profiles WHERE candidate_id = ANY($2)`
	for _, sql := range []string{
		`DELETE FROM public.interviews WHERE role_profile_id IN (SELECT id FROM public.role_profiles WHERE jd_id IN (` + ids + `))`,
		`DELETE FROM public.core_questions WHERE jd_id IN (` + ids + `)`,
		`DELETE FROM public.requirements WHERE jd_id IN (` + ids + `)`,
		`DELETE FROM public.role_profiles WHERE jd_id IN (` + ids + `)`,
		`DELETE FROM public.job_descriptions WHERE id IN (` + ids + `)`,
	} {
		if _, err := s.pool.Exec(ctx, sql, s.tag, s.users); err != nil {
			s.t.Errorf("cleanup %q: %v", sql, err)
		}
	}
}

func (s *seed) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.pool.Exec(s.ctx, sql, args...); err != nil {
		s.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (s *seed) later(sql string, args ...any) {
	s.undo = append(s.undo, func() {
		if _, err := s.pool.Exec(context.Background(), sql, args...); err != nil {
			s.t.Errorf("cleanup %q: %v", sql, err)
		}
	})
}

func (s *seed) user() uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, 'jd test', $2, 'candidate', true)`,
		id, fmt.Sprintf("jd-%s-%s@example.test", s.tag, id))
	s.later(`DELETE FROM public.users WHERE id = $1`, id)
	s.users = append(s.users, id)
	return id
}

// levels makes sure every extraction level exists, in case the database predates 000007.
func (s *seed) levels() {
	for _, name := range []string{"intern", "junior", "mid", "senior", "lead"} {
		id := uuid.New()
		tag, err := s.pool.Exec(s.ctx, `INSERT INTO public.levels (id, name) SELECT $1, $2
			WHERE NOT EXISTS (SELECT 1 FROM public.levels WHERE lower(name) = $2)`, id, name)
		if err != nil {
			s.t.Fatal(err)
		}
		if tag.RowsAffected() == 1 {
			s.later(`DELETE FROM public.levels WHERE id = $1`, id)
		}
	}
}

func (s *seed) count(sql string, args ...any) int {
	s.t.Helper()
	var n int
	if err := s.pool.QueryRow(s.ctx, sql, args...).Scan(&n); err != nil {
		s.t.Fatal(err)
	}
	return n
}

func wantCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != want {
		t.Fatalf("error = %v, want code %q", err, want)
	}
}

func strPtr(s string) *string { return &s }

// memoryStore stands in for object storage so the service can run against the real database.
// Its keys use the test's tag, so the cleanup finds the rows that reference them.
type memoryStore struct {
	prefix  string
	objects map[string]string
}

func (m *memoryStore) Upload(_ context.Context, body io.ReadSeeker, _ string) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	key := m.prefix + "/" + uuid.NewString()
	m.objects[key] = string(data)
	return key, nil
}

func (m *memoryStore) Download(_ context.Context, key string) (storage.Object, error) {
	text, ok := m.objects[key]
	if !ok {
		return storage.Object{}, storage.ErrNotFound
	}
	return storage.Object{Body: io.NopCloser(strings.NewReader(text))}, nil
}

// insertingBank writes core questions straight into the table, standing in for the AI generator.
type insertingBank struct{ pool *pgxpool.Pool }

func (b insertingBank) GenerateForPractice(ctx context.Context, _, jdID uuid.UUID) error {
	_, err := b.pool.Exec(ctx, `INSERT INTO public.core_questions (jd_id, content) VALUES ($1, 'SECRET QUESTION: explain Go channels')`, jdID)
	return err
}

func (insertingBank) Enabled() bool { return true }

func reviewed() domain.StructuredJD {
	level := domain.Senior
	return domain.StructuredJD{
		Title:          "Backend Engineer",
		SeniorityLevel: &level,
		Skills: []domain.ExtractedSkill{
			{Name: "Go", Category: domain.ProgrammingLanguage, Requirement: domain.Required, EvidenceText: strPtr("5+ years of Go")},
			{Name: "PostgreSQL", Category: domain.Database},
		},
		Technologies:    []string{"Kafka"},
		DomainKnowledge: []string{"Fintech"},
	}
}

// noExtractor satisfies the extraction dependency; these tests never call Analyze.
type noExtractor struct{}

func (noExtractor) Extract(context.Context, string) (domain.ExtractionCandidate, error) {
	return domain.ExtractionCandidate{}, errors.New("not used")
}
