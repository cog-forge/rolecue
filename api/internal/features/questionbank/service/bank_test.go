package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestNew(t *testing.T) {
	gen := &fakeGenerator{}
	for _, tc := range []struct {
		name    string
		repo    Repository
		gen     Generator
		retries int
		size    int
	}{
		{"nil repository", nil, gen, 0, 1},
		{"nil generator", &fakeRepository{}, nil, 0, 1},
		{"negative retries", &fakeRepository{}, gen, -1, 1},
		{"too many retries", &fakeRepository{}, gen, 2, 1},
		{"negative size", &fakeRepository{}, gen, 0, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.repo, tc.gen, tc.retries, tc.size); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestListScopesThroughPosting(t *testing.T) {
	repo := readyRepo()
	repo.existing = []domain.CoreQuestion{{ID: uuid.New(), Content: "q"}}
	svc := newService(t, repo, &fakeGenerator{}, 0, 3)
	got, err := svc.List(context.Background(), uuid.New(), "recruiter", uuid.New())
	if err != nil || len(got.Questions) != 1 {
		t.Fatalf("got %v, %v", got, err)
	}
	if !got.Editable || got.PostingStatus != domain.PostingPending || got.MaxQuestionLength != domain.MaxQuestionRunes || got.GenerationSize != 3 {
		t.Fatalf("bank state = %+v, want an editable pending bank with the server limits", got)
	}
	_, err = svc.List(context.Background(), uuid.New(), "candidate", uuid.New())
	assertCode(t, err, apperror.CodeForbidden)
	repo.postingErr = apperror.New(apperror.CodeJobPostingNotFound, "no")
	_, err = svc.List(context.Background(), uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	if repo.listCalls != 1 {
		t.Fatalf("list calls = %d; a missing posting must not read any bank", repo.listCalls)
	}
}

func TestListReportsLockedBank(t *testing.T) {
	repo := readyRepo()
	repo.posting.Status = "open"
	got, err := newService(t, repo, &fakeGenerator{}, 0, 0).List(context.Background(), uuid.New(), "recruiter", uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if got.Editable || got.PostingStatus != "open" || got.GenerationSize != 0 {
		t.Fatalf("bank state = %+v, want a read-only open bank with generation off", got)
	}
	if got.Questions == nil {
		t.Fatal("an empty bank must be [], not null")
	}
}

func TestCurrentBankReadsByJD(t *testing.T) {
	repo := readyRepo()
	repo.existing = []domain.CoreQuestion{{ID: uuid.New(), Content: "q"}}
	got, err := newService(t, repo, &fakeGenerator{}, 0, 0).CurrentBank(context.Background(), uuid.New())
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestEditValidationAndRoles(t *testing.T) {
	for _, tc := range []struct {
		name    string
		role    string
		content string
		code    apperror.Code
	}{
		{"candidate", "candidate", "ok?", apperror.CodeForbidden},
		{"admin", "admin", "ok?", apperror.CodeForbidden},
		{"blank", "recruiter", "   ", apperror.CodeValidation},
		{"too long", "recruiter", strings.Repeat("界", domain.MaxQuestionRunes+1), apperror.CodeValidation},
		{"invalid UTF-8", "recruiter", string([]byte{0xff}), apperror.CodeValidation},
		{"NUL character", "recruiter", "a\x00b", apperror.CodeValidation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := readyRepo()
			svc := newService(t, repo, &fakeGenerator{}, 0, 3)
			_, err := svc.Add(context.Background(), uuid.New(), tc.role, uuid.New(), tc.content)
			assertCode(t, err, tc.code)
			_, err = svc.Update(context.Background(), uuid.New(), tc.role, uuid.New(), uuid.New(), tc.content)
			assertCode(t, err, tc.code)
			if repo.opCalls != 0 {
				t.Fatal("invalid input reached the repository")
			}
		})
	}
	t.Run("delete by candidate", func(t *testing.T) {
		repo := readyRepo()
		err := newService(t, repo, &fakeGenerator{}, 0, 3).Delete(context.Background(), uuid.New(), "candidate", uuid.New(), uuid.New())
		assertCode(t, err, apperror.CodeForbidden)
		if repo.opCalls != 0 {
			t.Fatal("candidate delete reached the repository")
		}
	})
}

func TestEditForwardsOwnerAndTrimmedContent(t *testing.T) {
	repo := readyRepo()
	svc := newService(t, repo, &fakeGenerator{}, 0, 3)
	recruiter, question := uuid.New(), uuid.New()

	added, err := svc.Add(context.Background(), recruiter, "recruiter", uuid.New(), "  What is a goroutine?  ")
	if err != nil || added.Content != "What is a goroutine?" || repo.gotRecruit != recruiter {
		t.Fatalf("add: %+v, %v, owner %v", added, err, repo.gotRecruit)
	}
	updated, err := svc.Update(context.Background(), recruiter, "recruiter", uuid.New(), question, " Updated? ")
	if err != nil || updated.Content != "Updated?" || repo.gotQuestion != question {
		t.Fatalf("update: %+v, %v", updated, err)
	}
	if err := svc.Delete(context.Background(), recruiter, "recruiter", uuid.New(), question); err != nil {
		t.Fatal(err)
	}
}

func TestEditPassesRepositoryErrorsThrough(t *testing.T) {
	sentinel := apperror.New(apperror.CodeQuestionBankLocked, "locked")
	repo := readyRepo()
	repo.opErr = sentinel
	svc := newService(t, repo, &fakeGenerator{}, 0, 3)
	_, err := svc.Add(context.Background(), uuid.New(), "recruiter", uuid.New(), "q?")
	if !errors.Is(err, sentinel) {
		t.Fatalf("add error = %v", err)
	}
	if err := svc.Delete(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New()); !errors.Is(err, sentinel) {
		t.Fatalf("delete error = %v", err)
	}
}
