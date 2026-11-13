package domain

import (
	"testing"

	"github.com/google/uuid"
)

func ranked(n int, rank func(i int) int) ([]SamplingQuestion, []int) {
	q := ids(0xc0, n)
	bank := make([]SamplingQuestion, n)
	ranks := make([]int, n)
	for i := range bank {
		bank[i] = question(q[i])
		ranks[i] = rank(i)
	}
	return bank, ranks
}

func TestPreferringTakesTheBetterRankedQuestionsFirstAndShufflesWithinARank(t *testing.T) {
	bank, ranks := ranked(10, func(i int) int { return i / 4 }) // ranks 0 (4 q), 1 (4 q), 2 (2 q)
	seenFirstSets := map[string]bool{}
	for seed := uint64(0); seed < 40; seed++ {
		picked, err := SampleQuestionsPreferring(bank, ranks, nil, 6, seeded(seed))
		if err != nil {
			t.Fatal(err)
		}
		if len(picked) != 6 {
			t.Fatalf("picked %d, want 6", len(picked))
		}
		for _, id := range picked[:4] {
			if rankOf(bank, ranks, id) != 0 {
				t.Fatalf("seed %d: a rank %d question came before every rank 0 question was taken", seed, rankOf(bank, ranks, id))
			}
		}
		for _, id := range picked[4:] {
			if rankOf(bank, ranks, id) != 1 {
				t.Fatalf("seed %d: the last slots must hold rank 1 questions, got rank %d", seed, rankOf(bank, ranks, id))
			}
		}
		seenFirstSets[idsKey(picked[4:])] = true
	}
	if len(seenFirstSets) < 2 {
		t.Fatal("inside a rank the choice must stay random: the same questions were picked every time")
	}
}

func rankOf(bank []SamplingQuestion, ranks []int, id uuid.UUID) int {
	for i, q := range bank {
		if q.ID == id {
			return ranks[i]
		}
	}
	return -1
}

func TestPreferringKeepsTheMetricCoverAheadOfThePreference(t *testing.T) {
	m := ids(0xa0, 3)
	q := ids(0xb0, 8)
	// Only the three worst-ranked questions cover a metric each; the preferred ones cover nothing.
	bank := []SamplingQuestion{question(q[0]), question(q[1]), question(q[2]), question(q[3]), question(q[4]), question(q[5], m[0]), question(q[6], m[1]), question(q[7], m[2])}
	ranks := []int{0, 0, 0, 0, 0, 3, 3, 3}
	for seed := uint64(0); seed < 40; seed++ {
		picked, err := SampleQuestionsPreferring(bank, ranks, m, 5, seeded(seed))
		if err != nil {
			t.Fatal(err)
		}
		assertCovers(t, bank, picked, m)
		assertDistinctFromBank(t, bank, picked)
		if len(picked) != 5 {
			t.Fatalf("picked %d, want 5", len(picked))
		}
	}
}

func TestPreferringChoosesTheBetterRankedOfTwoEqualCovers(t *testing.T) {
	m := ids(0xa0, 1)
	q := ids(0xb0, 4)
	// Any one of the four covers the single metric: the rank 0 one must be the cover, every time.
	bank := []SamplingQuestion{question(q[0], m[0]), question(q[1], m[0]), question(q[2], m[0]), question(q[3], m[0])}
	ranks := []int{2, 2, 0, 2}
	for seed := uint64(0); seed < 40; seed++ {
		picked, err := SampleQuestionsPreferring(bank, ranks, m, 1, seeded(seed))
		if err != nil || len(picked) != 1 || picked[0] != q[2] {
			t.Fatalf("seed %d: picked %v err=%v, want the rank 0 question", seed, picked, err)
		}
	}
}

func TestPreferringNeverRefusesForCoverageAndNeverExceedsTheBank(t *testing.T) {
	m := ids(0xa0, 3)
	bank, ranks := ranked(3, func(int) int { return 0 })
	picked, err := SampleQuestionsPreferring(bank, ranks, m, 8, seeded(1))
	if err != nil || len(picked) != 3 {
		t.Fatalf("picked %v err=%v, want the 3 questions there are", picked, err)
	}
}

func TestPreferringRefusesAnEmptyBankAndMismatchedRanks(t *testing.T) {
	if _, err := SampleQuestionsPreferring(nil, nil, nil, 3, seeded(1)); err != ErrQuestionBankNotReady {
		t.Fatalf("empty bank: %v, want ErrQuestionBankNotReady", err)
	}
	bank, _ := ranked(3, func(int) int { return 0 })
	if _, err := SampleQuestionsPreferring(bank, []int{0}, nil, 3, seeded(1)); err == nil {
		t.Fatal("ranks that do not match the bank were accepted")
	}
	if _, err := SampleQuestionsPreferring(bank, []int{0, 0, 0}, nil, 0, seeded(1)); err != ErrQuestionBankNotReady {
		t.Fatalf("limit 0: %v, want ErrQuestionBankNotReady", err)
	}
}
