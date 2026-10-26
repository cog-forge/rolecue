//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// historyPage reads one page of the candidate's history through the service.
func historyPage(t *testing.T, ctx context.Context, svc *service.Service, candidate uuid.UUID, filter domain.HistoryFilter) domain.HistoryPage {
	t.Helper()
	page, err := svc.ListHistory(ctx, candidate, "candidate", filter)
	if err != nil {
		t.Fatalf("ListHistory(%+v) error = %v", filter, err)
	}
	return page
}

// allPages is the filter of the first page at the default size, with no type.
func allPages(page, size int32) domain.HistoryFilter {
	return domain.HistoryFilter{Page: page, PageSize: size}
}

// TestHistoryListsOnlyOwnSessions proves a candidate's history holds only their own sessions: another
// candidate's session never appears, and the total counts the candidate's sessions only.
func TestHistoryListsOnlyOwnSessions(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	alice := s.user("candidate")
	s.wallet(alice, 100)
	bob := s.user("candidate")
	s.wallet(bob, 100)

	aliceIDs := map[uuid.UUID]bool{}
	for i := 0; i < 2; i++ {
		got, _ := practiceSession(t, ctx, s, svc, alice, 4)
		aliceIDs[got.ID] = true
	}
	bobSession, _ := practiceSession(t, ctx, s, svc, bob, 4)

	page := historyPage(t, ctx, svc, alice, allPages(1, 20))
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("alice history total=%d items=%d, want 2 and 2", page.Total, len(page.Items))
	}
	for _, item := range page.Items {
		if !aliceIDs[item.ID] || item.ID == bobSession.ID {
			t.Fatalf("alice history contains %s, which is not one of her sessions", item.ID)
		}
	}

	bobPage := historyPage(t, ctx, svc, bob, allPages(1, 20))
	if bobPage.Total != 1 || len(bobPage.Items) != 1 || bobPage.Items[0].ID != bobSession.ID {
		t.Fatalf("bob history = %+v, want only his session", bobPage)
	}
}

// TestHistoryFiltersByType proves the type filter returns only the matching sessions and an absent filter
// returns both kinds.
func TestHistoryFiltersByType(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 100)
	practiceSession(t, ctx, s, svc, candidate, 4)
	practiceSession(t, ctx, s, svc, candidate, 4)
	_, posting, _ := s.openPosting(slotOf(2), 4)
	application := s.eligibleApplication(posting, candidate)
	recruitment, _, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	if err != nil {
		t.Fatalf("recruitment Create() error = %v", err)
	}

	all := historyPage(t, ctx, svc, candidate, allPages(1, 20))
	if all.Total != 3 {
		t.Fatalf("unfiltered total = %d, want 3", all.Total)
	}
	rec := historyPage(t, ctx, svc, candidate, domain.HistoryFilter{Type: domain.TypeRecruitment, Page: 1, PageSize: 20})
	if rec.Total != 1 || len(rec.Items) != 1 {
		t.Fatalf("recruitment filter = %+v, want exactly one", rec)
	}
	item := rec.Items[0]
	if item.ID != recruitment.ID || item.Type != domain.TypeRecruitment || item.ApplicationID == nil ||
		*item.ApplicationID != application || item.JobPostingTitle == nil {
		t.Fatalf("recruitment item = %+v, want the Application's session with its posting title", item)
	}
	prac := historyPage(t, ctx, svc, candidate, domain.HistoryFilter{Type: domain.TypePractice, Page: 1, PageSize: 20})
	if prac.Total != 2 {
		t.Fatalf("practice filter total = %d, want 2", prac.Total)
	}
	for _, p := range prac.Items {
		if p.Type != domain.TypePractice || p.ApplicationID != nil || p.JobPostingTitle != nil {
			t.Fatalf("practice item = %+v, want no application or posting", p)
		}
	}
}

// TestHistoryPagesInNewestFirstOrder pages five sessions two at a time. The pages together hold every
// session once, ordered by start time then id, newest first, and a page past the end is empty.
func TestHistoryPagesInNewestFirstOrder(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 100)
	created := map[uuid.UUID]bool{}
	for i := 0; i < 5; i++ {
		got, _ := practiceSession(t, ctx, s, svc, candidate, 4)
		created[got.ID] = true
	}

	var seen []domain.HistoryItem
	for _, tc := range []struct{ page, want int32 }{{1, 2}, {2, 2}, {3, 1}} {
		got := historyPage(t, ctx, svc, candidate, allPages(tc.page, 2))
		if got.Total != 5 || int32(len(got.Items)) != tc.want {
			t.Fatalf("page %d total=%d items=%d, want 5 and %d", tc.page, got.Total, len(got.Items), tc.want)
		}
		seen = append(seen, got.Items...)
	}
	if len(seen) != 5 {
		t.Fatalf("pages hold %d sessions, want 5", len(seen))
	}
	unique := map[uuid.UUID]bool{}
	for i, item := range seen {
		if !created[item.ID] || unique[item.ID] {
			t.Fatalf("item %d = %s is repeated or not a created session", i, item.ID)
		}
		unique[item.ID] = true
		if i > 0 {
			prev := seen[i-1]
			if prev.StartedAt.Before(item.StartedAt) {
				t.Fatalf("item %d starts after the item before it: order is not newest first", i)
			}
			if prev.StartedAt.Equal(item.StartedAt) && prev.ID.String() < item.ID.String() {
				t.Fatalf("item %d breaks the id tiebreak", i)
			}
		}
	}

	past := historyPage(t, ctx, svc, candidate, allPages(4, 2))
	if past.Total != 5 || past.Items == nil || len(past.Items) != 0 {
		t.Fatalf("page past the end = %+v, want an empty non-nil list with total 5", past)
	}
}

// TestHistoryItemsCarryOnlyListFields proves each history item has exactly the list fields. Questions,
// turns, feedback, the recording path and any current question are never part of an item.
func TestHistoryItemsCarryOnlyListFields(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 100)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 4)
	if _, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, "an answer"); err != nil {
		t.Fatalf("AppendTurn() error = %v", err)
	}

	page := historyPage(t, ctx, svc, candidate, allPages(1, 20))
	body, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil || len(decoded.Items) != 1 {
		t.Fatalf("decode page: %v items=%d", err, len(decoded.Items))
	}
	allowed := map[string]bool{
		"id": true, "type": true, "started_at": true, "completed_at": true, "evaluation_status": true,
		"status": true, "score": true, "role_profile_title": true, "job_posting_title": true, "application_id": true,
	}
	for key := range decoded.Items[0] {
		if !allowed[key] {
			t.Fatalf("history item has key %q, which is not a list field", key)
		}
	}
	if len(decoded.Items[0]) != len(allowed) {
		t.Fatalf("history item keys = %v, want exactly the %d list fields", decoded.Items[0], len(allowed))
	}
	if strings.Contains(string(body), "an answer") {
		t.Fatal("history JSON leaks an answer")
	}
}

// TestHistoryStatusFollowsCompletionAndEvaluation walks one session through in_progress, pending,
// completed with a score, and failed, as the evaluator and completion set them.
func TestHistoryStatusFollowsCompletionAndEvaluation(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 100)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 4)

	statusOf := func() domain.HistoryItem {
		t.Helper()
		page := historyPage(t, ctx, svc, candidate, allPages(1, 20))
		if len(page.Items) != 1 {
			t.Fatalf("history items = %d, want 1", len(page.Items))
		}
		return page.Items[0]
	}

	if item := statusOf(); item.Status != domain.StatusInProgress || item.Score != nil {
		t.Fatalf("open session = %+v, want in_progress without a score", item)
	}
	if _, err := svc.Complete(ctx, candidate, "candidate", started.ID); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if item := statusOf(); item.Status != domain.StatusPending {
		t.Fatalf("completed session status = %s, want pending before evaluation", item.Status)
	}
	s.exec(`UPDATE public.interviews SET evaluation_status = 'completed', score = 80 WHERE id = $1`, started.ID)
	if item := statusOf(); item.Status != domain.StatusCompleted || item.Score == nil || *item.Score != 80 {
		t.Fatalf("evaluated session = %+v, want completed with score 80", item)
	}
	s.exec(`UPDATE public.interviews SET evaluation_status = 'failed', score = NULL WHERE id = $1`, started.ID)
	if item := statusOf(); item.Status != domain.StatusFailed || item.Score != nil {
		t.Fatalf("failed evaluation = %+v, want failed without a score", item)
	}
}

// TestHistoryRejectsBadFiltersAndOtherRoles proves an unknown type, a bad page or page size, and a role
// other than candidate are refused.
func TestHistoryRejectsBadFiltersAndOtherRoles(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")

	_, err := svc.ListHistory(ctx, candidate, "candidate", domain.HistoryFilter{Type: "bogus", Page: 1, PageSize: 20})
	assertCode(t, err, apperror.CodeValidation)
	_, err = svc.ListHistory(ctx, candidate, "candidate", allPages(0, 20))
	assertCode(t, err, apperror.CodeValidation)
	_, err = svc.ListHistory(ctx, candidate, "candidate", allPages(1, 101))
	assertCode(t, err, apperror.CodeValidation)
	_, err = svc.ListHistory(ctx, candidate, "recruiter", allPages(1, 20))
	assertCode(t, err, apperror.CodeForbidden)
}

// TestHistoryEmptyListIsAnEmptyArray proves a candidate with no sessions gets an empty items array, not
// null.
func TestHistoryEmptyListIsAnEmptyArray(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")

	page := historyPage(t, ctx, svc, candidate, allPages(1, 20))
	if page.Total != 0 || page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("empty history = %+v, want total 0 and an empty non-nil list", page)
	}
	body, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"items":[]`) {
		t.Fatalf("empty history JSON = %s, want items []", body)
	}
}
