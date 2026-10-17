//go:build integration

package tests

import (
	"testing"

	authrepo "github.com/cog-forge/rolecue/api/internal/features/auth/repository"
	profiledomain "github.com/cog-forge/rolecue/api/internal/features/profile/domain"
	profilerepo "github.com/cog-forge/rolecue/api/internal/features/profile/repository"
	profileservice "github.com/cog-forge/rolecue/api/internal/features/profile/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func value(s string) profiledomain.StringChange {
	return profiledomain.StringChange{Present: true, Value: &s}
}

func TestRoleSelectionLifecycle(t *testing.T) {
	ctx, tx := openTransaction(t)
	id := seedUser(t, ctx, tx, "candidate")
	otherID := seedUser(t, ctx, tx, "candidate")
	auth := authrepo.NewRepository(tx)
	user, err := auth.Get(ctx, id)
	if err != nil || user.OnboardingRoleSelected || user.OnboardingCompleted {
		t.Fatalf("new account must be pending: user=%+v error=%v", user, err)
	}
	for i := 0; i < 2; i++ {
		user, err = auth.SelectRole(ctx, id, "recruiter")
		if err != nil || user.Role != "recruiter" || !user.OnboardingRoleSelected || user.OnboardingCompleted {
			t.Fatalf("selection/retry %d: user=%+v error=%v", i, user, err)
		}
	}
	_, err = auth.SelectRole(ctx, id, "candidate")
	wantCode(t, err, apperror.CodeOnboardingRoleConflict)
	other, err := auth.Get(ctx, otherID)
	if err != nil || other.Role != "candidate" || other.OnboardingRoleSelected {
		t.Fatalf("another account changed: user=%+v error=%v", other, err)
	}
	profile := profileservice.NewService(profilerepo.NewRepository(tx))
	patch := profiledomain.Patch{FullName: value("Alex"), CompanyName: value("Acme"), CompanyWebsite: value("https://acme.example")}
	for i := 0; i < 2; i++ {
		updated, updateErr := profile.Update(ctx, id, "recruiter", patch)
		if updateErr != nil || !updated.OnboardingCompleted || updated.FullName != "Alex" || updated.CompanyName == nil || *updated.CompanyName != "Acme" || updated.CompanyWebsite == nil || *updated.CompanyWebsite != "https://acme.example" {
			t.Fatalf("completion/retry %d: profile=%+v error=%v", i, updated, updateErr)
		}
	}
	_, err = auth.SelectRole(ctx, id, "recruiter")
	wantCode(t, err, apperror.CodeOnboardingRoleConflict)
}

func TestRoleSelectionRejectsIneligibleAccounts(t *testing.T) {
	for _, state := range []string{"admin", "locked", "unverified"} {
		t.Run(state, func(t *testing.T) {
			ctx, tx := openTransaction(t)
			role := "candidate"
			if state == "admin" {
				role = "admin"
			}
			id := seedUser(t, ctx, tx, role)
			if _, err := tx.Exec(ctx, `UPDATE public.users SET is_locked = $2, email_verified = $3 WHERE id = $1`, id, state == "locked", state != "unverified"); err != nil {
				t.Fatal(err)
			}
			auth := authrepo.NewRepository(tx)
			_, err := auth.SelectRole(ctx, id, "recruiter")
			wantCode(t, err, apperror.CodeForbidden)
			user, err := auth.Get(ctx, id)
			if err != nil || user.Role != role || (state != "admin" && user.OnboardingRoleSelected) {
				t.Fatalf("refused selection changed the user: user=%+v error=%v", user, err)
			}
		})
	}
}

func TestProfileCompletionRechecksEligibilityAtomically(t *testing.T) {
	for _, state := range []string{"role not selected", "role changed", "locked", "unverified", "missing recruiter metadata"} {
		t.Run(state, func(t *testing.T) {
			ctx, tx := openTransaction(t)
			id := seedUser(t, ctx, tx, "candidate")
			auth := authrepo.NewRepository(tx)
			role := "candidate"
			if state == "missing recruiter metadata" {
				role = "recruiter"
			}
			if state != "role not selected" {
				if _, err := auth.SelectRole(ctx, id, role); err != nil {
					t.Fatal(err)
				}
			}
			if state == "role changed" {
				if _, err := tx.Exec(ctx, `UPDATE public.users SET role = 'recruiter' WHERE id = $1`, id); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE public.users SET is_locked = $2, email_verified = $3 WHERE id = $1`, id, state == "locked", state != "unverified"); err != nil {
				t.Fatal(err)
			}
			// Call the real repository with a formerly authorized role. Its single
			// UPDATE must recheck eligibility without writing the name or completion.
			repo := profilerepo.NewRepository(tx)
			_, err := repo.Update(ctx, id, role, profiledomain.Patch{FullName: value("Should not be saved")})
			wantCode(t, err, apperror.CodeForbidden)
			current, err := repo.Get(ctx, id)
			if err != nil || current.FullName != "Original" || current.OnboardingCompleted {
				t.Fatalf("partial write on refused completion: profile=%+v error=%v", current, err)
			}
		})
	}
}

func TestRecruiterValidationDoesNotPartiallySave(t *testing.T) {
	ctx, tx := openTransaction(t)
	id := seedUser(t, ctx, tx, "candidate")
	if _, err := authrepo.NewRepository(tx).SelectRole(ctx, id, "recruiter"); err != nil {
		t.Fatal(err)
	}
	repo := profilerepo.NewRepository(tx)
	_, err := profileservice.NewService(repo).Update(ctx, id, "recruiter", profiledomain.Patch{
		FullName: value("Should not be saved"), CompanyName: value("Acme"), CompanyWebsite: profiledomain.StringChange{Present: true},
	})
	wantCode(t, err, apperror.CodeValidation)
	current, err := repo.Get(ctx, id)
	if err != nil || current.FullName != "Original" || current.CompanyName != nil || current.CompanyWebsite != nil || current.OnboardingCompleted {
		t.Fatalf("invalid metadata changed the account: profile=%+v error=%v", current, err)
	}
}
