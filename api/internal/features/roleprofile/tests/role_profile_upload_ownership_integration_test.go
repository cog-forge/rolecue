//go:build integration

package tests

import (
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// A PDF upload key can complete one Role Profile, and only for the Candidate it was issued to.
func TestRoleProfileUploadKeyIsOwnedAndSingleUse(t *testing.T) {
	env := newRoleProfileEnv(t)
	owner := insertFixtureUser(t, env, "candidate")
	other := insertFixtureUser(t, env, "candidate")
	id := uuid.New()
	if err := env.repo.RecordUpload(env.ctx, owner, id); err != nil {
		t.Fatal(err)
	}
	raw := "Senior backend engineer using Go, concurrency, caching and PostgreSQL."

	if _, err := env.repo.CreatePDFProfile(env.ctx, other, raw, id.String()); !isCode(err, apperror.CodeInvalidPDF) {
		t.Fatalf("foreign key: %v, want INVALID_PDF", err)
	}
	var profile domain.RoleProfile
	profile, err := env.repo.CreatePDFProfile(env.ctx, owner, raw, id.String())
	if err != nil {
		t.Fatalf("owner: %v", err)
	}
	t.Cleanup(func() { _ = env.repo.DeleteProfile(env.ctx, owner, profile.ID) })
	// The owner's retry of a lost response still gets the same profile; nobody else can reuse the key.
	if retry, err := env.repo.CreatePDFProfile(env.ctx, owner, raw, id.String()); err != nil || retry.ID != profile.ID {
		t.Fatalf("owner retry: %v", err)
	}
	if _, err := env.repo.CreatePDFProfile(env.ctx, other, raw, id.String()); !isCode(err, apperror.CodeInvalidPDF) {
		t.Fatalf("replay by another user: %v, want INVALID_PDF", err)
	}
	// Deleting the profile does not make the key usable again.
	if err := env.repo.DeleteProfile(env.ctx, owner, profile.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.repo.CreatePDFProfile(env.ctx, owner, raw, id.String()); !isCode(err, apperror.CodeInvalidPDF) {
		t.Fatalf("replay after delete: %v, want INVALID_PDF", err)
	}
	if _, err := env.repo.CreatePDFProfile(env.ctx, owner, raw, uuid.NewString()); !isCode(err, apperror.CodeInvalidPDF) {
		t.Fatalf("ungranted key: %v, want INVALID_PDF", err)
	}
}

func isCode(err error, code apperror.Code) bool {
	var appErr *apperror.AppError
	return errors.As(err, &appErr) && appErr.Code == code
}
