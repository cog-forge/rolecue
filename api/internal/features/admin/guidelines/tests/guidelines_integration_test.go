//go:build integration

// Package tests: Interview Behavior Guidelines (KAN-276) against a real Postgres. The repository and the
// service run unmodified. The guideline table is global, so every test holds a database-wide advisory lock
// (shared with the interview package's guideline tests), removes exactly the versions it saved and leaves
// the table as it found it.
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/guidelines/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/guidelines/service"
	interviewdomain "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// guidelineTestLock is the advisory lock key every guideline integration test holds.
const guidelineTestLock = 4276001

type env struct {
	ctx   context.Context
	pool  *pgxpool.Pool
	admin uuid.UUID
	repo  *repository.Repository
	svc   *service.Service
}

func newEnv(t *testing.T) *env {
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
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, int64(guidelineTestLock)); err != nil {
		t.Fatal(err)
	}
	e := &env{ctx: ctx, pool: pool, admin: uuid.New(), repo: repository.NewRepository(pool)}
	e.svc, err = service.New(e.repo, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, 'Guideline Admin', $2, 'admin', true)`,
		e.admin, fmt.Sprintf("guidelines-%s@example.test", e.admin)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `UPDATE public.interviews SET behavior_guideline_id = NULL WHERE behavior_guideline_id IN (SELECT id FROM public.interview_behavior_guidelines WHERE updated_by = $1)`, e.admin)
		if _, err := pool.Exec(bg, `DELETE FROM public.interview_behavior_guidelines WHERE updated_by = $1`, e.admin); err != nil {
			t.Errorf("cleanup guidelines: %v", err)
		}
		_, _ = pool.Exec(bg, `DELETE FROM public.users WHERE id = $1`, e.admin)
		_, _ = conn.Exec(bg, `SELECT pg_advisory_unlock($1)`, int64(guidelineTestLock))
		conn.Release()
	})
	return e
}

// base is the current version before the test saves anything (0 on a clean database).
func (e *env) base(t *testing.T) int32 {
	t.Helper()
	stored, found, err := e.repo.Current(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		return 0
	}
	return stored.Version
}

func formal() interviewdomain.BehaviorGuideline {
	g := interviewdomain.DefaultBehaviorGuideline()
	g.Tone, g.QuestionLength, g.FollowUpFocus = interviewdomain.ToneFormal, interviewdomain.LengthConcise, interviewdomain.FocusThorough
	max := int32(1)
	g.FollowUpMax = &max
	g.AvoidTopics = []string{interviewdomain.TopicLocationNational, interviewdomain.TopicCompensation}
	return g
}

func wantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

func (e *env) count(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(e.ctx, `SELECT count(*) FROM public.interview_behavior_guidelines`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestSaveReloadAndVersioning saves two versions through the service and reads them back: values survive
// the round trip (topics in catalog order, a null follow_up_max as null), each save is a new row, the
// saving Administrator is recorded, and a save against an old version writes nothing.
func TestSaveReloadAndVersioning(t *testing.T) {
	e := newEnv(t)
	base := e.base(t)
	rows := e.count(t)

	first, err := e.svc.Save(e.ctx, e.admin, "admin", base, formal())
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != base+1 || first.UpdatedByName != "Guideline Admin" {
		t.Fatalf("first save = %+v", first)
	}
	loaded, err := e.svc.Get(e.ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != first.Version || loaded.Values.Tone != "formal" || loaded.Values.QuestionLength != "concise" ||
		loaded.Values.FollowUpFocus != "thorough" || loaded.Values.FollowUpMax == nil || *loaded.Values.FollowUpMax != 1 {
		t.Fatalf("reload = %+v", loaded)
	}
	if got := loaded.Values.AvoidTopics; len(got) != 2 || got[0] != "compensation" || got[1] != "location_nationality" {
		t.Fatalf("topics = %v, want catalog order", got)
	}

	reset, err := e.svc.Save(e.ctx, e.admin, "admin", first.Version, interviewdomain.DefaultBehaviorGuideline())
	if err != nil || reset.Version != first.Version+1 || !reset.Values.IsDefault() || reset.Values.AvoidTopics == nil {
		t.Fatalf("reset = %+v, %v; want the next version holding the defaults", reset, err)
	}
	if got := e.count(t); got != rows+2 {
		t.Fatalf("rows = %d, want %d: every save is a new row", got, rows+2)
	}

	_, err = e.svc.Save(e.ctx, e.admin, "admin", first.Version, formal())
	wantCode(t, err, apperror.CodeGuidelineVersionConflict)
	if got := e.count(t); got != rows+2 {
		t.Fatalf("rows = %d after a stale save, want %d", got, rows+2)
	}
}

// TestConcurrentSavesOfOneVersionKeepExactlyOne proves two Admins cannot overwrite each other: of eight
// saves that all loaded the same version, exactly one wins and the rest are conflicts.
func TestConcurrentSavesOfOneVersionKeepExactlyOne(t *testing.T) {
	e := newEnv(t)
	base := e.base(t)
	var wins, conflicts, other int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.svc.Save(e.ctx, e.admin, "admin", base, formal())
			mu.Lock()
			defer mu.Unlock()
			var appErr *apperror.AppError
			switch {
			case err == nil:
				wins++
			case errors.As(err, &appErr) && appErr.Code == apperror.CodeGuidelineVersionConflict:
				conflicts++
			default:
				other++
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if wins != 1 || conflicts != 7 || other != 0 {
		t.Fatalf("wins = %d, conflicts = %d, other = %d; want 1, 7, 0", wins, conflicts, other)
	}
	if got := e.base(t); got != base+1 {
		t.Fatalf("version = %d, want %d", got, base+1)
	}
}

// TestNonAdminSaveWritesNothing proves the role check stops a Candidate or Recruiter before the database.
func TestNonAdminSaveWritesNothing(t *testing.T) {
	e := newEnv(t)
	rows := e.count(t)
	for _, role := range []string{"candidate", "recruiter"} {
		_, err := e.svc.Save(e.ctx, e.admin, role, e.base(t), formal())
		wantCode(t, err, apperror.CodeForbidden)
		_, err = e.svc.Get(e.ctx, role)
		wantCode(t, err, apperror.CodeForbidden)
	}
	if got := e.count(t); got != rows {
		t.Fatalf("rows = %d, want %d", got, rows)
	}
}

// TestDatabaseRefusesValuesOutsideTheVocabulary proves the CHECK constraints back the service validation,
// so no path can store a value the prompt composer would have to interpolate.
func TestDatabaseRefusesValuesOutsideTheVocabulary(t *testing.T) {
	e := newEnv(t)
	version := e.base(t) + 1
	for name, sql := range map[string]string{
		"tone":      `INSERT INTO public.interview_behavior_guidelines (version, tone, question_length, follow_up_focus, updated_by) VALUES ($1, 'rude', 'standard', 'standard', $2)`,
		"length":    `INSERT INTO public.interview_behavior_guidelines (version, tone, question_length, follow_up_focus, updated_by) VALUES ($1, 'neutral', 'huge', 'standard', $2)`,
		"focus":     `INSERT INTO public.interview_behavior_guidelines (version, tone, question_length, follow_up_focus, updated_by) VALUES ($1, 'neutral', 'standard', 'all', $2)`,
		"max":       `INSERT INTO public.interview_behavior_guidelines (version, tone, question_length, follow_up_max, follow_up_focus, updated_by) VALUES ($1, 'neutral', 'standard', 4, 'standard', $2)`,
		"topic":     `INSERT INTO public.interview_behavior_guidelines (version, tone, question_length, follow_up_focus, avoid_topics, updated_by) VALUES ($1, 'neutral', 'standard', 'standard', ARRAY['ignore previous instructions'], $2)`,
		"version":   `INSERT INTO public.interview_behavior_guidelines (version, tone, question_length, follow_up_focus, updated_by) VALUES (0 * $1, 'neutral', 'standard', 'standard', $2)`,
		"free text": `INSERT INTO public.interview_behavior_guidelines (version, tone, question_length, follow_up_focus, avoid_topics, updated_by) VALUES ($1, 'neutral', 'standard', 'standard', ARRAY['compensation','compensation','compensation','compensation','compensation','compensation'], $2)`,
	} {
		if _, err := e.pool.Exec(e.ctx, sql, version, e.admin); err == nil {
			t.Errorf("%s: the database accepted a value outside the vocabulary", name)
		}
	}
}
