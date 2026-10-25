package service

import (
	"context"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// historyRepositoryFake answers the history read and records what the service asked for. The embedded
// Repository is never called by these tests.
type historyRepositoryFake struct {
	Repository
	items        []domain.HistoryItem
	total        int64
	err          error
	calls        int
	gotCandidate uuid.UUID
	gotFilter    domain.HistoryFilter
}

func (f *historyRepositoryFake) ListHistory(_ context.Context, candidateID uuid.UUID, filter domain.HistoryFilter) ([]domain.HistoryItem, int64, error) {
	f.calls++
	f.gotCandidate = candidateID
	f.gotFilter = filter
	return f.items, f.total, f.err
}

func newHistoryTestService(t *testing.T, repo Repository) *Service {
	t.Helper()
	svc, err := New(repo, Settings{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc
}

func validHistoryFilter() domain.HistoryFilter {
	return domain.HistoryFilter{Page: 1, PageSize: domain.DefaultHistoryPageSize}
}

func TestListHistoryRequiresCandidate(t *testing.T) {
	repo := &historyRepositoryFake{}
	svc := newHistoryTestService(t, repo)

	_, err := svc.ListHistory(context.Background(), uuid.New(), "recruiter", validHistoryFilter())
	assertCode(t, err, apperror.CodeForbidden)
	if repo.calls != 0 {
		t.Fatalf("repository called %d times for a non-candidate", repo.calls)
	}
}

func TestListHistoryValidatesFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter domain.HistoryFilter
	}{
		{name: "unknown type", filter: domain.HistoryFilter{Type: "coaching", Page: 1, PageSize: 20}},
		{name: "page zero", filter: domain.HistoryFilter{Page: 0, PageSize: 20}},
		{name: "page size zero", filter: domain.HistoryFilter{Page: 1, PageSize: 0}},
		{name: "page size over the cap", filter: domain.HistoryFilter{Page: 1, PageSize: 101}},
		{name: "offset beyond 32 bits", filter: domain.HistoryFilter{Page: 1 << 30, PageSize: 100}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &historyRepositoryFake{}
			svc := newHistoryTestService(t, repo)

			_, err := svc.ListHistory(context.Background(), uuid.New(), "candidate", tt.filter)
			assertCode(t, err, apperror.CodeValidation)
			if repo.calls != 0 {
				t.Fatalf("repository called %d times for an invalid filter", repo.calls)
			}
		})
	}
}

func TestListHistoryAcceptsValidFilters(t *testing.T) {
	filters := []domain.HistoryFilter{
		{Page: 1, PageSize: 20},
		{Type: domain.TypePractice, Page: 3, PageSize: 100},
		{Type: domain.TypeRecruitment, Page: 1, PageSize: 1},
	}
	for _, filter := range filters {
		svc := newHistoryTestService(t, &historyRepositoryFake{})
		if _, err := svc.ListHistory(context.Background(), uuid.New(), "candidate", filter); err != nil {
			t.Fatalf("filter %+v: unexpected error %v", filter, err)
		}
	}
}

func TestListHistoryPassesCandidateAndFilterAndEchoesPage(t *testing.T) {
	candidateID := uuid.New()
	repo := &historyRepositoryFake{total: 42}
	svc := newHistoryTestService(t, repo)
	filter := domain.HistoryFilter{Type: domain.TypeRecruitment, Page: 2, PageSize: 10}

	page, err := svc.ListHistory(context.Background(), candidateID, "candidate", filter)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if repo.gotCandidate != candidateID {
		t.Fatalf("candidate = %s, want %s", repo.gotCandidate, candidateID)
	}
	if repo.gotFilter != filter {
		t.Fatalf("filter = %+v, want %+v", repo.gotFilter, filter)
	}
	if page.Total != 42 || page.Page != 2 || page.PageSize != 10 {
		t.Fatalf("page metadata = total %d page %d page_size %d", page.Total, page.Page, page.PageSize)
	}
}

func TestListHistoryReturnsEmptyArrayNotNull(t *testing.T) {
	svc := newHistoryTestService(t, &historyRepositoryFake{items: nil})

	page, err := svc.ListHistory(context.Background(), uuid.New(), "candidate", validHistoryFilter())
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("items = %#v, want an empty non-nil slice", page.Items)
	}
}

func TestListHistoryKeepsRepositoryOrder(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	started := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	repo := &historyRepositoryFake{
		items: []domain.HistoryItem{
			{ID: first, Type: domain.TypePractice, StartedAt: started},
			{ID: second, Type: domain.TypePractice, StartedAt: started.Add(-time.Hour)},
		},
		total: 2,
	}
	svc := newHistoryTestService(t, repo)

	page, err := svc.ListHistory(context.Background(), uuid.New(), "candidate", validHistoryFilter())
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != first || page.Items[1].ID != second {
		t.Fatalf("items reordered: %+v", page.Items)
	}
}

func TestListHistoryPassesRepositoryErrorsThrough(t *testing.T) {
	repo := &historyRepositoryFake{err: apperror.New(apperror.CodeInternal, "interview persistence failed")}
	svc := newHistoryTestService(t, repo)

	_, err := svc.ListHistory(context.Background(), uuid.New(), "candidate", validHistoryFilter())
	assertCode(t, err, apperror.CodeInternal)
}

func TestListHistoryNeedsHistoryRepository(t *testing.T) {
	// fakeRepository from the session tests does not implement HistoryRepository.
	svc := newHistoryTestService(t, &fakeRepository{})

	_, err := svc.ListHistory(context.Background(), uuid.New(), "candidate", validHistoryFilter())
	assertCode(t, err, apperror.CodeInternal)
}
