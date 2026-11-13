package domain

import (
	"fmt"
	"sort"

	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/google/uuid"
)

// Roadmap-aware practice sampling (KAN-347, RM-06; Learning Roadmap spec rules 8 and 9).
//
// SampleQuestionsPreferring is SampleQuestionsBestEffort with a preference. Every bank question carries a
// rank (lower is preferred) and the random order the sampler works in is sorted by rank, stably, so questions
// of a better rank come first and equally ranked ones stay random. That order is what both parts of the
// sampler follow, so the preference acts where the rules leave a choice:
//  1. The Barem metric cover still comes first. A cover is the smallest set of questions that reaches the most
//     active metrics within the limit; among covers of the same size the better-ranked questions are found
//     first. The preference never costs a metric that a cover can reach.
//  2. The remaining slots are filled in rank order, so unassessed questions and questions of topics that are not
//     achieved yet come before the others, and no fixed set of questions is asked forever (the order inside a
//     rank is random).
//
// Like the best-effort sampler it never refuses for coverage: the Candidate cannot repair a bank.

// SampleQuestionsPreferring picks at most limit questions of bank in the order they will be asked. ranks is
// parallel to bank. It returns ErrQuestionBankNotReady for an empty bank or a limit below 1.
func SampleQuestionsPreferring(bank []SamplingQuestion, ranks []int, active []uuid.UUID, limit int, random RandomSource) ([]uuid.UUID, error) {
	if len(ranks) != len(bank) {
		return nil, fmt.Errorf("sampling: %d ranks for %d questions", len(ranks), len(bank))
	}
	if len(bank) == 0 || limit <= 0 {
		return nil, ErrQuestionBankNotReady
	}
	order := shuffled(len(bank), random)
	sort.SliceStable(order, func(a, b int) bool { return ranks[order[a]] < ranks[order[b]] })
	coverage, err := evaluationdomain.AnalyzeCoverage(bank, active, limit, order)
	if err != nil {
		// More active metrics than the search holds: sample by preference alone, as best effort does.
		coverage = evaluationdomain.Coverage{}
	}
	return coverThenFill(bank, coverage.Cover, order, limit), nil
}
