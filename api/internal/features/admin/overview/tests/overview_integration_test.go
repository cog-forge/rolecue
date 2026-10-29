//go:build integration

// Package tests holds the slow, opt-in tests of the Admin overview. The counts are SQL aggregates over
// the whole database, and the database may already hold real rows, so no test asserts an absolute total:
// each test reads the overview before seeding, seeds its own rows, reads again and asserts the delta.
// Tests 6 to 8 need no database beyond what is listed. They need a migrated PostgreSQL database:
//
//	cd api && set -a && . ../.env && set +a
//	go test -tags=integration ./internal/features/admin/overview/tests -count=1
package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/overview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/admin/overview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/overview/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// windows are the windows the Admin can ask for; every windowed assertion runs against each of them.
var windows = []int{7, 30, 90}

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

// seeder inserts rows and deletes exactly those rows afterwards, in reverse order. Each child row is
// registered after its parent, so the reverse order removes children first.
type seeder struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	undo []func()
}

func newSeeder(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *seeder {
	s := &seeder{t: t, ctx: ctx, pool: pool}
	t.Cleanup(func() {
		for i := len(s.undo) - 1; i >= 0; i-- {
			s.undo[i]()
		}
	})
	return s
}

func (s *seeder) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.pool.Exec(s.ctx, sql, args...); err != nil {
		s.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (s *seeder) later(sql string, args ...any) {
	s.undo = append(s.undo, func() {
		if _, err := s.pool.Exec(context.Background(), sql, args...); err != nil {
			s.t.Errorf("cleanup %q: %v", sql, err)
		}
	})
}

// user inserts an account with a unique email and returns its id. Candidates and Recruiters get no
// lock; the caller adds one with an UPDATE when a test needs it.
func (s *seeder) user(role string) uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	var company *string
	if role == "recruiter" {
		acme := "Overview Co"
		company = &acme
	}
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified, company_name)
		VALUES ($1, $2, $3, $4, true, $5)`,
		id, "overview "+role, fmt.Sprintf("overview-%s@example.test", id), role, company)
	s.later(`DELETE FROM public.users WHERE id = $1`, id)
	return id
}

func (s *seeder) levelID() uuid.UUID {
	s.t.Helper()
	var id uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `SELECT id FROM public.levels ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		s.t.Fatalf("no seniority level seeded: %v", err)
	}
	return id
}

func (s *seeder) jobDescription() uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, content_path, level_id) VALUES ($1, $2, $3)`,
		id, "job-descriptions/"+id.String()+".pdf", s.levelID())
	s.later(`DELETE FROM public.job_descriptions WHERE id = $1`, id)
	return id
}

// posting inserts a Job Posting with the given status; it has no interview slot.
func (s *seeder) posting(recruiterID uuid.UUID, status string) uuid.UUID {
	s.t.Helper()
	jdID := s.jobDescription()
	id := uuid.New()
	s.exec(`INSERT INTO public.job_postings (id, status, jd_id, recruiter_id, title)
		VALUES ($1, $2::public.job_posting_status, $3, $4, 'overview posting')`,
		id, status, jdID, recruiterID)
	s.later(`DELETE FROM public.job_postings WHERE id = $1`, id)
	return id
}

// roleProfile inserts the Role Profile a practice interview needs (migration 000012).
func (s *seeder) roleProfile(candidateID uuid.UUID) uuid.UUID {
	s.t.Helper()
	jdID := s.jobDescription()
	id := uuid.New()
	s.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title) VALUES ($1, $2, $3, 'overview role')`,
		id, jdID, candidateID)
	s.later(`DELETE FROM public.role_profiles WHERE id = $1`, id)
	return id
}

// application inserts an Application with the given status and created_at.
func (s *seeder) application(postingID, candidateID uuid.UUID, status string, createdAt time.Time) uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	s.exec(`INSERT INTO public.applications (id, job_posting_id, candidate_id, cv_path, status, created_at)
		VALUES ($1, $2, $3, 'cvs/overview.pdf', $4::public.application_status, $5)`,
		id, postingID, candidateID, status, createdAt)
	s.later(`DELETE FROM public.applications WHERE id = $1`, id)
	return id
}

// interview inserts an interview session. roleProfileID is set for practice and nil for recruitment;
// applicationID is the reverse. completedAt is a time.Time or nil. Both are typed any so nil becomes NULL.
func (s *seeder) interview(typ string, roleProfileID, applicationID any, startedAt time.Time, completedAt any, evaluation string) {
	s.t.Helper()
	id := uuid.New()
	s.exec(`INSERT INTO public.interviews
		(id, role_profile_id, application_id, type, started_at, completed_at, evaluation_status)
		VALUES ($1, $2, $3, $4::public.interview_type, $5, $6, $7)`,
		id, roleProfileID, applicationID, typ, startedAt, completedAt, evaluation)
	s.later(`DELETE FROM public.interviews WHERE id = $1`, id)
}

// fakeFinance stands in for the wallet billing summary, which is not under test here.
type fakeFinance struct{ calls int }

func (f *fakeFinance) Summary(context.Context, time.Time, time.Time) (domain.OverviewFinance, error) {
	f.calls++
	return domain.OverviewFinance{}, nil
}

// countingRepository records every call and never touches a database. The role and days checks must
// reject a request before any of these methods run.
type countingRepository struct{ calls int }

func (r *countingRepository) Accounts(context.Context, time.Time, time.Time) (domain.OverviewAccounts, error) {
	r.calls++
	return domain.OverviewAccounts{}, nil
}

func (r *countingRepository) Postings(context.Context) (domain.OverviewPostings, error) {
	r.calls++
	return domain.OverviewPostings{}, nil
}

func (r *countingRepository) Applications(context.Context, time.Time, time.Time) (domain.OverviewApplications, error) {
	r.calls++
	return domain.OverviewApplications{}, nil
}

func (r *countingRepository) Interviews(context.Context, time.Time, time.Time) (domain.OverviewInterviews, error) {
	r.calls++
	return domain.OverviewInterviews{}, nil
}

func newService(t *testing.T, pool *pgxpool.Pool) *service.Service {
	t.Helper()
	svc, err := service.New(repository.NewRepository(pool), &fakeFinance{})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// read returns the overview for an Admin caller. Tests always read as the Admin, so the role check passes.
func read(t *testing.T, ctx context.Context, svc *service.Service, days int) domain.Overview {
	t.Helper()
	got, err := svc.Get(ctx, "admin", days)
	if err != nil {
		t.Fatalf("Get(admin, %d): %v", days, err)
	}
	return got
}

func assertDelta(t *testing.T, field string, got, want int64) {
	t.Helper()
	if got != want {
		t.Errorf("%s delta = %d, want %d", field, got, want)
	}
}

func postingCounts(p domain.OverviewPostings) map[string]int64 {
	return map[string]int64{
		"pending":       p.Pending,
		"open":          p.Open,
		"intake_closed": p.IntakeClosed,
		"closed":        p.Closed,
		"rejected":      p.Rejected,
	}
}

func applicationStatusCounts(a domain.OverviewApplications) map[string]int64 {
	return map[string]int64{
		"pending":            a.Pending,
		"interview_eligible": a.InterviewEligible,
		"interviewed":        a.Interviewed,
		"approved":           a.Approved,
		"rejected":           a.Rejected,
	}
}

func TestOverviewAccountsCountCandidatesAndRecruitersWithActiveLocks(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeeder(t, ctx, pool)
	svc := newService(t, pool)
	before := read(t, ctx, svc, 30).Accounts

	s.user("candidate")
	s.user("candidate")
	s.user("recruiter")
	admin := s.user("admin")
	s.exec(`UPDATE public.users SET is_locked = true, lock_expires_at = NULL WHERE id = $1`, admin)

	indefinite := s.user("candidate")
	s.exec(`UPDATE public.users SET is_locked = true, lock_expires_at = NULL WHERE id = $1`, indefinite)
	expired := s.user("candidate")
	s.exec(`UPDATE public.users SET is_locked = true, lock_expires_at = now() - interval '1 hour' WHERE id = $1`, expired)
	future := s.user("candidate")
	s.exec(`UPDATE public.users SET is_locked = true, lock_expires_at = now() + interval '1 day' WHERE id = $1`, future)

	after := read(t, ctx, svc, 30).Accounts
	// 2 plain candidates + 3 locked candidates; the Admin is not a candidate.
	assertDelta(t, "accounts.candidates", after.Candidates-before.Candidates, 5)
	assertDelta(t, "accounts.recruiters", after.Recruiters-before.Recruiters, 1)
	// Only the indefinite lock and the future lock are active. The expired lock and the locked Admin are not counted.
	assertDelta(t, "accounts.locked", after.Locked-before.Locked, 2)
	// All six seeded Candidates and Recruiters were created now; the Admin is not counted.
	assertDelta(t, "accounts.new_in_window", after.NewInWindow-before.NewInWindow, 6)
}

func TestOverviewNewInWindowFollowsTheWindow(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeeder(t, ctx, pool)
	svc := newService(t, pool)
	before := map[int]int64{}
	for _, d := range windows {
		before[d] = read(t, ctx, svc, d).Accounts.NewInWindow
	}

	id := s.user("candidate")
	s.exec(`UPDATE public.users SET created_at = now() - interval '10 days' WHERE id = $1`, id)

	// Ten days ago is outside a 7-day window and inside the 30- and 90-day windows.
	want := map[int]int64{7: 0, 30: 1, 90: 1}
	for _, d := range windows {
		got := read(t, ctx, svc, d).Accounts.NewInWindow - before[d]
		assertDelta(t, fmt.Sprintf("accounts.new_in_window (days=%d)", d), got, want[d])
	}
}

func TestOverviewJobPostingsCountEachStatusAllTime(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeeder(t, ctx, pool)
	svc := newService(t, pool)
	before := postingCounts(read(t, ctx, svc, 30).JobPostings)

	recruiter := s.user("recruiter")
	statuses := []string{"pending", "open", "intake_closed", "closed", "rejected"}
	for _, st := range statuses {
		s.posting(recruiter, st)
	}

	after := postingCounts(read(t, ctx, svc, 30).JobPostings)
	for _, st := range statuses {
		assertDelta(t, "job_postings."+st, after[st]-before[st], 1)
	}
}

func TestOverviewApplicationsCountStatusesAndSubmittedInWindow(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeeder(t, ctx, pool)
	svc := newService(t, pool)
	beforeStatus := applicationStatusCounts(read(t, ctx, svc, 30).Applications)
	beforeSubmitted := map[int]int64{}
	for _, d := range windows {
		beforeSubmitted[d] = read(t, ctx, svc, d).Applications.SubmittedInWindow
	}

	candidate := s.user("candidate")
	recruiter := s.user("recruiter")
	posting := s.posting(recruiter, "open")
	now := time.Now()
	for _, st := range []string{"pending", "interview_eligible", "interviewed", "approved", "rejected"} {
		s.application(posting, candidate, st, now)
	}
	// Created 40 days ago: outside the 30-day window, inside the 90-day window. Its status still counts all-time.
	s.application(posting, candidate, "approved", now.Add(-40*24*time.Hour))

	afterStatus := applicationStatusCounts(read(t, ctx, svc, 30).Applications)
	wantStatus := map[string]int64{"pending": 1, "interview_eligible": 1, "interviewed": 1, "approved": 2, "rejected": 1}
	for st, want := range wantStatus {
		assertDelta(t, "applications."+st, afterStatus[st]-beforeStatus[st], want)
	}

	// Five recent applications are submitted in every window; the 40-day-old one joins only at 90 days.
	wantSubmitted := map[int]int64{7: 5, 30: 5, 90: 6}
	for _, d := range windows {
		got := read(t, ctx, svc, d).Applications.SubmittedInWindow - beforeSubmitted[d]
		assertDelta(t, fmt.Sprintf("applications.submitted_in_window (days=%d)", d), got, wantSubmitted[d])
	}
}

func TestOverviewInterviewsCountStartsCompletionsAndFailures(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeeder(t, ctx, pool)
	svc := newService(t, pool)
	beforeInterviews := map[int]domainInterviews{}
	for _, d := range windows {
		beforeInterviews[d] = read(t, ctx, svc, d).Interviews
	}

	candidate := s.user("candidate")
	recruiter := s.user("recruiter")
	posting := s.posting(recruiter, "open")
	now := time.Now()

	// Practice started a day ago and completed inside the window, evaluation completed.
	s.interview("practice", s.roleProfile(candidate), nil, now.Add(-24*time.Hour), now.Add(-23*time.Hour-30*time.Minute), "completed")
	// Practice started two days ago, still running, evaluation pending.
	s.interview("practice", s.roleProfile(candidate), nil, now.Add(-48*time.Hour), nil, "pending")
	// Recruitment started three days ago, completed two days ago, evaluation failed.
	app := s.application(posting, candidate, "interviewed", now)
	s.interview("recruitment", nil, app, now.Add(-72*time.Hour), now.Add(-48*time.Hour), "failed")
	// Started and completed 100 days ago: outside every window, but its failed evaluation counts all-time.
	s.interview("practice", s.roleProfile(candidate), nil,
		now.Add(-100*24*time.Hour), now.Add(-100*24*time.Hour+time.Hour), "failed")

	// Practice started: 2 (the 100-day-old one is excluded). Recruitment started: 1. Completed: 2
	// (practice completed a day ago and the recruitment completed two days ago). Evaluation failed: 2, all-time.
	want := domainInterviews{PracticeStartedInWindow: 2, RecruitmentStartedInWindow: 1, CompletedInWindow: 2, EvaluationFailed: 2}
	for _, d := range windows {
		after := read(t, ctx, svc, d).Interviews
		b := beforeInterviews[d]
		assertDelta(t, fmt.Sprintf("interviews.practice_started_in_window (days=%d)", d), after.PracticeStartedInWindow-b.PracticeStartedInWindow, want.PracticeStartedInWindow)
		assertDelta(t, fmt.Sprintf("interviews.recruitment_started_in_window (days=%d)", d), after.RecruitmentStartedInWindow-b.RecruitmentStartedInWindow, want.RecruitmentStartedInWindow)
		assertDelta(t, fmt.Sprintf("interviews.completed_in_window (days=%d)", d), after.CompletedInWindow-b.CompletedInWindow, want.CompletedInWindow)
		assertDelta(t, fmt.Sprintf("interviews.evaluation_failed (days=%d)", d), after.EvaluationFailed-b.EvaluationFailed, want.EvaluationFailed)
	}
}

// domainInterviews aliases the interview aggregate so the expected values read as a literal.
type domainInterviews = domain.OverviewInterviews

func TestOverviewResponseCarriesEveryKeyAsANumber(t *testing.T) {
	ctx, pool := openPool(t)
	svc := newService(t, pool)

	// days 0 means the default 30-day window.
	got := read(t, ctx, svc, 0)
	if got.Window.Days != 30 {
		t.Fatalf("window.days = %d for days=0, want 30", got.Window.Days)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	requireKeys(t, doc, "", "generated_at", "window", "accounts", "job_postings", "applications", "interviews", "finance")
	requireKeys(t, doc, "window", "days", "from", "to")
	requireNumericKeys(t, doc, "accounts", "candidates", "recruiters", "locked", "new_in_window")
	requireNumericKeys(t, doc, "job_postings", "pending", "open", "intake_closed", "closed", "rejected")
	requireNumericKeys(t, doc, "applications", "pending", "interview_eligible", "interviewed", "approved", "rejected", "submitted_in_window")
	requireNumericKeys(t, doc, "interviews", "practice_started_in_window", "recruitment_started_in_window", "completed_in_window", "evaluation_failed")
	requireNumericKeys(t, doc, "finance", "revenue_vnd", "successful_deposits", "coins_sold", "coins_spent", "coins_refunded", "unique_payers", "success_rate")
}

// requireKeys checks that the object at section (or the top level when section is empty) has exactly
// the documented keys.
func requireKeys(t *testing.T, doc map[string]any, section string, keys ...string) {
	t.Helper()
	obj := doc
	if section != "" {
		m, ok := doc[section].(map[string]any)
		if !ok {
			t.Fatalf("%s is missing or not an object", section)
		}
		obj = m
	}
	for _, k := range keys {
		if _, ok := obj[k]; !ok {
			t.Errorf("%s: key %q is missing", section, k)
		}
	}
	if len(obj) != len(keys) {
		t.Errorf("%s: has %d keys, want %d", section, len(obj), len(keys))
	}
}

func requireNumericKeys(t *testing.T, doc map[string]any, section string, keys ...string) {
	t.Helper()
	requireKeys(t, doc, section, keys...)
	obj := doc[section].(map[string]any)
	for _, k := range keys {
		if _, ok := obj[k].(float64); !ok {
			t.Errorf("%s.%s = %#v, want a number", section, k, obj[k])
		}
	}
}

func TestOverviewRejectsNonAdminBeforeQuerying(t *testing.T) {
	for _, role := range []string{"candidate", "recruiter", ""} {
		t.Run("role="+role, func(t *testing.T) {
			repo := &countingRepository{}
			finance := &fakeFinance{}
			svc, err := service.New(repo, finance)
			if err != nil {
				t.Fatal(err)
			}
			_, err = svc.Get(context.Background(), role, 30)
			if !apperror.IsCode(err, apperror.CodeForbidden) {
				t.Fatalf("Get(%q) error = %v, want FORBIDDEN", role, err)
			}
			if repo.calls != 0 || finance.calls != 0 {
				t.Fatalf("repository calls = %d, finance calls = %d, want none", repo.calls, finance.calls)
			}
		})
	}
}

func TestOverviewRejectsUnsupportedWindowBeforeQuerying(t *testing.T) {
	for _, days := range []int{5, 1, 60, -7} {
		t.Run(fmt.Sprintf("days=%d", days), func(t *testing.T) {
			repo := &countingRepository{}
			finance := &fakeFinance{}
			svc, err := service.New(repo, finance)
			if err != nil {
				t.Fatal(err)
			}
			_, err = svc.Get(context.Background(), "admin", days)
			if !apperror.IsCode(err, apperror.CodeValidation) {
				t.Fatalf("Get(admin, %d) error = %v, want VALIDATION_ERROR", days, err)
			}
			if repo.calls != 0 || finance.calls != 0 {
				t.Fatalf("repository calls = %d, finance calls = %d, want none", repo.calls, finance.calls)
			}
		})
	}
	// The supported windows are accepted and span exactly that many days.
	for _, days := range windows {
		repo := &countingRepository{}
		svc, err := service.New(repo, &fakeFinance{})
		if err != nil {
			t.Fatal(err)
		}
		got, err := svc.Get(context.Background(), "admin", days)
		if err != nil {
			t.Fatalf("Get(admin, %d): %v", days, err)
		}
		if got.Window.Days != days || got.Window.To.Sub(got.Window.From) != time.Duration(days)*24*time.Hour {
			t.Fatalf("window for days=%d = %+v", days, got.Window)
		}
	}
}

func TestOverviewJSONCarriesNoPersonalData(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeeder(t, ctx, pool)
	svc := newService(t, pool)

	tag := uuid.NewString()
	name := "Privacy Probe " + tag
	email := "overview-privacy-" + tag + "@example.test"
	company := "Probe Co " + tag
	id := uuid.New()
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified, company_name)
		VALUES ($1, $2, $3, 'recruiter', true, $4)`, id, name, email, company)
	s.later(`DELETE FROM public.users WHERE id = $1`, id)

	raw, err := json.Marshal(read(t, ctx, svc, 30))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, personal := range []string{email, name, company, tag, "example.test"} {
		if strings.Contains(body, personal) {
			t.Fatalf("overview JSON contains personal data %q", personal)
		}
	}
}
