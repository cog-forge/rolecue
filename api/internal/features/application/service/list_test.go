package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestOnlyRecruitersListApplications(t *testing.T) {
	for _, role := range []string{"candidate", "admin", "", "Recruiter"} {
		repo := &fakeRepository{}
		svc := newService(t, repo)
		_, _, err := svc.ListForPosting(context.Background(), recruiterID, role, postingID, domain.ListFilter{Limit: 20})
		assertCode(t, err, apperror.CodeForbidden)
		_, _, err = svc.ListOwn(context.Background(), recruiterID, role, domain.ListFilter{Limit: 20})
		assertCode(t, err, apperror.CodeForbidden)
		if repo.listCalls != 0 {
			t.Fatalf("%q: repository was called", role)
		}
	}
}

func TestListForPostingScopesToThePathPosting(t *testing.T) {
	other := uuid.New()
	repo := &fakeRepository{items: []domain.ListItem{{ID: applicationID}}, total: 41}
	// A posting filter smuggled into the filter must not override the path.
	items, total, err := newService(t, repo).ListForPosting(context.Background(), recruiterID, "recruiter", postingID,
		domain.ListFilter{JobPostingID: &other, Limit: 20, Offset: 40})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != applicationID || total != 41 {
		t.Fatalf("items = %+v, total = %d", items, total)
	}
	if repo.listRecruit != recruiterID {
		t.Fatalf("recruiter = %s, want %s", repo.listRecruit, recruiterID)
	}
	if repo.listFilter.JobPostingID == nil || *repo.listFilter.JobPostingID != postingID {
		t.Fatalf("posting filter = %v, want %s", repo.listFilter.JobPostingID, postingID)
	}
	if repo.listFilter.Limit != 20 || repo.listFilter.Offset != 40 {
		t.Fatalf("paging = %d/%d", repo.listFilter.Limit, repo.listFilter.Offset)
	}
}

func TestListOwnPassesTheOptionalPostingFilter(t *testing.T) {
	repo := &fakeRepository{}
	svc := newService(t, repo)
	if _, _, err := svc.ListOwn(context.Background(), recruiterID, "recruiter", domain.ListFilter{Limit: 20}); err != nil {
		t.Fatal(err)
	}
	if repo.listFilter.JobPostingID != nil {
		t.Fatalf("posting filter = %v, want none", repo.listFilter.JobPostingID)
	}
	if _, _, err := svc.ListOwn(context.Background(), recruiterID, "recruiter", domain.ListFilter{JobPostingID: &postingID, Limit: 20}); err != nil {
		t.Fatal(err)
	}
	if repo.listFilter.JobPostingID == nil || *repo.listFilter.JobPostingID != postingID {
		t.Fatalf("posting filter = %v, want %s", repo.listFilter.JobPostingID, postingID)
	}
}

func TestListReturnsEmptySliceNotNil(t *testing.T) {
	items, total, err := newService(t, &fakeRepository{}).ListOwn(context.Background(), recruiterID, "recruiter", domain.ListFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if items == nil || len(items) != 0 || total != 0 {
		t.Fatalf("items = %#v, total = %d; want an empty non-nil slice", items, total)
	}
}

func TestListRejectsBadFilters(t *testing.T) {
	long := strings.Repeat("a", maxSearchLength+1)
	for name, filter := range map[string]domain.ListFilter{
		"limit zero":      {Limit: 0},
		"limit too big":   {Limit: 101},
		"negative offset": {Limit: 20, Offset: -1},
		"unknown status":  {Limit: 20, Status: ptr(domain.ApplicationStatus("hired"))},
		"empty status":    {Limit: 20, Status: ptr(domain.ApplicationStatus(""))},
		"long search":     {Limit: 20, Search: &long},
		"invalid UTF-8":   {Limit: 20, Search: ptr("\xff")},
		"NUL in search":   {Limit: 20, Search: ptr("Ha\x00Noi")},
	} {
		repo := &fakeRepository{}
		_, _, err := newService(t, repo).ListOwn(context.Background(), recruiterID, "recruiter", filter)
		assertCode(t, err, apperror.CodeValidation)
		if repo.listCalls != 0 {
			t.Fatalf("%s: repository was called", name)
		}
	}
}

func TestListAcceptsEveryStatus(t *testing.T) {
	for _, status := range []domain.ApplicationStatus{
		domain.StatusPending, domain.StatusInterviewEligible, domain.StatusInterviewed, domain.StatusApproved, domain.StatusRejected,
	} {
		repo := &fakeRepository{}
		if _, _, err := newService(t, repo).ListOwn(context.Background(), recruiterID, "recruiter", domain.ListFilter{Limit: 20, Status: &status}); err != nil {
			t.Fatalf("%s: %v", status, err)
		}
		if repo.listFilter.Status == nil || *repo.listFilter.Status != status {
			t.Fatalf("status = %v, want %s", repo.listFilter.Status, status)
		}
	}
}

func TestListNormalisesSearch(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   *string
		want *string
	}{
		{name: "absent search stays off"},
		{name: "blank search is dropped", in: ptr("  \t")},
		{name: "value is trimmed", in: ptr("  linh@x.com "), want: ptr("linh@x.com")},
		{name: "LIKE wildcards match themselves", in: ptr(`100%_a\`), want: ptr(`100\%\_a\\`)},
		{name: "exactly the maximum length is accepted", in: ptr(strings.Repeat("é", maxSearchLength)), want: ptr(strings.Repeat("é", maxSearchLength))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{}
			if _, _, err := newService(t, repo).ListOwn(context.Background(), recruiterID, "recruiter", domain.ListFilter{Limit: 20, Search: tc.in}); err != nil {
				t.Fatal(err)
			}
			got := repo.listFilter.Search
			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Fatalf("search = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestListPassesRepositoryErrorsThrough(t *testing.T) {
	want := apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")
	_, _, err := newService(t, &fakeRepository{err: want}).ListForPosting(context.Background(), recruiterID, "recruiter", postingID, domain.ListFilter{Limit: 20})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
