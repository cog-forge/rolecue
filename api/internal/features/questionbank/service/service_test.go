package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type fakeRepository struct {
	posting     domain.Posting
	postingErr  error
	existing    []domain.CoreQuestion
	input       domain.GenerationInput
	ownsErr     error
	saveErr     error
	opErr       error
	saved       []string
	savedFor    string
	listCalls   int
	inputCalls  int
	saveCalls   int
	opCalls     int
	gotRecruit  uuid.UUID
	gotContent  string
	gotQuestion uuid.UUID
}

func (f *fakeRepository) Posting(_ context.Context, recruiterID, _ uuid.UUID) (domain.Posting, error) {
	f.gotRecruit = recruiterID
	return f.posting, f.postingErr
}
func (f *fakeRepository) CandidateOwnsJD(context.Context, uuid.UUID, uuid.UUID) error {
	return f.ownsErr
}
func (f *fakeRepository) List(context.Context, uuid.UUID) ([]domain.CoreQuestion, error) {
	f.listCalls++
	return f.existing, nil
}
func (f *fakeRepository) GenerationInput(context.Context, uuid.UUID) (domain.GenerationInput, error) {
	f.inputCalls++
	return f.input, nil
}
func (f *fakeRepository) Add(_ context.Context, recruiterID, _ uuid.UUID, content string) (domain.CoreQuestion, error) {
	f.opCalls++
	f.gotRecruit, f.gotContent = recruiterID, content
	return domain.CoreQuestion{ID: uuid.New(), Content: content}, f.opErr
}
func (f *fakeRepository) Update(_ context.Context, recruiterID, _, questionID uuid.UUID, content string) (domain.CoreQuestion, error) {
	f.opCalls++
	f.gotRecruit, f.gotQuestion, f.gotContent = recruiterID, questionID, content
	return domain.CoreQuestion{ID: questionID, Content: content}, f.opErr
}
func (f *fakeRepository) Delete(_ context.Context, recruiterID, _, questionID uuid.UUID) error {
	f.opCalls++
	f.gotRecruit, f.gotQuestion = recruiterID, questionID
	return f.opErr
}
func (f *fakeRepository) SaveForPosting(_ context.Context, recruiterID, _ uuid.UUID, contents []string) ([]domain.CoreQuestion, error) {
	f.saveCalls++
	f.gotRecruit, f.saved, f.savedFor = recruiterID, contents, "posting"
	return toQuestions(contents), f.saveErr
}
func (f *fakeRepository) SaveForPractice(_ context.Context, _, _ uuid.UUID, contents []string) ([]domain.CoreQuestion, error) {
	f.saveCalls++
	f.saved, f.savedFor = contents, "practice"
	return toQuestions(contents), f.saveErr
}

func toQuestions(contents []string) []domain.CoreQuestion {
	out := make([]domain.CoreQuestion, 0, len(contents))
	for _, c := range contents {
		out = append(out, domain.CoreQuestion{ID: uuid.New(), Content: c})
	}
	return out
}

type fakeGenerator struct {
	calls  int
	gotIn  domain.GenerationInput
	gotNum int
	fn     func(call int) (domain.GenerationCandidate, error)
}

func (g *fakeGenerator) Generate(_ context.Context, in domain.GenerationInput, count int) (domain.GenerationCandidate, error) {
	g.calls++
	g.gotIn, g.gotNum = in, count
	return g.fn(g.calls)
}

func questionsOf(n int) domain.GenerationCandidate {
	c := domain.GenerationCandidate{Questions: []string{}}
	for i := 0; i < n; i++ {
		c.Questions = append(c.Questions, fmt.Sprintf("Question number %d?", i+1))
	}
	return c
}

func assertCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var got *apperror.AppError
	if !errors.As(err, &got) || got == nil || got.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func readyRepo() *fakeRepository {
	return &fakeRepository{
		posting: domain.Posting{JDID: uuid.New(), Status: domain.PostingPending},
		input: domain.GenerationInput{
			Requirements:   []domain.ConfirmedRequirement{{Name: "Go", Target: "backend", Type: "required"}},
			RefinementNote: "skip Docker",
		},
	}
}

func newService(t *testing.T, repo Repository, gen Generator, retries, size int) *Service {
	t.Helper()
	s, err := New(repo, gen, retries, size)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

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
	if gen.gotNum != 3 || len(gen.gotIn.Requirements) != 1 || gen.gotIn.RefinementNote != "skip Docker" {
		t.Fatalf("generator got count=%d input=%+v", gen.gotNum, gen.gotIn)
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

func TestValidateCandidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   domain.GenerationCandidate
		size int
		ok   bool
		want []string
	}{
		{"exact count", domain.GenerationCandidate{Questions: []string{"A?", "B?"}}, 2, true, []string{"A?", "B?"}},
		{"trims", domain.GenerationCandidate{Questions: []string{"  A?  ", "B?"}}, 2, true, []string{"A?", "B?"}},
		{"nil list", domain.GenerationCandidate{}, 2, false, nil},
		{"too few", domain.GenerationCandidate{Questions: []string{"A?"}}, 2, false, nil},
		{"extra distinct questions keep the first size", domain.GenerationCandidate{Questions: []string{"A?", "B?", "C?"}}, 2, true, []string{"A?", "B?"}},
		{"case-insensitive duplicate drops below size", domain.GenerationCandidate{Questions: []string{"What is Go?", "what is go?"}}, 2, false, nil},
		{"duplicate removed but count still met", domain.GenerationCandidate{Questions: []string{"A?", "a?", "B?"}}, 2, true, []string{"A?", "B?"}},
		{"blank question", domain.GenerationCandidate{Questions: []string{"A?", "   "}}, 2, false, nil},
		{"NUL character", domain.GenerationCandidate{Questions: []string{"A?", "B\x00?"}}, 2, false, nil},
		{"invalid UTF-8", domain.GenerationCandidate{Questions: []string{"A?", string([]byte{0xff, 0xfe})}}, 2, false, nil},
		{"too long", domain.GenerationCandidate{Questions: []string{"A?", strings.Repeat("界", domain.MaxQuestionRunes+1)}}, 2, false, nil},
		{"max length ok", domain.GenerationCandidate{Questions: []string{"A?", strings.Repeat("界", domain.MaxQuestionRunes)}}, 2, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateCandidate(tc.in, tc.size)
			if !tc.ok {
				assertCode(t, err, apperror.CodeInvalidQuestionGenerationOutput)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.want != nil && strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
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

func TestListScopesThroughPosting(t *testing.T) {
	repo := readyRepo()
	repo.existing = []domain.CoreQuestion{{ID: uuid.New(), Content: "q"}}
	svc := newService(t, repo, &fakeGenerator{}, 0, 3)
	got, err := svc.List(context.Background(), uuid.New(), "recruiter", uuid.New())
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, %v", got, err)
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
