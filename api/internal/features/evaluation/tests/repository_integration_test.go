//go:build integration

// Package tests holds the slow, opt-in tests of posting evaluation weights. Ownership, the edit
// window under the row lock and the all-or-nothing replace live in SQL, so only a real database can
// prove them. They need a migrated PostgreSQL database (including the 000011 metrics seed):
//
//	cd api && set -a && . ../.env && set +a
//	go test -tags=integration ./internal/features/evaluation/tests -count=1
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	accuracyID = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	depthID    = uuid.MustParse("00000000-0000-4000-8000-000000000102")
)

func openPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

// seed inserts rows and deletes exactly those rows afterwards, in reverse order.
type seed struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	undo []func()
}

func newSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *seed {
	s := &seed{t: t, ctx: ctx, pool: pool}
	t.Cleanup(func() {
		for i := len(s.undo) - 1; i >= 0; i-- {
			s.undo[i]()
		}
	})
	return s
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

func (s *seed) recruiter() uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified, company_name)
		VALUES ($1, 'ev recruiter', $2, 'recruiter', true, 'Acme')`, id, fmt.Sprintf("ev-%s@example.test", id))
	s.later(`DELETE FROM public.users WHERE id = $1`, id)
	return id
}

// posting adds a posting with one core question, so the tests can show weights never touch it.
func (s *seed) posting(recruiterID uuid.UUID, status string) (uuid.UUID, uuid.UUID) {
	s.t.Helper()
	var levelID uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `SELECT id FROM public.levels ORDER BY id LIMIT 1`).Scan(&levelID); err != nil {
		s.t.Fatalf("no seniority level seeded: %v", err)
	}
	jdID, id := uuid.New(), uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, content_path, level_id) VALUES ($1, $2, $3)`,
		jdID, "job-postings/"+jdID.String()+".pdf", levelID)
	s.later(`DELETE FROM public.job_descriptions WHERE id = $1`, jdID)
	s.exec(`INSERT INTO public.core_questions (jd_id, content) VALUES ($1, 'kept question')`, jdID)
	s.later(`DELETE FROM public.core_questions WHERE jd_id = $1`, jdID)
	s.exec(`INSERT INTO public.job_postings (id, status, jd_id, recruiter_id, title)
		VALUES ($1, $2::public.job_posting_status, $3, $4, 'ev posting')`, id, status, jdID, recruiterID)
	s.later(`DELETE FROM public.job_postings WHERE id = $1`, id)
	s.later(`DELETE FROM public.metrics_percentage WHERE job_posting_id = $1`, id)
	return id, jdID
}

func (s *seed) questions(jdID uuid.UUID) []string {
	s.t.Helper()
	rows, err := s.pool.Query(s.ctx, `SELECT content FROM public.core_questions WHERE jd_id = $1 ORDER BY content`, jdID)
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			s.t.Fatal(err)
		}
		out = append(out, content)
	}
	return out
}

func weightsInput(pairs ...any) []domain.WeightInput {
	out := []domain.WeightInput{}
	for i := 0; i < len(pairs); i += 2 {
		id, percentage := pairs[i].(uuid.UUID), pairs[i+1].(float64)
		out = append(out, domain.WeightInput{MetricID: &id, Percentage: &percentage})
	}
	return out
}

func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != want {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

func newService(t *testing.T, pool *pgxpool.Pool) (*service.Service, *repository.Repository) {
	t.Helper()
	repo := repository.NewRepository(pool)
	svc, err := service.New(repo, 100)
	if err != nil {
		t.Fatal(err)
	}
	return svc, repo
}

func TestMetricsCatalogIsSeeded(t *testing.T) {
	ctx, pool := openPool(t)
	svc, _ := newService(t, pool)
	metrics, err := svc.Metrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, metric := range metrics {
		names[metric.Name] = true
	}
	for _, want := range []string{"Technical Accuracy", "Depth of Understanding", "Problem-Solving & Approach", "Answer Relevance", "Communication Clarity"} {
		if !names[want] {
			t.Fatalf("metrics = %+v, missing %q", metrics, want)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.metrics (name) VALUES ('technical accuracy')`); err == nil {
		_, _ = pool.Exec(context.Background(), `DELETE FROM public.metrics WHERE name = 'technical accuracy'`)
		t.Fatal("a metric name differing only by case was accepted")
	}
}

func TestReplaceAndReadOwnPosting(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, _ := newService(t, pool)
	owner := s.recruiter()
	postingID, jdID := s.posting(owner, "pending")

	before, err := svc.Weights(ctx, owner, "recruiter", postingID)
	if err != nil || before.Configured || !before.UsingDefaults || len(before.Weights) < 5 {
		t.Fatalf("Weights() before = %+v, %v, want not configured with the default weights", before, err)
	}
	var stored int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.metrics_percentage WHERE job_posting_id = $1`, postingID).Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("stored weights after reading defaults = %d, %v, want 0 (defaults are never saved)", stored, err)
	}
	if _, err := svc.ReplaceWeights(ctx, owner, "recruiter", postingID, weightsInput(accuracyID, 70.0, depthID, 30.0)); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ReplaceWeights(ctx, owner, "recruiter", postingID, weightsInput(accuracyID, 100.0))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Configured || len(got.Weights) != 1 || got.Weights[0].MetricID != accuracyID || got.Weights[0].Percentage != 100 {
		t.Fatalf("ReplaceWeights() = %+v, want only Technical Accuracy at 100 (the old set replaced)", got)
	}
	if questions := s.questions(jdID); len(questions) != 1 || questions[0] != "kept question" {
		t.Fatalf("core questions = %v, want them untouched by weights", questions)
	}
}

func TestOtherRecruitersSeeNotFound(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, _ := newService(t, pool)
	postingID, _ := s.posting(s.recruiter(), "pending")
	other := s.recruiter()

	_, err := svc.Weights(ctx, other, "recruiter", postingID)
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	_, err = svc.ReplaceWeights(ctx, other, "recruiter", postingID, weightsInput(accuracyID, 100.0))
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	_, err = svc.Weights(ctx, other, "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeJobPostingNotFound)
}

func TestNonPendingPostingIsLockedUnderTheRowLock(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	_, repo := newService(t, pool)
	owner := s.recruiter()
	for _, status := range []string{"open", "intake_closed", "closed", "rejected"} {
		postingID, _ := s.posting(owner, status)
		// Call the repository directly, as if the posting was approved after the service read it.
		_, err := repo.ReplaceWeights(ctx, owner, postingID, []domain.Weight{{MetricID: accuracyID, Percentage: 100}})
		assertCode(t, err, apperror.CodeEvaluationWeightsLocked)
	}
}

func TestFailedReplaceKeepsTheOldSet(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, repo := newService(t, pool)
	owner := s.recruiter()
	postingID, _ := s.posting(owner, "pending")
	if _, err := svc.ReplaceWeights(ctx, owner, "recruiter", postingID, weightsInput(accuracyID, 60.0, depthID, 40.0)); err != nil {
		t.Fatal(err)
	}
	// The second metric does not exist, so its insert fails after the delete and the first insert.
	_, err := repo.ReplaceWeights(ctx, owner, postingID, []domain.Weight{
		{MetricID: accuracyID, Percentage: 50}, {MetricID: uuid.New(), Percentage: 50},
	})
	assertCode(t, err, apperror.CodeEvaluationMetricUnknown)
	got, err := svc.Weights(ctx, owner, "recruiter", postingID)
	if err != nil || len(got.Weights) != 2 {
		t.Fatalf("Weights() after a failed replace = %+v, %v, want the previous two weights", got, err)
	}
}
