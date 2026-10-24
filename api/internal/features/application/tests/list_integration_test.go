//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// namedCandidate adds a Candidate whose name and email the search tests can aim at.
func (s *seed) namedCandidate(name, email string) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, $2, $3, 'candidate', true)`, id, name, email)
	s.later(`DELETE FROM public.users WHERE id = $1`, id)
	return id
}

// submittedAt moves an Application's creation time so the ordering does not depend on insert speed.
func (s *seed) submittedAt(applicationID uuid.UUID, at time.Time) {
	s.exec(`UPDATE public.applications SET created_at = $2 WHERE id = $1`, applicationID, at)
}

func ids(items []domain.ListItem) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func sameIDs(got []uuid.UUID, want ...uuid.UUID) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestListApplicationsIsOwnerScoped(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, other := s.user("recruiter"), s.user("recruiter")
	ownPosting, otherPosting := s.posting(owner, "open", slot(3)), s.posting(other, "open", slot(3))
	candidate := s.user("candidate")
	mine, theirs := s.application(ownPosting, candidate), s.application(otherPosting, candidate)

	items, total, err := svc.ListForPosting(ctx, owner, "recruiter", ownPosting, domain.ListFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || !sameIDs(ids(items), mine) {
		t.Fatalf("own posting: total = %d, items = %v, want only %s", total, ids(items), mine)
	}

	// Another Recruiter's posting is not found, with Applications or without.
	_, _, err = svc.ListForPosting(ctx, owner, "recruiter", otherPosting, domain.ListFilter{Limit: 20})
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	_, _, err = svc.ListForPosting(ctx, owner, "recruiter", uuid.New(), domain.ListFilter{Limit: 20})
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	_, _, err = svc.ListOwn(ctx, owner, "recruiter", domain.ListFilter{Limit: 20, JobPostingID: &otherPosting})
	assertCode(t, err, apperror.CodeJobPostingNotFound)

	// The cross-posting list never shows the other Recruiter's Application.
	items, total, err = svc.ListOwn(ctx, owner, "recruiter", domain.ListFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ID == theirs {
			t.Fatalf("the cross-posting list leaked %s", theirs)
		}
	}
	if total != int64(len(items)) {
		t.Fatalf("total = %d, items = %d", total, len(items))
	}
}

func TestListApplicationsOfAPostingWithoutAnyIsEmpty(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner := s.user("recruiter")
	posting := s.posting(owner, "pending", nil)

	items, total, err := newService(t, pool).ListForPosting(ctx, owner, "recruiter", posting, domain.ListFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if items == nil || len(items) != 0 || total != 0 {
		t.Fatalf("items = %#v, total = %d; want an empty non-nil slice", items, total)
	}
}

func TestListApplicationsOrdersNewestFirstAndPages(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	posting := s.posting(owner, "open", slot(5))
	now := time.Now()
	oldest := s.application(posting, s.user("candidate"))
	middle := s.application(posting, s.user("candidate"))
	newest := s.application(posting, s.user("candidate"))
	s.submittedAt(oldest, now.Add(-3*time.Hour))
	s.submittedAt(middle, now.Add(-2*time.Hour))
	s.submittedAt(newest, now.Add(-1*time.Hour))

	for _, tc := range []struct {
		limit, offset int32
		want          []uuid.UUID
	}{
		{20, 0, []uuid.UUID{newest, middle, oldest}},
		{2, 0, []uuid.UUID{newest, middle}},
		{2, 2, []uuid.UUID{oldest}},
		{2, 3, nil},
	} {
		items, total, err := svc.ListForPosting(ctx, owner, "recruiter", posting, domain.ListFilter{Limit: tc.limit, Offset: tc.offset})
		if err != nil {
			t.Fatal(err)
		}
		if total != 3 || !sameIDs(ids(items), tc.want...) {
			t.Fatalf("limit %d offset %d: total = %d, items = %v, want %v", tc.limit, tc.offset, total, ids(items), tc.want)
		}
	}
}

func TestListApplicationsFiltersByStatus(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	posting := s.posting(owner, "intake_closed", slot(5))
	pending := s.application(posting, s.user("candidate"))
	rejected := s.application(posting, s.user("candidate"))
	s.exec(`UPDATE public.applications SET status = 'rejected', reject_reason = 'intake closed' WHERE id = $1`, rejected)

	for status, want := range map[domain.ApplicationStatus]uuid.UUID{
		domain.StatusPending: pending, domain.StatusRejected: rejected,
	} {
		status := status
		items, total, err := svc.ListForPosting(ctx, owner, "recruiter", posting, domain.ListFilter{Limit: 20, Status: &status})
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 || !sameIDs(ids(items), want) {
			t.Fatalf("%s: total = %d, items = %v, want %s", status, total, ids(items), want)
		}
	}
	// Auto-rejected Applications still show, as rejected, after intake closes.
	items, _, err := svc.ListForPosting(ctx, owner, "recruiter", posting, domain.ListFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	for _, item := range items {
		if item.ID == rejected && (item.Status != domain.StatusRejected || item.RejectReason == nil) {
			t.Fatalf("rejected item = %+v", item)
		}
	}
	none := domain.ApplicationStatus("approved")
	_, total, err := svc.ListForPosting(ctx, owner, "recruiter", posting, domain.ListFilter{Limit: 20, Status: &none})
	if err != nil || total != 0 {
		t.Fatalf("approved: total = %d, err = %v", total, err)
	}
}

func TestListApplicationsSearchesNameAndEmail(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	posting := s.posting(owner, "open", slot(5))
	tag := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	byName := s.application(posting, s.namedCandidate("Linh Tran "+tag, fmt.Sprintf("a-%s@example.test", tag)))
	byEmail := s.application(posting, s.namedCandidate("Someone Else", fmt.Sprintf("find.%s@example.test", tag)))
	s.application(posting, s.namedCandidate("Percent 100%", fmt.Sprintf("p-%s@example.test", tag)))
	s.application(posting, s.namedCandidate("Unrelated", fmt.Sprintf("u-%s@example.test", tag)))
	s.submittedAt(byName, time.Now().Add(-2*time.Hour))
	s.submittedAt(byEmail, time.Now().Add(-1*time.Hour))

	search := func(text string) ([]domain.ListItem, int64) {
		t.Helper()
		items, total, err := svc.ListForPosting(ctx, owner, "recruiter", posting, domain.ListFilter{Limit: 20, Search: &text})
		if err != nil {
			t.Fatal(err)
		}
		return items, total
	}
	if items, total := search("linh tran " + strings.ToUpper(tag)); total != 1 || !sameIDs(ids(items), byName) {
		t.Fatalf("name search (case-insensitive): total = %d, items = %v", total, ids(items))
	}
	if items, total := search("FIND." + tag); total != 1 || !sameIDs(ids(items), byEmail) {
		t.Fatalf("email search (case-insensitive): total = %d, items = %v", total, ids(items))
	}
	if _, total := search("@example.test"); total != 4 {
		t.Fatalf("shared email domain: total = %d, want 4", total)
	}
	// A typed % matches itself, not everything.
	if items, total := search("100%"); total != 1 || items[0].Candidate.Name != "Percent 100%" {
		t.Fatalf("literal %%: total = %d, items = %+v", total, items)
	}
	if _, total := search("%"); total != 1 {
		t.Fatalf("lone %%: total = %d, want 1", total)
	}
	if _, total := search("no-such-candidate-" + tag); total != 0 {
		t.Fatalf("no match: total = %d", total)
	}
}

func TestListApplicationsReportsInterviewEligibility(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	posting := s.posting(owner, "open", slot(5))
	live := s.application(posting, s.user("candidate"))
	expired := s.application(posting, s.user("candidate"))
	waiting := s.application(posting, s.user("candidate"))
	s.exec(`UPDATE public.applications SET status = 'interview_eligible', interview_deadline = now() + interval '1 hour' WHERE id = $1`, live)
	s.exec(`UPDATE public.applications SET status = 'interview_eligible', interview_deadline = now() - interval '1 second' WHERE id = $1`, expired)

	items, _, err := svc.ListForPosting(ctx, owner, "recruiter", posting, domain.ListFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[uuid.UUID]domain.ListItem{}
	for _, item := range items {
		byID[item.ID] = item
	}
	if got := byID[live]; !got.InterviewEligible || got.InterviewDeadline == nil {
		t.Fatalf("live = %+v, want eligible with a deadline", got)
	}
	if got := byID[expired]; got.InterviewEligible || got.InterviewDeadline == nil {
		t.Fatalf("expired = %+v, want not eligible but the deadline kept", got)
	}
	if got := byID[waiting]; got.InterviewEligible || got.InterviewDeadline != nil || got.Status != domain.StatusPending {
		t.Fatalf("waiting = %+v, want pending with no deadline", got)
	}
}

func TestListApplicationsNeverExposesTheCVOrEvidence(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner := s.user("recruiter")
	posting := s.posting(owner, "open", slot(1))
	s.application(posting, s.user("candidate"))

	items, _, err := newService(t, pool).ListForPosting(context.Background(), owner, "recruiter", posting, domain.ListFilter{Limit: 20})
	if err != nil || len(items) != 1 {
		t.Fatalf("items = %d, err = %v", len(items), err)
	}
	body, err := json.Marshal(items[0])
	if err != nil {
		t.Fatal(err)
	}
	// The exact key set, so a new field of any name fails here and has to be reviewed.
	var item map[string]json.RawMessage
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatal(err)
	}
	var candidate map[string]json.RawMessage
	if err := json.Unmarshal(item["candidate"], &candidate); err != nil {
		t.Fatal(err)
	}
	keys := func(m map[string]json.RawMessage) []string {
		out := make([]string, 0, len(m))
		for key := range m {
			out = append(out, key)
		}
		sort.Strings(out)
		return out
	}
	wantItem := []string{
		"candidate", "id", "interview_deadline", "interview_eligible", "job_posting_id",
		"job_posting_title", "reject_reason", "status", "submitted_at", "updated_at",
	}
	if got := keys(item); !slices.Equal(got, wantItem) {
		t.Fatalf("item keys = %v, want %v", got, wantItem)
	}
	if got, want := keys(candidate), []string{"email", "id", "image", "name"}; !slices.Equal(got, want) {
		t.Fatalf("candidate keys = %v, want %v", got, want)
	}
	if strings.Contains(string(body), "cvs/test.pdf") {
		t.Fatalf("list item JSON contains the CV path: %s", body)
	}
}
