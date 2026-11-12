package domain

import (
	"math/rand/v2"
	"testing"

	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// The sampler and the coverage gates of the question bank (KAN-377) must never disagree: a bank the
// Recruiter's screen, the submission and the Admin approval accept is a bank every Candidate can start from,
// and a bank they refuse is one the sampler refuses. Both read evaluation/domain.AnalyzeCoverage; these
// tests run the sampler and the pure function on the same random fixtures to keep it so.

// randomBank builds a bank of 1..14 questions over the given metrics, each covering a random subset of them
// (some none at all).
func randomBank(source *rand.Rand, metrics []uuid.UUID) []SamplingQuestion {
	bank := make([]SamplingQuestion, 1+source.IntN(14))
	for i := range bank {
		bank[i] = SamplingQuestion{ID: uuid.UUID{0xB0, byte(i + 1)}}
		for _, metric := range metrics {
			if source.IntN(4) == 0 {
				bank[i].Metrics = append(bank[i].Metrics, metric)
			}
		}
	}
	return bank
}

func TestSamplerAgreesWithTheCoverageRule(t *testing.T) {
	metrics := ids(0xa0, 6)
	source := rand.New(rand.NewPCG(7, 11))
	for trial := 0; trial < 3000; trial++ {
		bank := randomBank(source, metrics)
		active := metrics[:1+source.IntN(len(metrics))]
		limit := 1 + source.IntN(8)

		verdict, err := evaluationdomain.CheckCoverage(bank, active, limit)
		if err != nil {
			t.Fatal(err)
		}
		picked, err := SampleQuestions(bank, active, limit, seeded(uint64(trial)))
		if verdict.Passes {
			if err != nil {
				t.Fatalf("trial %d: the coverage rule passes the bank but the sampler refused it: %v", trial, err)
			}
			if verdict.Assessed {
				assertCovers(t, bank, picked, active)
			}
			continue
		}
		assertCode(t, err, apperror.CodeQuestionBankCoverageInsufficient)
	}
}
