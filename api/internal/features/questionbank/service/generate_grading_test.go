package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func gradedBatch(n int, rubric *uuid.UUID) domain.GenerationCandidate {
	c := domain.GenerationCandidate{Questions: []domain.GeneratedQuestion{}}
	for i := 0; i < n; i++ {
		answer := "Key points of a strong answer."
		c.Questions = append(c.Questions, domain.GeneratedQuestion{Content: questionsOf(n).Questions[i].Content, RubricID: rubric, ExpectedAnswer: &answer})
	}
	return c
}

func TestRecruiterGenerationAsksOnceWithTheOwnersCatalog(t *testing.T) {
	repo := readyRepo()
	allowed := uuid.New()
	repo.catalog = []domain.RubricOption{{ID: allowed, Name: "Explain a mechanism"}}
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return gradedBatch(3, &allowed), nil }}
	recruiter := uuid.New()
	got, err := newService(t, repo, gen, 1, 3).Generate(context.Background(), recruiter, "recruiter", uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if gen.calls != 1 {
		t.Fatalf("generation calls = %d, want exactly one (no separate classification call)", gen.calls)
	}
	if repo.catalogCall != 1 || repo.catalogFor != recruiter {
		t.Fatalf("catalog read %d times for %s; it must be read once for the posting owner", repo.catalogCall, repo.catalogFor)
	}
	if !gen.gotIn.Grading || len(gen.gotIn.Rubrics) != 1 || gen.gotIn.Rubrics[0].ID != allowed {
		t.Fatalf("generator input = %+v, want grading mode with the owner's catalog", gen.gotIn)
	}
	for _, q := range got.Questions {
		if q.RubricID == nil || *q.RubricID != allowed || q.ExpectedAnswer == nil || q.GradingIncomplete {
			t.Fatalf("stored question = %+v, want rubric and expected answer, complete", q)
		}
	}
}

func TestRubricOutsideTheCatalogIsRefusedAndNothingIsStored(t *testing.T) {
	repo := readyRepo()
	repo.catalog = []domain.RubricOption{{ID: uuid.New(), Name: "mine"}}
	someoneElses := uuid.New() // another Recruiter's private rubric, guessed or leaked into the output
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return gradedBatch(3, &someoneElses), nil }}
	_, err := newService(t, repo, gen, 1, 3).Generate(context.Background(), uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeInvalidQuestionGenerationOutput)
	if repo.saveCalls != 0 {
		t.Fatal("a bank with a foreign rubric id was stored")
	}
	if gen.calls != 2 {
		t.Fatalf("calls = %d; invalid output may use the existing single retry and no more", gen.calls)
	}
}

func TestNoSuitableRubricIsStoredAsIncomplete(t *testing.T) {
	repo := readyRepo() // empty catalog
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return gradedBatch(2, nil), nil }}
	got, err := newService(t, repo, gen, 0, 2).Generate(context.Background(), uuid.New(), "recruiter", uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range got.Questions {
		if q.RubricID != nil || !q.GradingIncomplete {
			t.Fatalf("question without a suitable rubric = %+v, want no rubric and grading_incomplete", q)
		}
	}
}

// A practice bank is rubric-complete too (KAN-334): the same single call asks for a rubric and an expected
// answer per question, from the active system templates only (the catalog is read for no owner).
func TestPracticeGenerationAsksForGradingWithSystemRubricsOnly(t *testing.T) {
	repo := readyRepo()
	system := uuid.New()
	repo.catalog = []domain.RubricOption{{ID: system, Name: "system"}}
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return gradedBatch(2, &system), nil }}
	got, err := newService(t, repo, gen, 0, 2).GenerateForPractice(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if !gen.gotIn.Grading || len(gen.gotIn.Rubrics) != 1 || repo.catalogCall != 1 || repo.catalogFor != uuid.Nil {
		t.Fatalf("practice generation input = %+v (catalog reads %d for %s), want grading with the system catalog", gen.gotIn, repo.catalogCall, repo.catalogFor)
	}
	if len(repo.savedGraded) != 2 || repo.savedGraded[0].RubricID == nil || *repo.savedGraded[0].RubricID != system || repo.savedGraded[0].ExpectedAnswer == nil {
		t.Fatalf("saved = %+v, want the rubric and expected answer stored with each practice question", repo.savedGraded)
	}
	// The Candidate-facing result never carries grading data: CoreQuestion has an id and content only.
	if len(got) != 2 || got[0].Content == "" {
		t.Fatalf("got = %+v", got)
	}
}

func TestCatalogFailureStopsBeforeTheModelCall(t *testing.T) {
	repo := readyRepo()
	repo.catalogErr = apperror.New(apperror.CodeInternal, "db down")
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return questionsOf(1), nil }}
	_, err := newService(t, repo, gen, 0, 1).Generate(context.Background(), uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeInternal)
	if gen.calls != 0 {
		t.Fatal("the model was called without a catalog")
	}
}
