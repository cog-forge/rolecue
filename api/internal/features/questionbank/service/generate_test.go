package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestGenerateSuccessUsesOwnerAndSize(t *testing.T) {
	repo := readyRepo()
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return questionsOf(3), nil }}
	recruiter := uuid.New()
	got, err := newService(t, repo, gen, 0, 3).Generate(context.Background(), recruiter, "recruiter", uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || repo.savedFor != "posting" || len(repo.saved) != 3 {
		t.Fatalf("got %d questions, saved %d for %q", len(got), len(repo.saved), repo.savedFor)
	}
	if repo.gotRecruit != recruiter {
		t.Fatal("owner did not reach the repository")
	}
	if gen.gotIn.Seniority.Name != "Senior" {
		t.Fatalf("seniority level did not reach the generator: %+v", gen.gotIn.Seniority)
	}
	if gen.gotNum != 3 || len(gen.gotIn.Requirements) != 1 || gen.gotIn.RefinementNote != "skip Docker" {
		t.Fatalf("generator got count=%d input=%+v", gen.gotNum, gen.gotIn)
	}
}

func TestGenerateWhileRejected(t *testing.T) {
	repo := readyRepo()
	repo.posting.Status = domain.PostingRejected
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return questionsOf(3), nil }}
	if _, err := newService(t, repo, gen, 0, 3).Generate(context.Background(), uuid.New(), "recruiter", uuid.New()); err != nil || repo.saveCalls != 1 {
		t.Fatalf("rejected posting could not regenerate its bank: saves=%d err=%v", repo.saveCalls, err)
	}
}

func TestGeneratePreconditions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		role   string
		size   int
		change func(*fakeRepository)
		code   apperror.Code
	}{
		{"candidate", "candidate", 3, func(*fakeRepository) {}, apperror.CodeForbidden},
		{"admin", "admin", 3, func(*fakeRepository) {}, apperror.CodeForbidden},
		{"disabled size", "recruiter", 0, func(*fakeRepository) {}, apperror.CodeQuestionBankUnavailable},
		{"posting not found", "recruiter", 3, func(r *fakeRepository) {
			r.postingErr = apperror.New(apperror.CodeJobPostingNotFound, "no")
		}, apperror.CodeJobPostingNotFound},
		{"posting locked", "recruiter", 3, func(r *fakeRepository) { r.posting.Status = "open" }, apperror.CodeQuestionBankLocked},
		{"bank exists", "recruiter", 3, func(r *fakeRepository) {
			r.existing = []domain.CoreQuestion{{ID: uuid.New(), Content: "q"}}
		}, apperror.CodeQuestionBankExists},
		{"no requirements", "recruiter", 3, func(r *fakeRepository) { r.input.Requirements = nil }, apperror.CodeRequirementsNotConfirmed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := readyRepo()
			tc.change(repo)
			gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return questionsOf(3), nil }}
			_, err := newService(t, repo, gen, 0, tc.size).Generate(context.Background(), uuid.New(), tc.role, uuid.New())
			assertCode(t, err, tc.code)
			if gen.calls != 0 || repo.saveCalls != 0 {
				t.Fatalf("failed precondition reached the model (%d) or storage (%d)", gen.calls, repo.saveCalls)
			}
		})
	}
}

func TestGenerateRetriesOnlyInvalidOutput(t *testing.T) {
	bad := domain.GenerationCandidate{Questions: []string{"only one"}}
	for _, tc := range []struct {
		name      string
		retries   int
		fn        func(call int) (domain.GenerationCandidate, error)
		wantCalls int
		wantCode  apperror.Code
	}{
		{"invalid then valid", 1, func(call int) (domain.GenerationCandidate, error) {
			if call == 1 {
				return bad, nil
			}
			return questionsOf(3), nil
		}, 2, ""},
		{"invalid twice", 1, func(int) (domain.GenerationCandidate, error) { return bad, nil }, 2, apperror.CodeInvalidQuestionGenerationOutput},
		{"invalid without retry budget", 0, func(int) (domain.GenerationCandidate, error) { return bad, nil }, 1, apperror.CodeInvalidQuestionGenerationOutput},
		{"adapter invalid output is retried", 1, func(call int) (domain.GenerationCandidate, error) {
			if call == 1 {
				return domain.GenerationCandidate{}, apperror.New(apperror.CodeInvalidQuestionGenerationOutput, "bad json")
			}
			return questionsOf(3), nil
		}, 2, ""},
		{"transport error is not retried", 1, func(int) (domain.GenerationCandidate, error) {
			return domain.GenerationCandidate{}, errors.New("upstream down")
		}, 1, apperror.CodeQuestionGenerationFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := readyRepo()
			gen := &fakeGenerator{fn: tc.fn}
			_, err := newService(t, repo, gen, tc.retries, 3).Generate(context.Background(), uuid.New(), "recruiter", uuid.New())
			if tc.wantCode == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				assertCode(t, err, tc.wantCode)
				if repo.saveCalls != 0 {
					t.Fatal("invalid output was stored")
				}
			}
			if gen.calls != tc.wantCalls {
				t.Fatalf("model calls = %d, want %d", gen.calls, tc.wantCalls)
			}
		})
	}
}

func TestGenerateStopsWhenContextIsCancelled(t *testing.T) {
	repo := readyRepo()
	ctx, cancel := context.WithCancel(context.Background())
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) {
		cancel()
		return domain.GenerationCandidate{Questions: []string{"x"}}, nil
	}}
	_, err := newService(t, repo, gen, 1, 3).Generate(ctx, uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeQuestionGenerationFailed)
	if gen.calls != 1 {
		t.Fatalf("cancelled context was retried: %d calls", gen.calls)
	}
}

func TestGeneratePassesRepositorySaveError(t *testing.T) {
	repo := readyRepo()
	repo.saveErr = apperror.New(apperror.CodeQuestionBankExists, "raced")
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return questionsOf(2), nil }}
	_, err := newService(t, repo, gen, 0, 2).Generate(context.Background(), uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeQuestionBankExists)
}

func TestGenerateForPractice(t *testing.T) {
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return questionsOf(2), nil }}
	t.Run("success", func(t *testing.T) {
		repo := readyRepo()
		got, err := newService(t, repo, gen, 0, 2).GenerateForPractice(context.Background(), uuid.New(), uuid.New())
		if err != nil || len(got) != 2 || repo.savedFor != "practice" {
			t.Fatalf("got %v, %v, saved for %q", got, err, repo.savedFor)
		}
	})
	t.Run("not the owner", func(t *testing.T) {
		repo := readyRepo()
		repo.ownsErr = apperror.New(apperror.CodeJDNotFound, "no")
		_, err := newService(t, repo, gen, 0, 2).GenerateForPractice(context.Background(), uuid.New(), uuid.New())
		assertCode(t, err, apperror.CodeJDNotFound)
		if repo.inputCalls != 0 || repo.saveCalls != 0 {
			t.Fatal("a foreign JD was read or written")
		}
	})
	t.Run("disabled", func(t *testing.T) {
		_, err := newService(t, readyRepo(), gen, 0, 0).GenerateForPractice(context.Background(), uuid.New(), uuid.New())
		assertCode(t, err, apperror.CodeQuestionBankUnavailable)
	})
}

func TestGenerateStopsWhenCancelledDuringBackoff(t *testing.T) {
	repo := readyRepo()
	ctx, cancel := context.WithCancel(context.Background())
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) {
		// Invalid output schedules a retry after a short pause; cancel while the service waits.
		time.AfterFunc(20*time.Millisecond, cancel)
		return domain.GenerationCandidate{Questions: []string{"only one"}}, nil
	}}
	_, err := newService(t, repo, gen, 1, 3).Generate(ctx, uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeQuestionGenerationFailed)
	if gen.calls != 1 || repo.saveCalls != 0 {
		t.Fatalf("model calls = %d, saves = %d; a cancelled wait must not retry or store", gen.calls, repo.saveCalls)
	}
}
