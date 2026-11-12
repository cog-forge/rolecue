package domain

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// ids returns n stable, distinct ids whose first byte is the base, so a failing assertion is readable.
func ids(base byte, n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.UUID{base, byte(i + 1)}
	}
	return out
}

// question builds a bank question that covers the given metrics.
func question(id uuid.UUID, metrics ...uuid.UUID) SamplingQuestion {
	return SamplingQuestion{ID: id, Metrics: metrics}
}

// seeded returns a deterministic random source.
func seeded(seed uint64) RandomSource {
	return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
}

// coveredBy returns the metrics the picked questions cover, restricted to the active ones.
func coveredBy(bank []SamplingQuestion, picked []uuid.UUID, active []uuid.UUID) map[uuid.UUID]bool {
	byID := make(map[uuid.UUID]SamplingQuestion, len(bank))
	for _, q := range bank {
		byID[q.ID] = q
	}
	covered := map[uuid.UUID]bool{}
	for _, id := range picked {
		for _, m := range byID[id].Metrics {
			if slices.Contains(active, m) {
				covered[m] = true
			}
		}
	}
	return covered
}

func assertCovers(t *testing.T, bank []SamplingQuestion, picked, active []uuid.UUID) {
	t.Helper()
	covered := coveredBy(bank, picked, active)
	for _, m := range active {
		if !covered[m] {
			t.Fatalf("picked %v does not cover active metric %v", picked, m)
		}
	}
}

func assertDistinctFromBank(t *testing.T, bank []SamplingQuestion, picked []uuid.UUID) {
	t.Helper()
	inBank := map[uuid.UUID]bool{}
	for _, q := range bank {
		inBank[q.ID] = true
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range picked {
		if !inBank[id] || seen[id] {
			t.Fatalf("picked %v holds a question outside the bank or twice", picked)
		}
		seen[id] = true
	}
}

func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	if err == nil || !apperror.IsCode(err, want) {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

func TestSampleQuestionsCoversEveryActiveMetric(t *testing.T) {
	m := ids(0xa0, 3)
	q := ids(0xb0, 6)
	bank := []SamplingQuestion{
		question(q[0], m[0]), question(q[1], m[0]), question(q[2], m[1]),
		question(q[3], m[1]), question(q[4], m[2]), question(q[5], m[2]),
	}
	for seed := range uint64(200) {
		picked, err := SampleQuestions(bank, m, 3, seeded(seed))
		if err != nil {
			t.Fatalf("seed %d: SampleQuestions() error = %v", seed, err)
		}
		if len(picked) != 3 {
			t.Fatalf("seed %d: picked %d questions, want 3", seed, len(picked))
		}
		assertDistinctFromBank(t, bank, picked)
		assertCovers(t, bank, picked, m)
	}
}

func TestSampleQuestionsSingleMetric(t *testing.T) {
	m := ids(0xa0, 1)
	q := ids(0xb0, 5)
	// Only the last question carries the metric: it must always be picked, wherever the limit is 1.
	bank := []SamplingQuestion{question(q[0]), question(q[1]), question(q[2]), question(q[3]), question(q[4], m[0])}
	for seed := range uint64(50) {
		picked, err := SampleQuestions(bank, m, 1, seeded(seed))
		if err != nil || len(picked) != 1 || picked[0] != q[4] {
			t.Fatalf("seed %d: SampleQuestions() = %v, %v, want only the covering question", seed, picked, err)
		}
	}
}

func TestSampleQuestionsRefusesWhenBankCannotCoverWithinLimit(t *testing.T) {
	m := ids(0xa0, 3)
	q := ids(0xb0, 3)
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[1]), question(q[2], m[2])}
	picked, err := SampleQuestions(bank, m, 2, seeded(1))
	assertCode(t, err, apperror.CodeQuestionBankCoverageInsufficient)
	if picked != nil {
		t.Fatalf("picked = %v, want nothing on a refusal", picked)
	}
}

func TestSampleQuestionsRefusesWhenNoQuestionCoversAMetric(t *testing.T) {
	m := ids(0xa0, 3)
	q := ids(0xb0, 4)
	// The bank has rubrics, but nothing assesses m[2]: it is a missing metric, never a zero.
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[1]), question(q[2], m[0], m[1]), question(q[3])}
	_, err := SampleQuestions(bank, m, 4, seeded(1))
	assertCode(t, err, apperror.CodeQuestionBankCoverageInsufficient)
}

func TestSampleQuestionsRubricOnlyOnInactiveMetricsIsRefused(t *testing.T) {
	m := ids(0xa0, 2)
	q := ids(0xb0, 2)
	// The bank carries rubric data, so the coverage rule applies, but only for a metric that is not active.
	bank := []SamplingQuestion{question(q[0], m[1]), question(q[1], m[1])}
	_, err := SampleQuestions(bank, m[:1], 2, seeded(1))
	assertCode(t, err, apperror.CodeQuestionBankCoverageInsufficient)
}

func TestSampleQuestionsIgnoresInactiveMetrics(t *testing.T) {
	m := ids(0xa0, 3)
	q := ids(0xb0, 3)
	// m[2] has weight zero, so it is not in the active list; nothing assesses it and that is fine.
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[1]), question(q[2], m[0], m[1])}
	for seed := range uint64(50) {
		picked, err := SampleQuestions(bank, m[:2], 2, seeded(seed))
		if err != nil {
			t.Fatalf("seed %d: SampleQuestions() error = %v", seed, err)
		}
		assertCovers(t, bank, picked, m[:2])
	}
}

func TestSampleQuestionsOneQuestionCoveringManyMetrics(t *testing.T) {
	m := ids(0xa0, 4)
	q := ids(0xb0, 5)
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[1]), question(q[2], m[0], m[1], m[2], m[3]), question(q[3], m[2]), question(q[4])}
	for seed := range uint64(100) {
		picked, err := SampleQuestions(bank, m, 1, seeded(seed))
		if err != nil || len(picked) != 1 || picked[0] != q[2] {
			t.Fatalf("seed %d: SampleQuestions() = %v, %v, want only the question that covers everything", seed, picked, err)
		}
	}
}

// A greedy cover can overshoot the limit although a cover exists. G looks best (it covers 4 metrics) but
// leaves 3 and 6 to two more questions, while P and Q cover everything. A bank that can cover must not be
// refused, so the sampler falls back to an exact search.
func TestSampleQuestionsFindsACoverThatGreedyMisses(t *testing.T) {
	m := ids(0xa0, 6)
	q := ids(0xb0, 3)
	bank := []SamplingQuestion{
		question(q[0], m[0], m[1], m[3], m[4]), // G
		question(q[1], m[0], m[1], m[2]),       // P
		question(q[2], m[3], m[4], m[5]),       // Q
	}
	for seed := range uint64(100) {
		picked, err := SampleQuestions(bank, m, 2, seeded(seed))
		if err != nil {
			t.Fatalf("seed %d: SampleQuestions() error = %v, want a cover of P and Q", seed, err)
		}
		slices.SortFunc(picked, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
		if !slices.Equal(picked, []uuid.UUID{q[1], q[2]}) {
			t.Fatalf("seed %d: picked %v, want P and Q", seed, picked)
		}
	}
}

func TestSampleQuestionsBankSmallerThanLimitReturnsTheWholeBank(t *testing.T) {
	m := ids(0xa0, 2)
	q := ids(0xb0, 3)
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[1]), question(q[2])}
	picked, err := SampleQuestions(bank, m, 8, seeded(3))
	if err != nil {
		t.Fatalf("SampleQuestions() error = %v", err)
	}
	if len(picked) != 3 {
		t.Fatalf("picked %d questions, want the 3 of the bank, never more", len(picked))
	}
	assertDistinctFromBank(t, bank, picked)
}

func TestSampleQuestionsSmallBankThatCannotCoverIsRefusedEvenUnderTheLimit(t *testing.T) {
	m := ids(0xa0, 2)
	q := ids(0xb0, 2)
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[0])}
	_, err := SampleQuestions(bank, m, 8, seeded(3))
	assertCode(t, err, apperror.CodeQuestionBankCoverageInsufficient)
}

func TestSampleQuestionsQuestionWithoutCriteriaCoversNothingButCanFill(t *testing.T) {
	m := ids(0xa0, 1)
	q := ids(0xb0, 4)
	// q[3] has a rubric with no criteria, q[1] and q[2] have no rubric: all three are empty here.
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1]), question(q[2]), question(q[3])}
	sawFiller := map[uuid.UUID]bool{}
	for seed := range uint64(100) {
		picked, err := SampleQuestions(bank, m, 3, seeded(seed))
		if err != nil {
			t.Fatalf("seed %d: SampleQuestions() error = %v", seed, err)
		}
		assertCovers(t, bank, picked, m)
		for _, id := range picked {
			sawFiller[id] = true
		}
	}
	for _, id := range q[1:] {
		if !sawFiller[id] {
			t.Fatalf("question %v never filled a slot, want empty questions to take part in the fill", id)
		}
	}
}

func TestSampleQuestionsRubricLessBankIsAPlainRandomSample(t *testing.T) {
	m := ids(0xa0, 5)
	q := ids(0xb0, 12)
	bank := make([]SamplingQuestion, len(q))
	for i, id := range q {
		bank[i] = question(id)
	}
	sets := map[string]bool{}
	for seed := range uint64(200) {
		picked, err := SampleQuestions(bank, m, 4, seeded(seed))
		if err != nil {
			t.Fatalf("seed %d: a bank without any rubric must behave as today, got error = %v", seed, err)
		}
		if len(picked) != 4 {
			t.Fatalf("seed %d: picked %d questions, want 4", seed, len(picked))
		}
		assertDistinctFromBank(t, bank, picked)
		sorted := slices.Clone(picked)
		slices.SortFunc(sorted, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
		sets[idsKey(sorted)] = true
	}
	if len(sets) < 50 {
		t.Fatalf("200 seeds gave %d different samples, want a random spread", len(sets))
	}
}

func TestSampleQuestionsWithoutActiveMetricsIsAPlainRandomSample(t *testing.T) {
	m := ids(0xa0, 2)
	q := ids(0xb0, 6)
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[1]), question(q[2]), question(q[3]), question(q[4]), question(q[5])}
	picked, err := SampleQuestions(bank, nil, 3, seeded(1))
	if err != nil || len(picked) != 3 {
		t.Fatalf("SampleQuestions() = %v, %v, want 3 questions and no coverage requirement", picked, err)
	}
}

func TestSampleQuestionsIsDeterministicForTheSameSource(t *testing.T) {
	m := ids(0xa0, 4)
	q := ids(0xb0, 20)
	bank := make([]SamplingQuestion, len(q))
	for i, id := range q {
		bank[i] = question(id, m[i%4])
	}
	first, err := SampleQuestions(bank, m, 8, seeded(42))
	if err != nil {
		t.Fatal(err)
	}
	second, err := SampleQuestions(bank, m, 8, seeded(42))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(first, second) {
		t.Fatalf("same seed gave %v then %v, want the same sample", first, second)
	}
}

// Different seeds must give different samples: the sampler is not a fixed function of the bank, so
// Candidates do not all see the same questions.
func TestSampleQuestionsStaysRandomAcrossSeeds(t *testing.T) {
	m := ids(0xa0, 4)
	q := ids(0xb0, 20)
	bank := make([]SamplingQuestion, len(q))
	for i, id := range q {
		bank[i] = question(id, m[i%4])
	}
	sets := map[string]bool{}
	first := map[uuid.UUID]bool{}
	for seed := range uint64(300) {
		picked, err := SampleQuestions(bank, m, 8, seeded(seed))
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		assertCovers(t, bank, picked, m)
		sorted := slices.Clone(picked)
		slices.SortFunc(sorted, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
		sets[idsKey(sorted)] = true
		first[picked[0]] = true
	}
	if len(sets) < 100 {
		t.Fatalf("300 seeds gave %d different samples, want a random spread", len(sets))
	}
	if len(first) < 5 {
		t.Fatalf("300 seeds opened with %d different questions, want the opener to vary too", len(first))
	}
}

// Every question of the bank can be picked: the cover does not lock onto a favourite.
func TestSampleQuestionsEveryQuestionIsReachable(t *testing.T) {
	m := ids(0xa0, 3)
	q := ids(0xb0, 12)
	bank := make([]SamplingQuestion, len(q))
	for i, id := range q {
		bank[i] = question(id, m[i%3])
	}
	seen := map[uuid.UUID]bool{}
	for seed := range uint64(300) {
		picked, err := SampleQuestions(bank, m, 5, seeded(seed))
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range picked {
			seen[id] = true
		}
	}
	if len(seen) != len(q) {
		t.Fatalf("only %d of %d bank questions were ever picked", len(seen), len(q))
	}
}

// The cover comes first, then the random fill: the first k positions alone cover every active metric,
// where k is the size of the cover, so a session that ends early still has evidence for every metric
// up to that point.
func TestSampleQuestionsPutsTheCoverFirst(t *testing.T) {
	m := ids(0xa0, 3)
	q := ids(0xb0, 12)
	bank := make([]SamplingQuestion, len(q))
	for i, id := range q {
		// Three metric groups of four questions; a minimal cover needs one of each.
		bank[i] = question(id, m[i%3])
	}
	for seed := range uint64(200) {
		picked, err := SampleQuestions(bank, m, 8, seeded(seed))
		if err != nil {
			t.Fatal(err)
		}
		assertCovers(t, bank, picked[:3], m)
	}
}

func TestSampleQuestionsOrdersTheCoverByNewCoverage(t *testing.T) {
	m := ids(0xa0, 4)
	q := ids(0xb0, 3)
	// The wide question is always first, whatever the seed; the others follow.
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[0], m[1], m[2]), question(q[2], m[3])}
	for seed := range uint64(50) {
		picked, err := SampleQuestions(bank, m, 3, seeded(seed))
		if err != nil || picked[0] != q[1] {
			t.Fatalf("seed %d: SampleQuestions() = %v, %v, want the widest question first", seed, picked, err)
		}
	}
}

func TestSampleQuestionsDoesNotChangeItsInput(t *testing.T) {
	m := ids(0xa0, 2)
	q := ids(0xb0, 5)
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[1]), question(q[2], m[0]), question(q[3]), question(q[4], m[1])}
	before := slices.Clone(bank)
	activeBefore := slices.Clone(m)
	if _, err := SampleQuestions(bank, m, 3, seeded(9)); err != nil {
		t.Fatal(err)
	}
	for i := range bank {
		if bank[i].ID != before[i].ID || !slices.Equal(bank[i].Metrics, before[i].Metrics) {
			t.Fatalf("bank[%d] changed", i)
		}
	}
	if !slices.Equal(m, activeBefore) {
		t.Fatal("active metrics changed")
	}
}

func TestSampleQuestionsDuplicateActiveMetricsCountOnce(t *testing.T) {
	m := ids(0xa0, 2)
	q := ids(0xb0, 2)
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[1])}
	picked, err := SampleQuestions(bank, []uuid.UUID{m[0], m[1], m[0]}, 2, seeded(1))
	if err != nil || len(picked) != 2 {
		t.Fatalf("SampleQuestions() = %v, %v, want both questions", picked, err)
	}
}

func TestSampleQuestionsEmptyBankOrLimitIsNotReady(t *testing.T) {
	m := ids(0xa0, 1)
	q := ids(0xb0, 1)
	_, err := SampleQuestions(nil, m, 8, seeded(1))
	assertCode(t, err, apperror.CodeQuestionBankNotReady)
	_, err = SampleQuestions([]SamplingQuestion{question(q[0], m[0])}, m, 0, seeded(1))
	assertCode(t, err, apperror.CodeQuestionBankNotReady)
}

func TestSampleQuestionsRefusesMoreMetricsThanItCanSearch(t *testing.T) {
	m := ids(0xa0, 70)
	q := ids(0xb0, 1)
	bank := []SamplingQuestion{question(q[0], m...)}
	if _, err := SampleQuestions(bank, m, 1, seeded(1)); err == nil {
		t.Fatal("SampleQuestions() error = nil, want an error for more active metrics than the search supports")
	}
}

// scripted is a random source that replays fixed answers, to show the result depends on the injected
// source and on nothing else.
type scripted struct {
	values []int
	next   int
}

func (s *scripted) IntN(n int) int {
	v := s.values[s.next%len(s.values)] % n
	s.next++
	return v
}

func TestSampleQuestionsUsesOnlyTheInjectedSource(t *testing.T) {
	m := ids(0xa0, 2)
	q := ids(0xb0, 6)
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[0]), question(q[2], m[1]), question(q[3], m[1]), question(q[4]), question(q[5])}
	run := func() []uuid.UUID {
		picked, err := SampleQuestions(bank, m, 4, &scripted{values: []int{0, 3, 1, 2, 5, 4}})
		if err != nil {
			t.Fatal(err)
		}
		return picked
	}
	if a, b := run(), run(); !slices.Equal(a, b) {
		t.Fatalf("a scripted source gave %v then %v, want the same sample", a, b)
	}
}

func idsKey(list []uuid.UUID) string {
	out := make([]byte, 0, len(list)*16)
	for _, id := range list {
		out = append(out, id[:]...)
	}
	return string(out)
}
