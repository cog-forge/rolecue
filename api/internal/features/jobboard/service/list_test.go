package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/jobboard/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestNewRequiresDependencies(t *testing.T) {
	if _, err := New(nil, &fakeTexts{}); err == nil {
		t.Fatal("New accepted a nil repository")
	}
	if _, err := New(&fakeRepository{}, nil); err == nil {
		t.Fatal("New accepted a nil text store")
	}
}

func TestListReturnsOpenPostingsForCandidates(t *testing.T) {
	item := domain.BoardPosting{ID: uuid.New(), Title: "Backend Engineer", SeniorityLevel: "mid"}
	repo := &fakeRepository{items: []domain.BoardPosting{item}, total: 41}
	items, total, err := newService(t, repo, nil).List(context.Background(), "candidate", domain.BoardFilter{Limit: 20, Offset: 40})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != item.ID || total != 41 {
		t.Fatalf("items = %+v, total = %d", items, total)
	}
	if repo.filter != (domain.BoardFilter{Limit: 20, Offset: 40}) {
		t.Fatalf("filter = %+v", repo.filter)
	}
}

func TestListReturnsEmptySliceNotNil(t *testing.T) {
	items, total, err := newService(t, &fakeRepository{}, nil).List(context.Background(), "candidate", domain.BoardFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if items == nil || len(items) != 0 || total != 0 {
		t.Fatalf("items = %#v, total = %d; want an empty non-nil slice", items, total)
	}
}

func TestListRefusesOtherRoles(t *testing.T) {
	for _, role := range []string{"recruiter", "admin", "", "Candidate"} {
		repo := &fakeRepository{}
		_, _, err := newService(t, repo, nil).List(context.Background(), role, domain.BoardFilter{Limit: 20})
		assertCode(t, err, apperror.CodeForbidden)
		if repo.calls != 0 {
			t.Fatalf("%q: repository was called", role)
		}
	}
}

func TestListRejectsBadFilters(t *testing.T) {
	long := strings.Repeat("a", maxFilterLength+1)
	for name, filter := range map[string]domain.BoardFilter{
		"limit zero":        {Limit: 0},
		"limit too big":     {Limit: 101},
		"negative offset":   {Limit: 20, Offset: -1},
		"long search":       {Limit: 20, Search: &long},
		"long location":     {Limit: 20, Location: &long},
		"long type":         {Limit: 20, EmploymentType: &long},
		"invalid UTF-8":     {Limit: 20, Search: ptr("\xff")},
		"NUL in location":   {Limit: 20, Location: ptr("Ha\x00Noi")},
		"NUL in employment": {Limit: 20, EmploymentType: ptr("\x00")},
	} {
		repo := &fakeRepository{}
		_, _, err := newService(t, repo, nil).List(context.Background(), "candidate", filter)
		assertCode(t, err, apperror.CodeValidation)
		if repo.calls != 0 {
			t.Fatalf("%s: repository was called", name)
		}
	}
}

func TestListNormalisesTextFilters(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		in                         domain.BoardFilter
		search, location, employed *string
	}{
		{name: "absent filters stay off", in: domain.BoardFilter{Limit: 20}},
		{name: "blank filters are dropped", in: domain.BoardFilter{Limit: 20, Search: ptr("  "), Location: ptr(""), EmploymentType: ptr("\t")}},
		{
			name:   "values are trimmed",
			in:     domain.BoardFilter{Limit: 20, Search: ptr("  Go dev "), Location: ptr(" Ha Noi "), EmploymentType: ptr(" Full-time ")},
			search: ptr("Go dev"), location: ptr("Ha Noi"), employed: ptr("Full-time"),
		},
		{
			name:   "LIKE wildcards match themselves",
			in:     domain.BoardFilter{Limit: 20, Search: ptr(`100%_C\`), Location: ptr("a_b"), EmploymentType: ptr("50%_x")},
			search: ptr(`100\%\_C\\`), location: ptr(`a\_b`), employed: ptr("50%_x"),
		},
		{
			name:   "exactly the maximum length is accepted",
			in:     domain.BoardFilter{Limit: 20, Search: ptr(strings.Repeat("é", maxFilterLength))},
			search: ptr(strings.Repeat("é", maxFilterLength)),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{}
			if _, _, err := newService(t, repo, nil).List(context.Background(), "candidate", tc.in); err != nil {
				t.Fatal(err)
			}
			for field, pair := range map[string][2]*string{
				"search": {repo.filter.Search, tc.search}, "location": {repo.filter.Location, tc.location},
				"employment_type": {repo.filter.EmploymentType, tc.employed},
			} {
				got, want := pair[0], pair[1]
				if (got == nil) != (want == nil) || (got != nil && *got != *want) {
					t.Fatalf("%s = %v, want %v", field, deref(got), deref(want))
				}
			}
		})
	}
}

func TestListPassesRepositoryErrorsThrough(t *testing.T) {
	want := apperror.New(apperror.CodeInternal, "job board persistence failed")
	_, _, err := newService(t, &fakeRepository{err: want}, nil).List(context.Background(), "candidate", domain.BoardFilter{Limit: 20})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func deref(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
