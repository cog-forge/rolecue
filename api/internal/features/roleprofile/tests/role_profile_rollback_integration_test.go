//go:build integration

package tests

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// The review write is one transaction: SaveProfileJD (write 1), DeleteProfileRequirements
// (write 2), then InsertProfileRequirement (write 3). The invalid requirement kind violates
// the requirements_kind check on write 3, so the first two writes must roll back.
func TestRoleProfileReviewWriteRollsBackWhenAnyWriteFails(t *testing.T) {
	env := newRoleProfileEnv(t)
	owner := insertFixtureUser(t, env, "candidate")
	profile := createAnalyzedProfile(t, env, owner)
	before, err := env.app.Get(env.ctx, owner, "candidate", profile.ID)
	if err != nil {
		t.Fatal("get before", err)
	}

	// Bypasses the service validation on purpose: the database constraint is what must fail.
	failing := domain.RoleReview{
		Title:          "Rolled back title",
		RefinementNote: "Rolled back note",
		Requirements:   []domain.RoleRequirement{{Name: "Rust", Target: "Proficient", Kind: "bogus"}},
	}
	_, err = env.repo.SaveProfile(env.ctx, owner, profile.ID, domain.SaveRoleProfile{RoleReview: failing})
	wantCode(t, err, apperror.CodeInternal)

	after, err := env.app.Get(env.ctx, owner, "candidate", profile.ID)
	if err != nil {
		t.Fatal("get after", err)
	}
	if after.RefinementNote != before.RefinementNote || after.Title != before.Title || after.Status != domain.Review {
		t.Fatalf("partial review write survived: before=%+v after=%+v", before.RoleReview, after.RoleReview)
	}
	if len(after.Requirements) != len(before.Requirements) {
		t.Fatalf("requirements changed: before=%d after=%d", len(before.Requirements), len(after.Requirements))
	}
	for i := range before.Requirements {
		if after.Requirements[i].Name != before.Requirements[i].Name {
			t.Fatalf("requirement %d = %q, want %q", i, after.Requirements[i].Name, before.Requirements[i].Name)
		}
	}
}
