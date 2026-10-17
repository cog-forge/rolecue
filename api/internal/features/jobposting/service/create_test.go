package service

import (
	"context"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestCreateStoresTextThenRows(t *testing.T) {
	repo, store := &fakeRepository{}, newStore()
	recruiter := uuid.New()
	input := validCreate()
	input.Title = "  Backend Engineer  "
	input.Location = ptr("  ")
	input.EmploymentType = ptr(" Full-time ")

	got, err := newService(t, repo, store).Create(context.Background(), recruiter, "recruiter", input)
	if err != nil {
		t.Fatal(err)
	}
	if repo.recruiter != recruiter {
		t.Fatal("the posting was not stored for the session user")
	}
	if store.objects[repo.created.ContentPath] != input.RawText {
		t.Fatal("the JD text was not stored under the posting's content_path")
	}
	if repo.created.Title != "Backend Engineer" || repo.created.Location != nil || *repo.created.EmploymentType != "Full-time" {
		t.Fatalf("fields were not normalized: %+v", repo.created.CreateInput)
	}
	if got.RawText != input.RawText || got.Status != domain.StatusPending {
		t.Fatalf("response: status=%s text matches=%t", got.Status, got.RawText == input.RawText)
	}
	// The repository returns the stored posting; a separate read after commit could fail and make
	// a client retry create a duplicate.
	if repo.getCalls != 0 {
		t.Fatal("create re-read the posting after the insert")
	}
}

func TestCreateValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*domain.CreateInput)
		code   apperror.Code
	}{
		{"blank title", func(in *domain.CreateInput) { in.Title = "  " }, apperror.CodeValidation},
		{"title too long", func(in *domain.CreateInput) { in.Title = strings.Repeat("界", 201) }, apperror.CodeValidation},
		{"short text", func(in *domain.CreateInput) { in.RawText = "Go developer" }, apperror.CodeJDTooShort},
		{"long text", func(in *domain.CreateInput) { in.RawText = strings.Repeat("a", MaxTextLength+1) }, apperror.CodeJDTooLong},
		{"invalid UTF-8", func(in *domain.CreateInput) { in.RawText = validText() + "\xff" }, apperror.CodeInvalidJDInput},
		{"unknown seniority", func(in *domain.CreateInput) { in.SeniorityLevel = "principal" }, apperror.CodeValidation},
		{"no requirements", func(in *domain.CreateInput) { in.Requirements = nil }, apperror.CodeValidation},
		{"blank requirement", func(in *domain.CreateInput) { in.Requirements[0].Name = " " }, apperror.CodeValidation},
		{"unknown target", func(in *domain.CreateInput) { in.Requirements[0].Target = "soft_skill" }, apperror.CodeValidation},
		{"unknown type", func(in *domain.CreateInput) { in.Requirements[0].Type = "optional" }, apperror.CodeValidation},
		{"duplicate requirement", func(in *domain.CreateInput) {
			in.Requirements[1] = domain.PostingRequirement{Name: " go ", Target: "programming_language", Type: domain.Preferred}
		}, apperror.CodeValidation},
		{"too many requirements", func(in *domain.CreateInput) {
			in.Requirements = nil
			for i := 0; i <= maxRequirements; i++ {
				in.Requirements = append(in.Requirements, domain.PostingRequirement{Name: uuid.NewString(), Target: "tool", Type: domain.Required})
			}
		}, apperror.CodeValidation},
		{"negative salary", func(in *domain.CreateInput) { in.SalaryMin = ptr(int32(-1)) }, apperror.CodeValidation},
		{"salary range reversed", func(in *domain.CreateInput) {
			in.SalaryMin, in.SalaryMax = ptr(int32(2000)), ptr(int32(1000))
		}, apperror.CodeValidation},
		{"zero duration", func(in *domain.CreateInput) { in.InterviewDuration = ptr(int32(0)) }, apperror.CodeValidation},
		{"long location", func(in *domain.CreateInput) { in.Location = ptr(strings.Repeat("x", 201)) }, apperror.CodeValidation},
		{"missing avatar", func(in *domain.CreateInput) { in.RequiredAvatarID = uuid.Nil }, apperror.CodeValidation},
		{"missing voice", func(in *domain.CreateInput) { in.RequiredVoiceID = uuid.Nil }, apperror.CodeValidation},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo, store := &fakeRepository{}, newStore()
			input := validCreate()
			tt.change(&input)
			_, err := newService(t, repo, store).Create(context.Background(), uuid.New(), "recruiter", input)
			assertCode(t, err, tt.code)
			if store.uploads+repo.createCalls != 0 {
				t.Fatal("invalid input reached storage or the repository")
			}
		})
	}
}

func TestCreateDoesNotWriteRowsWhenUploadFails(t *testing.T) {
	repo, store := &fakeRepository{}, newStore()
	store.uploadErr = errBoom
	_, err := newService(t, repo, store).Create(context.Background(), uuid.New(), "recruiter", validCreate())
	assertCode(t, err, apperror.CodeInternal)
	if repo.createCalls != 0 {
		t.Fatal("rows were written without the JD text")
	}
}

func TestCreatePassesRepositoryErrorsThrough(t *testing.T) {
	repo := &fakeRepository{err: domain.ErrAvatarNotOwned}
	_, err := newService(t, repo, newStore()).Create(context.Background(), uuid.New(), "recruiter", validCreate())
	assertCode(t, err, apperror.CodeValidation)
}
