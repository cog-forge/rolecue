package service

import (
	"context"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/google/uuid"
)

// KAN-378: a Candidate's practice bank is checked against the whole metric catalog by the same generation
// retry as a posting's bank (generate_coverage_test.go), keeps the best proposal when the attempts run out,
// and never logs coverage detail, because a Candidate triggers it.

func TestPracticeGenerationChecksTheWholeCatalogAndKeepsTheBestBank(t *testing.T) {
	f := newCoverageFixture()
	gen := &fakeGenerator{fn: func(call int) (domain.GenerationCandidate, error) {
		if call == 1 {
			return f.batch(f.rA, f.rA, f.rA, f.rA), nil
		}
		return f.batch(f.rA, f.rBC, f.rBC, f.rA), nil
	}}
	if _, err := f.service(t, gen, 1, 4).GenerateForPractice(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if f.repo.activeCalls != 1 || f.repo.activeFor != nil {
		t.Fatalf("active metrics read %d times for posting %v, want one read for the catalog (no posting)", f.repo.activeCalls, f.repo.activeFor)
	}
	if gen.calls != 2 || f.repo.savedFor != "practice" {
		t.Fatalf("calls=%d savedFor=%q, want the one retry and a practice save", gen.calls, f.repo.savedFor)
	}
	if saved := savedRubrics(f.repo); saved[1] != f.rBC {
		t.Fatalf("saved rubrics = %v, want the wider proposal", saved)
	}
}

func TestPracticeGenerationLogsNoCoverageDetail(t *testing.T) {
	ctx, logs := observedBankContext()
	f := newCoverageFixture()
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return f.batch(f.rA, f.rA, f.rA, f.rA), nil }}
	if _, err := f.service(t, gen, 1, 4).GenerateForPractice(ctx, uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	final := logs.FilterMessage("question generation attempts ran out, saving the widest bank").All()
	if len(final) != 1 {
		t.Fatalf("final entries = %d, logs:\n%s", len(final), renderBankLogs(logs))
	}
	fields := final[0].ContextMap()
	if _, ok := fields["covered"]; ok {
		t.Fatalf("practice log carries a coverage count: %v", fields)
	}
	if _, ok := fields["total"]; ok {
		t.Fatalf("practice log carries a coverage total: %v", fields)
	}
	if strings.Contains(renderBankLogs(logs), "Question number") {
		t.Fatal("a log line carries question text")
	}
}
