package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/google/uuid"
)

// callerRecorder records the user each repository call receives, so the test can
// check that ownership is always keyed by the authenticated caller.
type callerRecorder struct {
	*profileFake
	callers map[string]uuid.UUID
}

func (r *callerRecorder) GetProfile(ctx context.Context, user, id uuid.UUID) (domain.RoleProfile, error) {
	r.callers["get"] = user
	return r.profileFake.GetProfile(ctx, user, id)
}
func (r *callerRecorder) SaveProfile(ctx context.Context, user, id uuid.UUID, in domain.SaveRoleProfile) (domain.RoleProfile, error) {
	r.callers["save"] = user
	return r.profileFake.SaveProfile(ctx, user, id, in)
}
func (r *callerRecorder) ConfirmProfile(ctx context.Context, user, id uuid.UUID) (domain.RoleProfile, error) {
	r.callers["confirm"] = user
	return r.profileFake.ConfirmProfile(ctx, user, id)
}
func (r *callerRecorder) ClaimAnalysis(ctx context.Context, user, id uuid.UUID) (domain.RoleProfile, error) {
	r.callers["analyze"] = user
	return r.profileFake.ClaimAnalysis(ctx, user, id)
}
func (r *callerRecorder) DeleteProfile(_ context.Context, user, _ uuid.UUID) error {
	r.callers["delete"] = user
	return nil
}

func TestCandidateCallerIsForwardedToEveryOwnedOperation(t *testing.T) {
	app, repo, _ := fixture(t)
	rec := &callerRecorder{profileFake: repo, callers: map[string]uuid.UUID{}}
	app.repository = rec
	ctx := context.Background()
	owner, id := uuid.New(), repo.profile.ID

	if _, err := app.Get(ctx, owner, "candidate", id); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Save(ctx, owner, "candidate", id, domain.SaveRoleProfile{RoleReview: repo.profile.RoleReview}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Confirm(ctx, owner, "candidate", id); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Analyze(ctx, owner, "candidate", id); err != nil {
		t.Fatal(err)
	}
	if err := app.Delete(ctx, owner, "candidate", id); err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"get", "save", "confirm", "analyze", "delete"} {
		if got, ok := rec.callers[op]; !ok || got != owner {
			t.Fatalf("%s received user %v, want caller %v", op, got, owner)
		}
	}
}
