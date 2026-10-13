package service

import (
	"context"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// storedPosting puts a posting and its JD text in the fakes.
func storedPosting(t *testing.T, text string) (*fakeRepository, *fakeStore) {
	t.Helper()
	store := newStore()
	store.objects["key"] = text
	repo := &fakeRepository{record: domain.Record{ContentPath: "key", JobPosting: domain.JobPosting{ID: uuid.New(), Status: domain.StatusPending}}}
	return repo, store
}

func TestGetReadsTextBack(t *testing.T) {
	repo, store := storedPosting(t, validText())
	recruiter := uuid.New()
	got, err := newService(t, repo, store).Get(context.Background(), recruiter, "recruiter", repo.record.ID)
	if err != nil || got.RawText != validText() {
		t.Fatalf("text matches=%t err=%v", got.RawText == validText(), err)
	}
	if repo.recruiter != recruiter {
		t.Fatal("the read was not scoped to the session user")
	}
}

func TestGetRejectsOversizedStoredText(t *testing.T) {
	repo, store := storedPosting(t, strings.Repeat("a", maxStoredTextBytes+1))
	_, err := newService(t, repo, store).Get(context.Background(), uuid.New(), "recruiter", repo.record.ID)
	assertCode(t, err, apperror.CodeInternal)
}

func TestGetPassesNotFoundThrough(t *testing.T) {
	repo := &fakeRepository{err: apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")}
	_, err := newService(t, repo, newStore()).Get(context.Background(), uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeJobPostingNotFound)
}

func TestUpdateNormalizesPatch(t *testing.T) {
	repo, store := storedPosting(t, validText())
	patch := domain.Edit{
		Title:    domain.StringChange{Present: true, Value: ptr("  Senior Backend Engineer ")},
		Location: domain.StringChange{Present: true, Value: ptr("   ")},
		Requirements: domain.RequirementsChange{Present: true, Value: &[]domain.PostingRequirement{
			{Name: " Kafka ", Target: "technology", Type: domain.Required},
		}},
	}
	_, err := newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", repo.record.ID, patch)
	if err != nil {
		t.Fatal(err)
	}
	if *repo.patch.Title.Value != "Senior Backend Engineer" {
		t.Fatalf("title = %q", *repo.patch.Title.Value)
	}
	if !repo.patch.Location.Present || repo.patch.Location.Value != nil {
		t.Fatal("a blank location should clear the field")
	}
	if (*repo.patch.Requirements.Value)[0].Name != "Kafka" {
		t.Fatal("requirements were not trimmed")
	}
}

func TestUpdateValidation(t *testing.T) {
	id := uuid.New()
	for _, tt := range []struct {
		name  string
		patch domain.Edit
	}{
		{"null title", domain.Edit{Title: domain.StringChange{Present: true}}},
		{"blank title", domain.Edit{Title: domain.StringChange{Present: true, Value: ptr(" ")}}},
		{"null seniority", domain.Edit{SeniorityLevel: domain.StringChange{Present: true}}},
		{"unknown seniority", domain.Edit{SeniorityLevel: domain.StringChange{Present: true, Value: ptr("staff")}}},
		{"empty requirements", domain.Edit{Requirements: domain.RequirementsChange{Present: true, Value: &[]domain.PostingRequirement{}}}},
		{"null requirements", domain.Edit{Requirements: domain.RequirementsChange{Present: true}}},
		{"negative salary", domain.Edit{SalaryMax: domain.Int32Change{Present: true, Value: ptr(int32(-5))}}},
		{"zero duration", domain.Edit{InterviewDuration: domain.Int32Change{Present: true, Value: ptr(int32(0))}}},
		{"cleared avatar", domain.Edit{RequiredAvatarID: domain.UUIDChange{Present: true}}},
		{"cleared voice", domain.Edit{RequiredVoiceID: domain.UUIDChange{Present: true}}},
		{"nil voice", domain.Edit{RequiredVoiceID: domain.UUIDChange{Present: true, Value: &uuid.Nil}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo, store := storedPosting(t, validText())
			_, err := newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", id, tt.patch)
			assertCode(t, err, apperror.CodeValidation)
			if repo.updateCalls != 0 {
				t.Fatal("an invalid patch reached the repository")
			}
		})
	}
}

// The repository enforces pending-only edits and the requirements lock under a row lock; the
// service must report them unchanged and must not read the posting again after the failed write.
func TestUpdatePassesStateErrorsThrough(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		code apperror.Code
	}{
		{"not pending", domain.ErrNotPending, apperror.CodeJobPostingNotPending},
		{"requirements locked", domain.ErrRequirementsLocked, apperror.CodeJobPostingRequirementsLocked},
		{"salary after merge", domain.ErrSalaryRange, apperror.CodeValidation},
		{"voice inactive", domain.ErrVoiceNotActive, apperror.CodeValidation},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo, store := storedPosting(t, validText())
			repo.updateErr = tt.err
			_, err := newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", uuid.New(),
				domain.Edit{SeniorityLevel: domain.StringChange{Present: true, Value: ptr("senior")}})
			assertCode(t, err, tt.code)
			if repo.getCalls != 1 {
				t.Fatalf("get calls = %d, want only the read before the write", repo.getCalls)
			}
		})
	}
}

func TestListFilter(t *testing.T) {
	bad := domain.Status("archived")
	_, _, err := newService(t, &fakeRepository{}, newStore()).List(context.Background(), uuid.New(), "recruiter", domain.ListFilter{Status: &bad, Limit: 20})
	assertCode(t, err, apperror.CodeValidation)

	repo := &fakeRepository{}
	_, _, err = newService(t, repo, newStore()).List(context.Background(), uuid.New(), "recruiter", domain.ListFilter{Search: ptr(" 100%_off "), Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if *repo.filter.Search != `100\%\_off` {
		t.Fatalf("search = %q, want wildcards escaped", *repo.filter.Search)
	}

	repo = &fakeRepository{}
	_, _, err = newService(t, repo, newStore()).List(context.Background(), uuid.New(), "recruiter", domain.ListFilter{Search: ptr("  "), Limit: 20})
	if err != nil || repo.filter.Search != nil {
		t.Fatalf("a blank search should mean no filter: search=%v err=%v", repo.filter.Search, err)
	}
}

// The JD text is read before the write, so a storage outage cannot report a saved edit as failed.
func TestUpdateReadsTextBeforeWriting(t *testing.T) {
	repo, store := storedPosting(t, validText())
	store.downloadErr = errBoom
	_, err := newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", uuid.New(),
		domain.Edit{Title: domain.StringChange{Present: true, Value: ptr("New title")}})
	assertCode(t, err, apperror.CodeInternal)
	if repo.updateCalls != 0 {
		t.Fatal("the edit was written although the response could not be built")
	}

	repo, store = storedPosting(t, validText())
	got, err := newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", uuid.New(),
		domain.Edit{Title: domain.StringChange{Present: true, Value: ptr("New title")}})
	if err != nil || got.RawText != validText() || repo.updateCalls != 1 {
		t.Fatalf("err=%v updates=%d text matches=%t", err, repo.updateCalls, got.RawText == validText())
	}
}
