package domain

import (
	"math/rand/v2"
	"slices"
	"testing"

	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// Best-effort sampling for practice (KAN-378): the Candidate cannot repair a bank, so a bank that cannot
// cover every active metric still starts, with the widest cover that fits in the limit.

func TestBestEffortFullyCoverableBankCoversEverything(t *testing.T) {
	m := ids(0xa0, 3)
	q := ids(0xb0, 6)
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[1]), question(q[2], m[2]), question(q[3]), question(q[4]), question(q[5])}
	for seed := uint64(0); seed < 50; seed++ {
		picked, err := SampleQuestionsBestEffort(bank, m, 3, seeded(seed))
		if err != nil {
			t.Fatal(err)
		}
		assertCovers(t, bank, picked, m)
		assertDistinctFromBank(t, bank, picked)
	}
}

func TestBestEffortNeverRefusesAMetricNoQuestionCovers(t *testing.T) {
	m := ids(0xa0, 3)
	q := ids(0xb0, 4)
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[1]), question(q[2], m[0]), question(q[3])}
	if _, err := SampleQuestions(bank, m, 4, seeded(1)); err == nil {
		t.Fatal("the strict sampler must still refuse this bank")
	}
	for seed := uint64(0); seed < 50; seed++ {
		picked, err := SampleQuestionsBestEffort(bank, m, 4, seeded(seed))
		if err != nil {
			t.Fatalf("seed %d: best effort refused: %v", seed, err)
		}
		covered := coveredBy(bank, picked, m)
		if !covered[m[0]] || !covered[m[1]] || covered[m[2]] {
			t.Fatalf("seed %d: covered %v, want m1 and m2 only", seed, covered)
		}
	}
}

func TestBestEffortPartialCoverWhenTheLimitIsTooSmall(t *testing.T) {
	m := ids(0xa0, 4)
	q := ids(0xb0, 5)
	// Limit 2: {q0, q1} covers all four, so it is reachable and must be found even though q4 alone looks wide.
	bank := []SamplingQuestion{question(q[0], m[0], m[1]), question(q[1], m[2], m[3]), question(q[2], m[0]), question(q[3], m[1]), question(q[4], m[0], m[2])}
	for seed := uint64(0); seed < 50; seed++ {
		picked, err := SampleQuestionsBestEffort(bank, m, 2, seeded(seed))
		if err != nil {
			t.Fatal(err)
		}
		assertCovers(t, bank, picked, m)
	}
	// Limit 1: the widest single question covers two metrics.
	for seed := uint64(0); seed < 50; seed++ {
		picked, err := SampleQuestionsBestEffort(bank, m, 1, seeded(seed))
		if err != nil {
			t.Fatal(err)
		}
		if got := len(coveredBy(bank, picked, m)); got != 2 {
			t.Fatalf("seed %d: one question covers %d metrics, want the widest (2)", seed, got)
		}
	}
}

func TestBestEffortPutsTheCoverFirstAndFillsAtRandom(t *testing.T) {
	m := ids(0xa0, 2)
	q := ids(0xb0, 8)
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[1]), question(q[2]), question(q[3]), question(q[4]), question(q[5]), question(q[6]), question(q[7])}
	sets := map[string]bool{}
	for seed := uint64(0); seed < 80; seed++ {
		picked, err := SampleQuestionsBestEffort(bank, m, 4, seeded(seed))
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(picked[:2], q[0]) || !slices.Contains(picked[:2], q[1]) {
			t.Fatalf("seed %d: picked %v, want the cover first", seed, picked)
		}
		sets[idsKey(picked)] = true
	}
	if len(sets) < 5 {
		t.Fatalf("%d different samples in 80 seeds, want the fill to stay random", len(sets))
	}
}

func TestBestEffortRubricLessBankIsAPlainRandomSample(t *testing.T) {
	m := ids(0xa0, 5)
	q := ids(0xb0, 12)
	bank := make([]SamplingQuestion, len(q))
	for i := range q {
		bank[i] = question(q[i])
	}
	sets := map[string]bool{}
	for seed := uint64(0); seed < 60; seed++ {
		picked, err := SampleQuestionsBestEffort(bank, m, 8, seeded(seed))
		if err != nil || len(picked) != 8 {
			t.Fatalf("seed %d: picked %d questions, err %v; want 8 at random", seed, len(picked), err)
		}
		sets[idsKey(picked)] = true
	}
	if len(sets) < 10 {
		t.Fatalf("%d different samples, want a plain random sample", len(sets))
	}
}

func TestBestEffortKeepsTheNotReadyRefusals(t *testing.T) {
	m := ids(0xa0, 1)
	q := ids(0xb0, 1)
	_, err := SampleQuestionsBestEffort(nil, m, 8, seeded(1))
	assertCode(t, err, apperror.CodeQuestionBankNotReady)
	_, err = SampleQuestionsBestEffort([]SamplingQuestion{question(q[0], m[0])}, m, 0, seeded(1))
	assertCode(t, err, apperror.CodeQuestionBankNotReady)
}

func TestBestEffortFallsBackToRandomWhenTheCoverageSearchCannotRun(t *testing.T) {
	m := ids(0xa0, 70)
	q := ids(0xb0, 3)
	bank := []SamplingQuestion{question(q[0], m...), question(q[1]), question(q[2])}
	picked, err := SampleQuestionsBestEffort(bank, m, 2, seeded(1))
	if err != nil || len(picked) != 2 {
		t.Fatalf("picked %v, err %v; want a random sample, never a refusal", picked, err)
	}
}

func TestBestEffortSamplerReachesTheWidestCoverTheRuleFinds(t *testing.T) {
	metrics := ids(0xa0, 6)
	source := rand.New(rand.NewPCG(13, 17))
	for trial := 0; trial < 3000; trial++ {
		bank := randomBank(source, metrics)
		active := metrics[:1+source.IntN(len(metrics))]
		limit := 1 + source.IntN(8)

		verdict, err := evaluationdomain.CheckCoverage(bank, active, limit)
		if err != nil {
			t.Fatal(err)
		}
		picked, err := SampleQuestionsBestEffort(bank, active, limit, seeded(uint64(trial)))
		if err != nil {
			t.Fatalf("trial %d: the best-effort sampler must never refuse, got %v", trial, err)
		}
		assertDistinctFromBank(t, bank, picked)
		if want := min(limit, len(bank)); len(picked) != want {
			t.Fatalf("trial %d: picked %d questions, want %d", trial, len(picked), want)
		}
		if verdict.Assessed && len(coveredBy(bank, picked, active)) != verdict.Covered {
			t.Fatalf("trial %d: the sample covers %d active metrics, the widest cover within %d questions covers %d",
				trial, len(coveredBy(bank, picked, active)), limit, verdict.Covered)
		}
	}
}
