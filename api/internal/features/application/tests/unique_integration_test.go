//go:build integration

package tests

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// KAN-268: the database itself refuses a second Application of one Candidate to one posting, whatever
// path writes it. The apply path's advisory lock and check stay in front of this.
func TestSecondApplicationRowViolatesTheUniqueIndex(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	candidate := s.user("candidate")
	postingID := s.posting(s.user("recruiter"), "open", slot(3))
	s.application(postingID, candidate)

	_, err := pool.Exec(ctx, `INSERT INTO public.applications (job_posting_id, candidate_id, cv_path) VALUES ($1, $2, 'cvs/dup.pdf')`,
		postingID, candidate)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "applications_posting_candidate_key" {
		t.Fatalf("duplicate insert error = %v, want a unique violation of applications_posting_candidate_key", err)
	}

	// Another Candidate on the same posting, and the same Candidate on another posting, are fine.
	s.application(postingID, s.user("candidate"))
	s.application(s.posting(s.user("recruiter"), "open", slot(3)), candidate)
}

// The migration removes existing duplicates deterministically before it builds the index: the
// Application with an interview survives, otherwise the earliest one does.
func TestApplicationUniqueMigrationDedupesDeterministically(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	migrations := filepath.Join("..", "..", "..", "..", "migrations")
	run := func(file string) {
		t.Helper()
		sql, err := os.ReadFile(filepath.Join(migrations, file))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}
	// Leave the index in place whatever happens, so later tests see the real schema.
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM public.applications WHERE cv_path = 'cvs/dedupe.pdf'`)
		_, _ = pool.Exec(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS applications_posting_candidate_key ON public.applications (job_posting_id, candidate_id)`)
	})
	run("000025_application_unique.down.sql")

	candidate, other := s.user("candidate"), s.user("candidate")
	postingID := s.posting(s.user("recruiter"), "open", slot(3))
	base := time.Now().Add(-time.Hour)
	insert := func(candidateID uuid.UUID, age time.Duration) uuid.UUID {
		id := uuid.New()
		s.exec(`INSERT INTO public.applications (id, job_posting_id, candidate_id, cv_path, created_at)
			VALUES ($1, $2, $3, 'cvs/dedupe.pdf', $4)`, id, postingID, candidateID, base.Add(age))
		return id
	}
	earliest, withInterview, newest := insert(candidate, 0), insert(candidate, time.Minute), insert(candidate, 2*time.Minute)
	s.interview(withInterview)
	first, second := insert(other, 0), insert(other, time.Minute)

	run("000025_application_unique.up.sql")

	exists := func(id uuid.UUID) bool {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.applications WHERE id = $1`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}
	if exists(earliest) || !exists(withInterview) || exists(newest) {
		t.Fatalf("pair with an interview: earliest kept=%v, with-interview kept=%v, newest kept=%v, want only the one with the interview",
			exists(earliest), exists(withInterview), exists(newest))
	}
	if !exists(first) || exists(second) {
		t.Fatalf("pair without interviews: first kept=%v, second kept=%v, want only the earliest", exists(first), exists(second))
	}
}
