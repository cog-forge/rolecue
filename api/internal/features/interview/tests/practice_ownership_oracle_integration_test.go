//go:build integration

package tests

import (
	"testing"

	"github.com/google/uuid"
)

// TestPracticeForeignProfileWithAnUnfinishedSessionIsHidden covers the case the plain foreign-profile test
// misses: the owner already has an unfinished session on the profile. The resume lookup used to run before
// the ownership check, so another Candidate got a different answer than for an unknown id (the owner's
// session id, in effect) and could tell the profile exists.
func TestPracticeForeignProfileWithAnUnfinishedSessionIsHidden(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	owner := s.user("candidate")
	other := s.user("candidate")
	s.wallet(owner, 20)
	s.wallet(other, 20)
	foreign, _ := s.confirmedProfile(owner, 4)
	ownersSession, _, err := svc.Create(ctx, owner, "candidate", practiceInput(foreign))
	if err != nil {
		t.Fatalf("owner Create() error = %v", err)
	}

	got, created, foreignErr := svc.Create(ctx, other, "candidate", practiceInput(foreign))
	_, _, missingErr := svc.Create(ctx, other, "candidate", practiceInput(uuid.New()))

	if foreignErr == nil || created || got.ID == ownersSession.ID {
		t.Fatalf("foreign start = session %s created %v error %v, want an error and no session", got.ID, created, foreignErr)
	}
	if codeOf(foreignErr) == "" || codeOf(foreignErr) != codeOf(missingErr) {
		t.Fatalf("foreign profile code %q differs from missing profile code %q", codeOf(foreignErr), codeOf(missingErr))
	}
	if b := s.balance(other); b != 20 {
		t.Fatalf("balance of the caller = %d, want 20 unchanged", b)
	}
}
