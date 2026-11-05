//go:build integration

package tests

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func (f fixture) newPosting(path string) domain.NewPosting {
	return domain.NewPosting{ContentPath: path, CreateInput: domain.CreateInput{
		Title: "Owned upload", SeniorityLevel: domain.Mid, RequiredAvatarID: f.avatar, RequiredVoiceID: f.voice,
		Requirements: []domain.PostingRequirement{{Name: "Go", Target: "tool", Type: domain.Required}},
	}}
}

// A JD PDF key can be attached once, and only by the Recruiter it was issued to.
func TestCreateConsumesUploadForItsOwnerOnce(t *testing.T) {
	ctx, f := newFixture(t)
	other := f.seed.user("recruiter")
	key := f.grant(t, ctx, f.recruiter)

	// The other Recruiter uses their own avatar so only the upload key can be the reason for the refusal.
	foreign := f.newPosting(key)
	foreign.RequiredAvatarID = f.seed.avatar(other)
	if _, err := f.repo.Create(ctx, other, foreign); codeOf(err) != apperror.CodeInvalidPDF {
		t.Fatalf("foreign key: %v, want INVALID_PDF", err)
	}
	record, err := f.repo.Create(ctx, f.recruiter, f.newPosting(key))
	if err != nil {
		t.Fatalf("owner: %v", err)
	}
	f.seed.track(record.ID)
	if _, err := f.repo.Create(ctx, f.recruiter, f.newPosting(key)); codeOf(err) != apperror.CodeInvalidPDF {
		t.Fatalf("replay: %v, want INVALID_PDF", err)
	}
	// A key with no grant row (uploaded before the migration) is refused too.
	if _, err := f.repo.Create(ctx, f.recruiter, f.newPosting(uuid.NewString())); codeOf(err) != apperror.CodeInvalidPDF {
		t.Fatalf("ungranted key: %v, want INVALID_PDF", err)
	}
}

// A rejected posting can swap its PDF, with the same ownership and single-use rules.
func TestUpdateConsumesUploadForItsOwnerOnce(t *testing.T) {
	ctx, f := newFixture(t)
	other := f.seed.user("recruiter")
	id := f.create(t, ctx, "Swap PDF")
	f.seed.exec(`UPDATE public.job_postings SET status = 'rejected', reject_reason = 'fix the PDF' WHERE id = $1`, id)
	key := f.grant(t, ctx, f.recruiter)
	document := &domain.NewDocument{ContentPath: key, RawText: "New JD text"}

	if err := f.repo.Update(ctx, other, id, domain.Edit{}, document); codeOf(err) != apperror.CodeJobPostingNotFound {
		t.Fatalf("other recruiter's posting: %v", err)
	}
	foreign := f.grant(t, ctx, other)
	if err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{}, &domain.NewDocument{ContentPath: foreign, RawText: "x"}); codeOf(err) != apperror.CodeInvalidPDF {
		t.Fatalf("foreign key: %v, want INVALID_PDF", err)
	}
	if err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{}, document); err != nil {
		t.Fatalf("owner: %v", err)
	}
	if err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{}, document); codeOf(err) != apperror.CodeInvalidPDF {
		t.Fatalf("replay: %v, want INVALID_PDF", err)
	}
}
