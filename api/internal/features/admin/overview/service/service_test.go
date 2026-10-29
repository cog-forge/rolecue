package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/overview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

var fixedNow = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)

type fakeRepository struct {
	accounts     domain.OverviewAccounts
	postings     domain.OverviewPostings
	applications domain.OverviewApplications
	interviews   domain.OverviewInterviews

	accountsErr     error
	postingsErr     error
	applicationsErr error
	interviewsErr   error

	calls int
	from  time.Time
	to    time.Time
}

func (f *fakeRepository) Accounts(_ context.Context, from, to time.Time) (domain.OverviewAccounts, error) {
	f.calls++
	f.from, f.to = from, to
	return f.accounts, f.accountsErr
}

func (f *fakeRepository) Postings(_ context.Context) (domain.OverviewPostings, error) {
	f.calls++
	return f.postings, f.postingsErr
}

func (f *fakeRepository) Applications(_ context.Context, from, to time.Time) (domain.OverviewApplications, error) {
	f.calls++
	f.from, f.to = from, to
	return f.applications, f.applicationsErr
}

func (f *fakeRepository) Interviews(_ context.Context, from, to time.Time) (domain.OverviewInterviews, error) {
	f.calls++
	f.from, f.to = from, to
	return f.interviews, f.interviewsErr
}

type fakeFinance struct {
	summary domain.OverviewFinance
	err     error
	calls   int
}

func (f *fakeFinance) Summary(_ context.Context, _, _ time.Time) (domain.OverviewFinance, error) {
	f.calls++
	return f.summary, f.err
}

func newTestService(t *testing.T, repo Repository, finance Finance) *Service {
	t.Helper()
	svc, err := New(repo, finance)
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return fixedNow }
	return svc
}

func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("want %s, got %v", want, err)
	}
	if appErr.Code != want {
		t.Fatalf("want %s, got %s", want, appErr.Code)
	}
}

func TestNewRejectsNilDependencies(t *testing.T) {
	if _, err := New(nil, &fakeFinance{}); err == nil {
		t.Fatal("nil repository accepted")
	}
	if _, err := New(&fakeRepository{}, nil); err == nil {
		t.Fatal("nil finance accepted")
	}
}

func TestGetRefusesNonAdminsBeforeQuerying(t *testing.T) {
	for _, role := range []string{"candidate", "recruiter", ""} {
		repo := &fakeRepository{}
		finance := &fakeFinance{}
		svc := newTestService(t, repo, finance)
		_, err := svc.Get(context.Background(), role, 30)
		assertCode(t, err, apperror.CodeForbidden)
		if repo.calls != 0 || finance.calls != 0 {
			t.Fatalf("role %q reached a dependency", role)
		}
	}
}

func TestGetWindowAndDays(t *testing.T) {
	tests := []struct {
		name     string
		days     int
		wantDays int
		wantErr  bool
	}{
		{name: "default", days: 0, wantDays: 30},
		{name: "seven", days: 7, wantDays: 7},
		{name: "thirty", days: 30, wantDays: 30},
		{name: "ninety", days: 90, wantDays: 90},
		{name: "five", days: 5, wantErr: true},
		{name: "thirty one", days: 31, wantErr: true},
		{name: "negative", days: -1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}
			finance := &fakeFinance{}
			svc := newTestService(t, repo, finance)
			got, err := svc.Get(context.Background(), "admin", tt.days)
			if tt.wantErr {
				assertCode(t, err, apperror.CodeValidation)
				if repo.calls != 0 || finance.calls != 0 {
					t.Fatal("invalid days reached a dependency")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Window.Days != tt.wantDays {
				t.Fatalf("days = %d, want %d", got.Window.Days, tt.wantDays)
			}
			wantFrom := fixedNow.Add(-time.Duration(tt.wantDays) * 24 * time.Hour)
			if !got.Window.From.Equal(wantFrom) || !got.Window.To.Equal(fixedNow) {
				t.Fatalf("window = [%s, %s), want [%s, %s)", got.Window.From, got.Window.To, wantFrom, fixedNow)
			}
			if !repo.from.Equal(wantFrom) || !repo.to.Equal(fixedNow) {
				t.Fatalf("repository window = [%s, %s), want [%s, %s)", repo.from, repo.to, wantFrom, fixedNow)
			}
			if !got.GeneratedAt.Equal(fixedNow) {
				t.Fatalf("generated_at = %s, want %s", got.GeneratedAt, fixedNow)
			}
		})
	}
}

func TestGetPassesThroughRepositoryAndFinance(t *testing.T) {
	repo := &fakeRepository{
		accounts:     domain.OverviewAccounts{Candidates: 5, Recruiters: 2, Locked: 1, NewInWindow: 3},
		postings:     domain.OverviewPostings{Pending: 1, Open: 2, IntakeClosed: 3, Closed: 4, Rejected: 5},
		applications: domain.OverviewApplications{Pending: 6, SubmittedInWindow: 7},
		interviews:   domain.OverviewInterviews{PracticeStartedInWindow: 8, EvaluationFailed: 9},
	}
	finance := &fakeFinance{summary: domain.OverviewFinance{RevenueVND: 100000, SuccessRate: 0.5}}
	svc := newTestService(t, repo, finance)
	got, err := svc.Get(context.Background(), "admin", 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts != repo.accounts || got.JobPostings != repo.postings ||
		got.Applications != repo.applications || got.Interviews != repo.interviews ||
		got.Finance != finance.summary {
		t.Fatalf("aggregates not passed through: %+v", got)
	}
}

func TestGetZeroDataKeepsEveryKey(t *testing.T) {
	svc := newTestService(t, &fakeRepository{}, &fakeFinance{})
	got, err := svc.Get(context.Background(), "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts != (domain.OverviewAccounts{}) || got.Finance != (domain.OverviewFinance{}) ||
		got.JobPostings != (domain.OverviewPostings{}) || got.Applications != (domain.OverviewApplications{}) ||
		got.Interviews != (domain.OverviewInterviews{}) {
		t.Fatalf("zero data not preserved: %+v", got)
	}
	if !got.GeneratedAt.Equal(fixedNow) {
		t.Fatalf("generated_at = %s, want %s", got.GeneratedAt, fixedNow)
	}
}

func TestGetPropagatesErrorsWithoutPartialData(t *testing.T) {
	repoErr := errors.New("repository down")
	financeErr := errors.New("finance down")
	tests := []struct {
		name    string
		repo    *fakeRepository
		finance *fakeFinance
		want    error
	}{
		{name: "accounts", repo: &fakeRepository{accountsErr: repoErr}, finance: &fakeFinance{}, want: repoErr},
		{name: "postings", repo: &fakeRepository{postingsErr: repoErr}, finance: &fakeFinance{}, want: repoErr},
		{name: "applications", repo: &fakeRepository{applicationsErr: repoErr}, finance: &fakeFinance{}, want: repoErr},
		{name: "interviews", repo: &fakeRepository{interviewsErr: repoErr}, finance: &fakeFinance{}, want: repoErr},
		{name: "finance", repo: &fakeRepository{}, finance: &fakeFinance{err: financeErr}, want: financeErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t, tt.repo, tt.finance)
			got, err := svc.Get(context.Background(), "admin", 30)
			if !errors.Is(err, tt.want) {
				t.Fatalf("want %v, got %v", tt.want, err)
			}
			if got.GeneratedAt != (time.Time{}) || got.Window != (domain.OverviewWindow{}) {
				t.Fatalf("partial data returned on error: %+v", got)
			}
		})
	}
}
