package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Metric coverage of a generated bank (KAN-377 for a posting, KAN-378 for a practice bank). Four active
// metrics A..D and three allowed rubrics: rA covers A, rBC covers B and C, rD covers D. The tests use a
// session limit of three questions, so a bank covers every metric only if it holds at least one question of
// each rubric.
type coverageFixture struct {
	repo           *fakeRepository
	rA, rBC, rD    uuid.UUID
	mA, mB, mC, mD uuid.UUID
}

func newCoverageFixture() coverageFixture {
	f := coverageFixture{repo: readyRepo(), rA: uuid.New(), rBC: uuid.New(), rD: uuid.New(),
		mA: uuid.New(), mB: uuid.New(), mC: uuid.New(), mD: uuid.New()}
	f.repo.catalog = []domain.RubricOption{
		{ID: f.rA, Name: "a", Criteria: []domain.RubricOptionCriterion{{MetricID: f.mA}}},
		{ID: f.rBC, Name: "bc", Criteria: []domain.RubricOptionCriterion{{MetricID: f.mB}, {MetricID: f.mC}}},
		{ID: f.rD, Name: "d", Criteria: []domain.RubricOptionCriterion{{MetricID: f.mD}}},
	}
	f.repo.active = []uuid.UUID{f.mA, f.mB, f.mC, f.mD}
	return f
}

// batch builds a graded proposal that uses the given rubrics in order.
func (f coverageFixture) batch(rubrics ...uuid.UUID) domain.GenerationCandidate {
	c := gradedBatch(len(rubrics), nil)
	for i := range c.Questions {
		id := rubrics[i]
		c.Questions[i].RubricID = &id
	}
	return c
}

func (f coverageFixture) service(t *testing.T, gen Generator, retries, size int) *Service {
	t.Helper()
	svc := newService(t, f.repo, gen, retries, size)
	svc.SetQuestionLimit(3)
	return svc
}

func savedRubrics(repo *fakeRepository) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(repo.savedGraded))
	for _, q := range repo.savedGraded {
		out = append(out, *q.RubricID)
	}
	return out
}

func TestUncoverableFirstBankIsRegeneratedOnce(t *testing.T) {
	f := newCoverageFixture()
	gen := &fakeGenerator{fn: func(call int) (domain.GenerationCandidate, error) {
		if call == 1 {
			return f.batch(f.rA, f.rA, f.rA, f.rA), nil
		}
		return f.batch(f.rA, f.rBC, f.rD, f.rA), nil
	}}
	if _, err := f.service(t, gen, 1, 4).Generate(context.Background(), uuid.New(), "recruiter", uuid.New()); err != nil {
		t.Fatal(err)
	}
	if gen.calls != 2 {
		t.Fatalf("calls = %d, want the uncoverable bank to cost exactly the one retry", gen.calls)
	}
	if got := savedRubrics(f.repo); got[1] != f.rBC || got[2] != f.rD {
		t.Fatalf("saved rubrics = %v, want the covering second proposal", got)
	}
}

func TestCoverableFirstBankCostsOneCall(t *testing.T) {
	f := newCoverageFixture()
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return f.batch(f.rA, f.rBC, f.rD, f.rA), nil }}
	if _, err := f.service(t, gen, 1, 4).Generate(context.Background(), uuid.New(), "recruiter", uuid.New()); err != nil {
		t.Fatal(err)
	}
	if gen.calls != 1 {
		t.Fatalf("calls = %d, want a covering bank accepted at once", gen.calls)
	}
}

func TestWhenAttemptsRunOutTheWidestBankIsSavedAnyway(t *testing.T) {
	f := newCoverageFixture()
	// Attempt 1 covers A only; attempt 2 covers A, B and C within three questions (D is missing): wider.
	gen := &fakeGenerator{fn: func(call int) (domain.GenerationCandidate, error) {
		if call == 1 {
			return f.batch(f.rA, f.rA, f.rA, f.rA), nil
		}
		return f.batch(f.rA, f.rBC, f.rBC, f.rA), nil
	}}
	if _, err := f.service(t, gen, 1, 4).Generate(context.Background(), uuid.New(), "recruiter", uuid.New()); err != nil {
		t.Fatalf("an uncoverable bank must still be saved, got %v", err)
	}
	if gen.calls != 2 || f.repo.saveCalls != 1 || len(f.repo.savedGraded) != 4 {
		t.Fatalf("calls=%d saves=%d saved=%d", gen.calls, f.repo.saveCalls, len(f.repo.savedGraded))
	}
	if saved := savedRubrics(f.repo); saved[1] != f.rBC {
		t.Fatalf("saved rubrics = %v, want the wider second proposal", saved)
	}
}

func TestTheWiderFirstBankSurvivesAWorseSecondOne(t *testing.T) {
	f := newCoverageFixture()
	gen := &fakeGenerator{fn: func(call int) (domain.GenerationCandidate, error) {
		if call == 1 {
			return f.batch(f.rA, f.rBC, f.rBC, f.rA), nil
		}
		return f.batch(f.rA, f.rA, f.rA, f.rA), nil
	}}
	if _, err := f.service(t, gen, 1, 4).Generate(context.Background(), uuid.New(), "recruiter", uuid.New()); err != nil {
		t.Fatal(err)
	}
	if saved := savedRubrics(f.repo); saved[1] != f.rBC {
		t.Fatalf("saved rubrics = %v, want the wider first proposal", saved)
	}
}

func TestWithoutARetryTheFirstUncoverableBankIsSaved(t *testing.T) {
	f := newCoverageFixture()
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return f.batch(f.rA, f.rA, f.rA, f.rA), nil }}
	if _, err := f.service(t, gen, 0, 4).Generate(context.Background(), uuid.New(), "recruiter", uuid.New()); err != nil {
		t.Fatal(err)
	}
	if gen.calls != 1 || f.repo.saveCalls != 1 {
		t.Fatalf("calls=%d saves=%d, want one call and a save (a routed task has no retry)", gen.calls, f.repo.saveCalls)
	}
}

func TestAnInvalidRetryAfterAValidUncoverableBankKeepsTheFirst(t *testing.T) {
	f := newCoverageFixture()
	gen := &fakeGenerator{fn: func(call int) (domain.GenerationCandidate, error) {
		if call == 1 {
			return f.batch(f.rA, f.rA, f.rA, f.rA), nil
		}
		return domain.GenerationCandidate{}, apperror.New(apperror.CodeInvalidQuestionGenerationOutput, "bad")
	}}
	if _, err := f.service(t, gen, 1, 4).Generate(context.Background(), uuid.New(), "recruiter", uuid.New()); err != nil {
		t.Fatalf("a valid earlier proposal must be kept, got %v", err)
	}
	if f.repo.saveCalls != 1 {
		t.Fatalf("saves = %d, want the first proposal saved", f.repo.saveCalls)
	}
}

func TestAnInvalidFirstProposalStillFailsTheGeneration(t *testing.T) {
	f := newCoverageFixture()
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) {
		return domain.GenerationCandidate{}, apperror.New(apperror.CodeInvalidQuestionGenerationOutput, "bad")
	}}
	_, err := f.service(t, gen, 0, 4).Generate(context.Background(), uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeInvalidQuestionGenerationOutput)
	if f.repo.saveCalls != 0 {
		t.Fatal("nothing valid was proposed, so nothing may be saved")
	}
}

func TestACoverageCheckThatCannotRunDoesNotCostTheBank(t *testing.T) {
	f := newCoverageFixture()
	for i := 0; i <= evaluationdomain.MaxCoverageMetrics; i++ {
		f.repo.active = append(f.repo.active, uuid.New())
	}
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return f.batch(f.rA, f.rBC, f.rD, f.rA), nil }}
	if _, err := f.service(t, gen, 1, 4).Generate(context.Background(), uuid.New(), "recruiter", uuid.New()); err != nil {
		t.Fatal(err)
	}
	if gen.calls != 1 || f.repo.saveCalls != 1 {
		t.Fatalf("calls=%d saves=%d, want the bank saved after one call", gen.calls, f.repo.saveCalls)
	}
}

func TestPostingGenerationChecksTheActiveMetricsOfThatPosting(t *testing.T) {
	f := newCoverageFixture()
	posting := uuid.New()
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return f.batch(f.rA, f.rBC, f.rD, f.rA), nil }}
	if _, err := f.service(t, gen, 0, 4).Generate(context.Background(), uuid.New(), "recruiter", posting); err != nil {
		t.Fatal(err)
	}
	if f.repo.activeFor == nil || *f.repo.activeFor != posting {
		t.Fatalf("active metrics read for %v, want the posting %s", f.repo.activeFor, posting)
	}
}

func TestMutationsAndTheBankViewCarryTheCoverage(t *testing.T) {
	f := newCoverageFixture()
	f.repo.coverage = evaluationdomain.CoverageReport{Passes: false, Assessed: true, Covered: 3, Total: 4, QuestionLimit: 3,
		MissingMetrics: []evaluationdomain.CoverageMetric{{ID: f.mD, Name: "D"}}}
	svc := f.service(t, &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return f.batch(f.rA), nil }}, 0, 1)
	ctx, recruiter, posting := context.Background(), uuid.New(), uuid.New()

	content := "A question?"
	written, err := svc.WriteQuestion(ctx, recruiter, "recruiter", posting, nil, domain.QuestionPatch{Content: &content})
	if err != nil || written.Coverage == nil || written.Coverage.Covered != 3 {
		t.Fatalf("WriteQuestion coverage = %+v, err %v", written.Coverage, err)
	}
	valid := domain.BulkRubricRequest{QuestionIDs: []uuid.UUID{uuid.New()}, RubricID: uuid.New()}
	valid.PreviousRevisions = map[uuid.UUID]string{valid.QuestionIDs[0]: string(make([]byte, 64))}
	applied, err := svc.ApplyRubric(ctx, recruiter, "recruiter", posting, valid)
	if err != nil || applied.Coverage == nil || len(applied.Coverage.MissingMetrics) != 1 {
		t.Fatalf("ApplyRubric coverage = %+v, err %v", applied.Coverage, err)
	}
	generated, err := f.service(t, &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return f.batch(f.rA, f.rBC, f.rD), nil }}, 0, 3).
		Generate(ctx, recruiter, "recruiter", posting)
	if err != nil || generated.Coverage == nil || generated.Coverage.Passes {
		t.Fatalf("Generate coverage = %+v, err %v", generated.Coverage, err)
	}
	bank, err := svc.List(ctx, recruiter, "recruiter", posting)
	if err != nil || bank.Coverage == nil || bank.Coverage.Total != 4 {
		t.Fatalf("List coverage = %+v, err %v", bank.Coverage, err)
	}
}

func TestAWriteSucceedsEvenWhenItsCoverageCannotBeRead(t *testing.T) {
	f := newCoverageFixture()
	f.repo.coverageErr = errors.New("db down")
	svc := f.service(t, &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return f.batch(f.rA), nil }}, 0, 1)
	content := "A question?"
	written, err := svc.WriteQuestion(context.Background(), uuid.New(), "recruiter", uuid.New(), nil, domain.QuestionPatch{Content: &content})
	if err != nil || written.Coverage != nil {
		t.Fatalf("WriteQuestion = coverage %+v, err %v; want the committed write reported with no coverage", written.Coverage, err)
	}
	if _, err := svc.List(context.Background(), uuid.New(), "recruiter", uuid.New()); err == nil {
		t.Fatal("the bank view must fail loudly when its coverage cannot be read")
	}
}

func TestUncoverableAttemptsOfAPostingLogCountsOnlyNeverQuestionOrMetricNames(t *testing.T) {
	ctx, logs := observedBankContext()
	f := newCoverageFixture()
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return f.batch(f.rA, f.rA, f.rA, f.rA), nil }}
	if _, err := f.service(t, gen, 1, 4).Generate(ctx, uuid.New(), "recruiter", uuid.New()); err != nil {
		t.Fatal(err)
	}
	retried := logs.FilterMessage("question generation attempt cannot cover every metric, retrying").All()
	final := logs.FilterMessage("question generation attempts ran out, saving the widest bank").All()
	if len(retried) != 1 || len(final) != 1 {
		t.Fatalf("uncovered attempt entries = %d retried, %d final, logs:\n%s", len(retried), len(final), renderBankLogs(logs))
	}
	if fields := final[0].ContextMap(); fields["covered"] != int64(1) || fields["total"] != int64(4) || fields["will_retry"] != false {
		t.Fatalf("final fields = %v", fields)
	}
	if rendered := renderBankLogs(logs); strings.Contains(rendered, "Question number") {
		t.Fatalf("a log line carries question text:\n%s", rendered)
	}
}
