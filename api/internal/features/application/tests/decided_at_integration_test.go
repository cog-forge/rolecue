//go:build integration

package tests

import (
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/google/uuid"
)

func decidedAt(t *testing.T, s *seed, id uuid.UUID) *time.Time {
	t.Helper()
	var at *time.Time
	if err := s.pool.QueryRow(s.ctx, `SELECT decided_at FROM public.applications WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

// KAN-305: decided_at is set once, on the terminal transition, and only then.
func TestDecidedAtIsSetOnceOnTheTerminalTransition(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	appID := s.application(postingID, s.user("candidate"))
	setStatusAndDeadline(s, appID, "interviewed", lcTime(lcAt(-time.Hour)))
	if decidedAt(t, s, appID) != nil {
		t.Fatal("decided_at set before any decision")
	}

	before := time.Now().Add(-time.Minute)
	if _, err := svc.Decide(ctx, owner, "recruiter", postingID, appID, domain.DecisionApprove, nil); err != nil {
		t.Fatal(err)
	}
	first := decidedAt(t, s, appID)
	if first == nil || first.Before(before) || first.After(time.Now().Add(time.Minute)) {
		t.Fatalf("decided_at = %v after the decision", first)
	}

	// A decided Application refuses a second decision, so the timestamp is not moved.
	_, _ = svc.Decide(ctx, owner, "recruiter", postingID, appID, domain.DecisionApprove, nil)
	if again := decidedAt(t, s, appID); again == nil || !again.Equal(*first) {
		t.Fatalf("decided_at moved from %v to %v", first, again)
	}
}

func TestDecidedAtStaysEmptyForNonTerminalApplications(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	postingID := s.posting(s.user("recruiter"), "open", slot(3))
	for _, status := range []string{"pending", "interview_eligible", "interviewed"} {
		appID := s.application(postingID, s.user("candidate"))
		s.setApplicationStatus(appID, status)
		if decidedAt(t, s, appID) != nil {
			t.Fatalf("decided_at set for status %s", status)
		}
	}
}
