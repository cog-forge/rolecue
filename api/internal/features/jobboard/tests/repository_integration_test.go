//go:build integration

// Package tests holds the slow, opt-in tests of the job board. Visibility by status lives in SQL,
// so only a real database can prove it. They need a migrated PostgreSQL database:
//
//	cd api && set -a && . ../.env && set +a
//	go test -tags=integration ./internal/features/jobboard/tests -count=1
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jobboard/domain"
	"github.com/cog-forge/rolecue/api/internal/features/jobboard/repository"
	"github.com/cog-forge/rolecue/api/internal/features/jobboard/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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

// seed inserts tagged rows and deletes exactly those rows afterwards, in reverse order. Every title
// contains the tag, so searching for it hides postings other tests or demo data left behind.
type seed struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	tag  string
	undo []func()
}

func newSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *seed {
	s := &seed{t: t, ctx: ctx, pool: pool, tag: "jb" + uuid.NewString()[:8]}
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

// level returns the id of a seniority level, adding it if the database is missing it.
func (s *seed) level(name string) uuid.UUID {
	s.t.Helper()
	var id uuid.UUID
	err := s.pool.QueryRow(s.ctx, `SELECT id FROM public.levels WHERE lower(name) = $1 ORDER BY id LIMIT 1`, name).Scan(&id)
	if err == nil {
		return id
	}
	if err := s.pool.QueryRow(s.ctx, `INSERT INTO public.levels (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		s.t.Fatal(err)
	}
	s.later(`DELETE FROM public.levels WHERE id = $1`, id)
	return id
}

func (s *seed) user(role string, company *string) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified, company_name, company_website)
		VALUES ($1, $2, $3, $4, true, $5, 'https://acme.test')`,
		id, "jb "+role, fmt.Sprintf("jb-%s-%s@example.test", s.tag, id), role, company)
	s.later(`DELETE FROM public.users WHERE id = $1`, id)
	return id
}

type postingSpec struct {
	title, status        string
	location, employment *string
	rawText              *string
	createdAt            time.Time
	requirements         []string
	slot                 int32
	rejectReason         *string
}

func (s *seed) posting(recruiter uuid.UUID, p postingSpec) uuid.UUID {
	s.t.Helper()
	jdID, id := uuid.New(), uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, content_path, raw_text, level_id) VALUES ($1, $2, $3, $4)`,
		jdID, "job-postings/"+jdID.String()+".pdf", p.rawText, s.level("mid"))
	s.later(`DELETE FROM public.job_descriptions WHERE id = $1`, jdID)
	for _, name := range p.requirements {
		s.exec(`INSERT INTO public.requirements (jd_id, name, target, type) VALUES ($1, $2, 'programming_language', 'required')`, jdID, name)
	}
	s.later(`DELETE FROM public.requirements WHERE jd_id = $1`, jdID)
	s.exec(`INSERT INTO public.core_questions (jd_id, content) VALUES ($1, 'secret question')`, jdID)
	s.later(`DELETE FROM public.core_questions WHERE jd_id = $1`, jdID)
	s.exec(`INSERT INTO public.job_postings (id, status, jd_id, recruiter_id, title, location, employment_type,
			salary_min, salary_max, interview_duration, interview_slot, reject_reason, created_at)
		VALUES ($1, $2::public.job_posting_status, $3, $4, $5, $6, $7, 1000, 2000, 30, $8, $9, $10)`,
		id, p.status, jdID, recruiter, s.tag+" "+p.title, p.location, p.employment, p.slot, p.rejectReason, p.createdAt)
	s.later(`DELETE FROM public.job_postings WHERE id = $1`, id)
	return id
}

func (s *seed) application(postingID, candidateID uuid.UUID) {
	id := uuid.New()
	s.exec(`INSERT INTO public.applications (id, job_posting_id, candidate_id) VALUES ($1, $2, $3)`, id, postingID, candidateID)
	s.later(`DELETE FROM public.applications WHERE id = $1`, id)
}

func codeOf(err error) apperror.Code {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func ptr[T any](v T) *T { return &v }

func ids(items []domain.BoardPosting) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func sameIDs(t *testing.T, got []domain.BoardPosting, want ...uuid.UUID) {
	t.Helper()
	g := ids(got)
	if len(g) != len(want) {
		t.Fatalf("got %v, want %v", g, want)
	}
	for i := range want {
		if g[i] != want[i] {
			t.Fatalf("got %v, want %v (order matters)", g, want)
		}
	}
}

type board struct {
	repo      *repository.Repository
	seed      *seed
	recruiter uuid.UUID
	candidate uuid.UUID
	base      time.Time
}

func newBoard(t *testing.T) (context.Context, *board) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	return ctx, &board{
		repo: repository.NewRepository(pool), seed: s,
		recruiter: s.user("recruiter", ptr("Acme")), candidate: s.user("candidate", nil),
		base: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
	}
}

// filter searches for the seed tag; LIKE needs no escaping because the tag is letters and digits.
func (b *board) filter(limit, offset int32) domain.BoardFilter {
	return domain.BoardFilter{Search: &b.seed.tag, Limit: limit, Offset: offset}
}

func TestOnlyOpenPostingsAreDiscoverable(t *testing.T) {
	ctx, b := newBoard(t)
	hidden := map[string]uuid.UUID{}
	for i, status := range []string{"pending", "rejected", "intake_closed", "closed"} {
		hidden[status] = b.seed.posting(b.recruiter, postingSpec{title: status, status: status, rawText: ptr("jd"),
			createdAt: b.base.Add(time.Duration(i+10) * time.Hour)})
	}
	open := b.seed.posting(b.recruiter, postingSpec{title: "open", status: "open", rawText: ptr("jd"), createdAt: b.base})

	items, total, err := b.repo.ListOpen(ctx, b.filter(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	sameIDs(t, items, open)
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if _, err := b.repo.GetOpen(ctx, b.candidate, open); err != nil {
		t.Fatalf("open posting: %v", err)
	}
	for status, id := range hidden {
		if _, err := b.repo.GetOpen(ctx, b.candidate, id); codeOf(err) != apperror.CodeJobPostingNotFound {
			t.Fatalf("%s posting: error = %v, want %s", status, err, apperror.CodeJobPostingNotFound)
		}
	}
	// A posting that does not exist looks exactly like one that is not open.
	_, missing := b.repo.GetOpen(ctx, b.candidate, uuid.New())
	_, pending := b.repo.GetOpen(ctx, b.candidate, hidden["pending"])
	if codeOf(missing) != codeOf(pending) || missing.Error() != pending.Error() {
		t.Fatalf("missing = %v, pending = %v; want the same error", missing, pending)
	}
}

func TestLockedRecruitersPostingsAreHidden(t *testing.T) {
	ctx, b := newBoard(t)
	lock := func(recruiter uuid.UUID, locked bool, expires *time.Time) {
		b.seed.exec(`UPDATE public.users SET is_locked = $2, lock_expires_at = $3 WHERE id = $1`, recruiter, locked, expires)
	}
	postingOf := func(recruiter uuid.UUID, title string, hour int) uuid.UUID {
		return b.seed.posting(recruiter, postingSpec{title: title, status: "open", rawText: ptr("jd"),
			createdAt: b.base.Add(time.Duration(hour) * time.Hour)})
	}
	locked := b.seed.user("recruiter", ptr("Locked Co"))
	timed := b.seed.user("recruiter", ptr("Timed Co"))
	expired := b.seed.user("recruiter", ptr("Expired Co"))
	lockedPosting := postingOf(locked, "locked", 1)
	timedPosting := postingOf(timed, "timed", 2)
	expiredPosting := postingOf(expired, "expired", 3)
	visible := postingOf(b.recruiter, "visible", 4)
	b.seed.application(lockedPosting, b.candidate)
	lock(locked, true, nil)
	lock(timed, true, ptr(time.Now().Add(time.Hour)))
	lock(expired, true, ptr(time.Now().Add(-time.Hour)))

	items, total, err := b.repo.ListOpen(ctx, b.filter(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	sameIDs(t, items, visible, expiredPosting)
	if total != 2 {
		t.Fatalf("total = %d, want 2 (the count hides locked Recruiters too)", total)
	}
	_, notOpen := b.repo.GetOpen(ctx, b.candidate, uuid.New())
	for name, id := range map[string]uuid.UUID{"locked": lockedPosting, "timed lock": timedPosting} {
		_, err := b.repo.GetOpen(ctx, b.candidate, id)
		if codeOf(err) != apperror.CodeJobPostingNotFound || err.Error() != notOpen.Error() {
			t.Fatalf("%s posting detail: error = %v, want the same 404 as a missing posting", name, err)
		}
	}
	if _, err := b.repo.GetOpen(ctx, b.candidate, expiredPosting); err != nil {
		t.Fatalf("expired lock posting detail: %v", err)
	}

	// Lifting the lock brings the posting back; the lock never changed its status or Applications.
	lock(locked, false, nil)
	detail, err := b.repo.GetOpen(ctx, b.candidate, lockedPosting)
	if err != nil || !detail.AlreadyApplied {
		t.Fatalf("posting after unlock = %+v, %v, want it visible with the Candidate's Application kept", detail, err)
	}
	var status string
	if err := b.seed.pool.QueryRow(ctx, `SELECT status::text FROM public.job_postings WHERE id = $1`, lockedPosting).Scan(&status); err != nil || status != "open" {
		t.Fatalf("posting status = %q, %v, want open (a lock never changes it)", status, err)
	}
	items, _, err = b.repo.ListOpen(ctx, b.filter(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	sameIDs(t, items, visible, expiredPosting, lockedPosting)
}

func TestListFiltersOrderAndPagination(t *testing.T) {
	ctx, b := newBoard(t)
	hanoiFull := b.seed.posting(b.recruiter, postingSpec{title: "Go Backend", status: "open", location: ptr("Ha Noi, Vietnam"),
		employment: ptr("Full-time"), rawText: ptr("jd"), createdAt: b.base.Add(3 * time.Hour)})
	saigonPart := b.seed.posting(b.recruiter, postingSpec{title: "Frontend React", status: "open", location: ptr("Ho Chi Minh City"),
		employment: ptr("Part-time"), rawText: ptr("jd"), createdAt: b.base.Add(2 * time.Hour)})
	hanoiFull2 := b.seed.posting(b.recruiter, postingSpec{title: "Golang Platform", status: "open", location: ptr("ha noi"),
		employment: ptr("full-time"), rawText: ptr("jd"), createdAt: b.base.Add(1 * time.Hour)})
	// Two postings created at the same instant are ordered by id, descending.
	tieA := b.seed.posting(b.recruiter, postingSpec{title: "Tie", status: "open", rawText: ptr("jd"), createdAt: b.base})
	tieB := b.seed.posting(b.recruiter, postingSpec{title: "Tie", status: "open", rawText: ptr("jd"), createdAt: b.base})
	first, second := tieA, tieB
	if tieB.String() > tieA.String() {
		first, second = tieB, tieA
	}

	items, total, err := b.repo.ListOpen(ctx, b.filter(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	sameIDs(t, items, hanoiFull, saigonPart, hanoiFull2, first, second)
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if items[0].CompanyName == nil || *items[0].CompanyName != "Acme" || items[0].SeniorityLevel != "mid" {
		t.Fatalf("first item = %+v", items[0])
	}

	page, total, err := b.repo.ListOpen(ctx, b.filter(2, 2))
	if err != nil {
		t.Fatal(err)
	}
	sameIDs(t, page, hanoiFull2, first)
	if total != 5 {
		t.Fatalf("paged total = %d, want 5", total)
	}
	past, total, err := b.repo.ListOpen(ctx, b.filter(10, 50))
	if err != nil || len(past) != 0 || past == nil || total != 5 {
		t.Fatalf("past the end: items = %#v, total = %d, err = %v", past, total, err)
	}

	for name, tc := range map[string]struct {
		filter domain.BoardFilter
		want   []uuid.UUID
	}{
		"title, any case": {domain.BoardFilter{Search: ptr(b.seed.tag + " GO")}, []uuid.UUID{hanoiFull, hanoiFull2}},
		"location part":   {domain.BoardFilter{Location: ptr("HA NOI")}, []uuid.UUID{hanoiFull, hanoiFull2}},
		"employment type": {domain.BoardFilter{EmploymentType: ptr("FULL-TIME")}, []uuid.UUID{hanoiFull, hanoiFull2}},
		"type is whole":   {domain.BoardFilter{EmploymentType: ptr("full")}, nil},
		"combined":        {domain.BoardFilter{Location: ptr("minh"), EmploymentType: ptr("part-time")}, []uuid.UUID{saigonPart}},
		"no match":        {domain.BoardFilter{Location: ptr("Da Nang")}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			f := tc.filter
			if f.Search == nil {
				f.Search = &b.seed.tag
			}
			f.Limit = 100
			got, total, err := b.repo.ListOpen(ctx, f)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("items is nil; want []")
			}
			sameIDs(t, got, tc.want...)
			if total != int64(len(tc.want)) {
				t.Fatalf("total = %d, want %d", total, len(tc.want))
			}
		})
	}
}

// The service escapes LIKE wildcards; this proves Postgres then treats them as plain characters.
func TestEscapedWildcardsMatchLiterally(t *testing.T) {
	ctx, b := newBoard(t)
	literal := b.seed.posting(b.recruiter, postingSpec{title: "100%_off", status: "open", rawText: ptr("jd"), createdAt: b.base})
	b.seed.posting(b.recruiter, postingSpec{title: "100 percent off", status: "open", rawText: ptr("jd"), createdAt: b.base.Add(time.Hour)})
	for _, search := range []string{`%`, `_`, `0%_o`} {
		svc, err := service.New(b.repo, noTexts{})
		if err != nil {
			t.Fatal(err)
		}
		got, total, err := svc.List(ctx, "candidate", domain.BoardFilter{Search: ptr(search), Location: nil, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, item := range got {
			if strings.HasPrefix(item.Title, b.seed.tag) {
				found++
				if item.ID != literal {
					t.Fatalf("search %q matched %q", search, item.Title)
				}
			}
		}
		if found != 1 || total < 1 {
			t.Fatalf("search %q found %d tagged postings (total %d), want only the literal one", search, found, total)
		}
	}
}

type noTexts struct{}

func (noTexts) Download(context.Context, string) (storage.Object, error) {
	return storage.Object{}, errors.New("not used")
}

func TestDetailContentAndAlreadyApplied(t *testing.T) {
	ctx, b := newBoard(t)
	id := b.seed.posting(b.recruiter, postingSpec{title: "Detail", status: "open", location: ptr("Ha Noi"), employment: ptr("Full-time"),
		rawText: ptr("Full JD text"), createdAt: b.base, requirements: []string{"Go", "PostgreSQL"},
		slot: 5, rejectReason: ptr("old reason")})
	other := b.seed.user("candidate", nil)
	b.seed.application(id, b.candidate)

	record, err := b.repo.GetOpen(ctx, b.candidate, id)
	if err != nil {
		t.Fatal(err)
	}
	if !record.AlreadyApplied {
		t.Fatal("already_applied = false for the Candidate who applied")
	}
	if record.StoredText == nil || *record.StoredText != "Full JD text" || record.CompanyWebsite == nil ||
		record.InterviewDuration == nil || *record.InterviewDuration != 30 || len(record.Requirements) != 2 ||
		record.Requirements[0].Name != "Go" || record.Requirements[0].Type != "required" {
		t.Fatalf("record = %+v", record)
	}
	notApplied, err := b.repo.GetOpen(ctx, other, id)
	if err != nil {
		t.Fatal(err)
	}
	if notApplied.AlreadyApplied {
		t.Fatal("already_applied = true for a Candidate who did not apply")
	}
}
