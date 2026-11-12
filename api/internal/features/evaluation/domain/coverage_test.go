package domain

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

func metricIDs(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.UUID{0xA0, byte(i + 1)}
	}
	return out
}

func covQuestion(n byte, metrics ...uuid.UUID) CoverageQuestion {
	return CoverageQuestion{ID: uuid.UUID{0xB0, n}, Metrics: metrics}
}

func mustCheck(t *testing.T, bank []CoverageQuestion, active []uuid.UUID, limit int) Coverage {
	t.Helper()
	got, err := CheckCoverage(bank, active, limit)
	if err != nil {
		t.Fatalf("CheckCoverage: %v", err)
	}
	return got
}

func TestCoverageSingleMetric(t *testing.T) {
	m := metricIDs(1)
	got := mustCheck(t, []CoverageQuestion{covQuestion(1, m[0]), covQuestion(2)}, m, 2)
	if !got.Passes || got.Covered != 1 || got.Total != 1 || len(got.Missing) != 0 || !got.Assessed {
		t.Fatalf("coverage = %+v, want a pass 1/1", got)
	}
}

func TestCoverageTwoMetricsOneQuestionEach(t *testing.T) {
	m := metricIDs(2)
	bank := []CoverageQuestion{covQuestion(1, m[0]), covQuestion(2, m[1])}
	if got := mustCheck(t, bank, m, 2); !got.Passes || got.Covered != 2 {
		t.Fatalf("limit 2 = %+v, want a pass", got)
	}
	got := mustCheck(t, bank, m, 1)
	if got.Passes || got.Covered != 1 || !got.ExceedsLimit || got.MinQuestions != 2 || len(got.Missing) != 0 {
		t.Fatalf("limit 1 = %+v, want a fail that needs 2 questions, nothing missing from the bank", got)
	}
}

func TestCoverageNamesTheMetricNoQuestionCovers(t *testing.T) {
	m := metricIDs(3)
	bank := []CoverageQuestion{covQuestion(1, m[0], m[1]), covQuestion(2, m[0])}
	got := mustCheck(t, bank, m, 8)
	if got.Passes || got.Covered != 2 || got.Total != 3 || !slices.Equal(got.Missing, []uuid.UUID{m[2]}) || got.ExceedsLimit {
		t.Fatalf("coverage = %+v, want m3 missing and 2/3", got)
	}
}

func TestCoverageOneQuestionCoveringManyMetrics(t *testing.T) {
	m := metricIDs(5)
	got := mustCheck(t, []CoverageQuestion{covQuestion(1, m...)}, m, 1)
	if !got.Passes || got.Covered != 5 || len(got.Cover) != 1 {
		t.Fatalf("coverage = %+v, want one question to cover everything", got)
	}
}

func TestCoverageMetricCoveredByManyQuestions(t *testing.T) {
	m := metricIDs(2)
	bank := []CoverageQuestion{covQuestion(1, m[0]), covQuestion(2, m[0]), covQuestion(3, m[0], m[1])}
	got := mustCheck(t, bank, m, 1)
	if !got.Passes || len(got.Cover) != 1 || got.Cover[0] != 2 {
		t.Fatalf("coverage = %+v, want the one wide question", got)
	}
}

func TestCoverageIgnoresInactiveMetrics(t *testing.T) {
	m := metricIDs(4)
	active := m[:2]
	// The bank covers m3 and m4 (zero weight, not active) but only m1 of the active ones.
	bank := []CoverageQuestion{covQuestion(1, m[0], m[2], m[3]), covQuestion(2, m[3])}
	got := mustCheck(t, bank, active, 8)
	if got.Passes || got.Total != 2 || got.Covered != 1 || !slices.Equal(got.Missing, []uuid.UUID{m[1]}) {
		t.Fatalf("coverage = %+v, want m2 missing", got)
	}
}

func TestCoverageRubricOnlyOnInactiveMetricsFails(t *testing.T) {
	m := metricIDs(3)
	got := mustCheck(t, []CoverageQuestion{covQuestion(1, m[2])}, m[:2], 4)
	if got.Passes || !got.Assessed || got.Covered != 0 || len(got.Missing) != 2 {
		t.Fatalf("coverage = %+v, want a fail with both active metrics missing", got)
	}
}

func TestCoverageBankSmallerThanTheLimit(t *testing.T) {
	m := metricIDs(2)
	bank := []CoverageQuestion{covQuestion(1, m[0]), covQuestion(2, m[1])}
	if got := mustCheck(t, bank, m, 20); !got.Passes || len(got.Cover) != 2 {
		t.Fatalf("coverage = %+v, want a pass with the whole small bank", got)
	}
	if got := mustCheck(t, bank[:1], m, 20); got.Passes || !slices.Equal(got.Missing, []uuid.UUID{m[1]}) {
		t.Fatalf("coverage = %+v, want a small bank that cannot cover to fail", got)
	}
}

func TestCoverageFindsACoverGreedyMisses(t *testing.T) {
	m := metricIDs(4)
	// Greedy takes the wide question first and then needs two more; two narrow ones are enough.
	bank := []CoverageQuestion{covQuestion(1, m[0], m[1]), covQuestion(2, m[2], m[3]), covQuestion(3, m[1], m[2])}
	got := mustCheck(t, bank, m, 2)
	if !got.Passes || len(got.Cover) != 2 {
		t.Fatalf("coverage = %+v, want the exact two-question cover", got)
	}
}

func TestCoverageWithoutRubricDataIsUnassessedAndPasses(t *testing.T) {
	m := metricIDs(3)
	got := mustCheck(t, []CoverageQuestion{covQuestion(1), covQuestion(2)}, m, 4)
	if !got.Passes || got.Assessed {
		t.Fatalf("coverage = %+v, want a bank with no rubric data to pass unassessed (plain random)", got)
	}
}

func TestCoverageWithoutActiveMetricsPasses(t *testing.T) {
	m := metricIDs(2)
	got := mustCheck(t, []CoverageQuestion{covQuestion(1, m[0])}, nil, 4)
	if !got.Passes || got.Assessed || got.Total != 0 {
		t.Fatalf("coverage = %+v, want zero active metrics to pass unassessed", got)
	}
}

func TestCoverageEmptyBankOrLimitIsNotCovered(t *testing.T) {
	m := metricIDs(1)
	if got := mustCheck(t, nil, m, 4); got.Passes {
		t.Fatalf("empty bank = %+v, want not covered", got)
	}
	if got := mustCheck(t, []CoverageQuestion{covQuestion(1, m[0])}, m, 0); got.Passes {
		t.Fatalf("limit 0 = %+v, want not covered", got)
	}
}

func TestCoverageDuplicateActiveMetricsCountOnce(t *testing.T) {
	m := metricIDs(2)
	got := mustCheck(t, []CoverageQuestion{covQuestion(1, m[0], m[1])}, []uuid.UUID{m[0], m[1], m[0]}, 1)
	if !got.Passes || got.Total != 2 {
		t.Fatalf("coverage = %+v, want 2 distinct active metrics", got)
	}
}

func TestCoverageBestEffortCoverIsTheWidestWithinTheLimit(t *testing.T) {
	m := metricIDs(5)
	// Limit 2, five metrics: the best two questions cover four metrics (q1 + q2), not the three of q3 + q4.
	bank := []CoverageQuestion{
		covQuestion(1, m[0], m[1]), covQuestion(2, m[2], m[3]),
		covQuestion(3, m[0], m[1], m[2]), covQuestion(4, m[2]),
	}
	got := mustCheck(t, bank, m, 2)
	if got.Passes || got.Covered != 4 || len(got.Cover) != 2 || !slices.Equal(got.Missing, []uuid.UUID{m[4]}) {
		t.Fatalf("coverage = %+v, want 4/5 with m5 missing", got)
	}
}

func TestCoverageCoverIsOrderedByNewCoverage(t *testing.T) {
	m := metricIDs(4)
	bank := []CoverageQuestion{covQuestion(1, m[0]), covQuestion(2, m[1], m[2], m[3])}
	got := mustCheck(t, bank, m, 2)
	if !got.Passes || got.Cover[0] != 1 {
		t.Fatalf("cover = %v, want the wide question first", got.Cover)
	}
}

func TestCoverageOrderOnlyChoosesAmongEqualCovers(t *testing.T) {
	m := metricIDs(2)
	bank := []CoverageQuestion{covQuestion(1, m[0], m[1]), covQuestion(2, m[0], m[1])}
	first, _ := AnalyzeCoverage(bank, m, 1, []int{0, 1})
	second, _ := AnalyzeCoverage(bank, m, 1, []int{1, 0})
	if first.Cover[0] != 0 || second.Cover[0] != 1 {
		t.Fatalf("covers = %v and %v, want the order to pick between equal questions", first.Cover, second.Cover)
	}
	if first.Passes != second.Passes || first.Covered != second.Covered {
		t.Fatal("the order must never change the verdict")
	}
}

func TestCoverageRefusesMoreMetricsThanItCanSearch(t *testing.T) {
	m := metricIDs(MaxCoverageMetrics + 1)
	if _, err := CheckCoverage([]CoverageQuestion{covQuestion(1, m...)}, m, 4); err == nil {
		t.Fatal("want an error for more metrics than the bitmask holds")
	}
}

func TestCoverageDoesNotChangeItsInput(t *testing.T) {
	m := metricIDs(3)
	bank := []CoverageQuestion{covQuestion(1, m[0]), covQuestion(2, m[1], m[2])}
	active := slices.Clone(m)
	_ = mustCheck(t, bank, active, 2)
	if !slices.Equal(active, m) || !slices.Equal(bank[1].Metrics, []uuid.UUID{m[1], m[2]}) {
		t.Fatal("the inputs were changed")
	}
}

func TestResolveQuestionLimit(t *testing.T) {
	for in, want := range map[int]int{0: DefaultQuestionLimit, -1: DefaultQuestionLimit, 5: 5, 20: 20} {
		if got := ResolveQuestionLimit(in); got != want {
			t.Fatalf("ResolveQuestionLimit(%d) = %d, want %d", in, got, want)
		}
	}
}
