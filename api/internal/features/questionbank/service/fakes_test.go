package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

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
	savedFrom   domain.GenerationInput
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

func (f *fakeRepository) SaveForPractice(_ context.Context, _, _ uuid.UUID, from domain.GenerationInput, contents []string) ([]domain.CoreQuestion, error) {
	f.saveCalls++
	f.savedFrom = from
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
			Seniority:      domain.SeniorityLevel{Name: "Senior"},
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
