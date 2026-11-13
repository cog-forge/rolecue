package domain

import (
	"github.com/google/uuid"
)

// RoadmapBankQuestion is one bank question as the Roadmap-aware start sees it: what the metric sampler needs,
// the Roadmap topic the question belongs to (nil for a bank written before the Roadmap existed) and whether
// the Candidate already has a valid result for it.
type RoadmapBankQuestion struct {
	SamplingQuestion
	TopicID  *uuid.UUID
	Assessed bool
}

// Ranks of the general practice preference, best first (Learning Roadmap spec rule 9): a question nobody has
// evaluated yet in a topic that is not achieved, then an evaluated one in such a topic, then the same two for
// achieved topics. A question without a topic belongs to no achieved topic.
const (
	rankUnassessedOpenTopic = iota
	rankAssessedOpenTopic
	rankUnassessedAchievedTopic
	rankAssessedAchievedTopic
)

// GeneralPracticeRanks ranks the whole bank for a general practice start. achieved holds the topics whose
// current result is achieved. Nothing is ever excluded: the ranks only decide which questions come first.
func GeneralPracticeRanks(bank []RoadmapBankQuestion, achieved map[uuid.UUID]bool) []int {
	ranks := make([]int, len(bank))
	for i, question := range bank {
		achievedTopic := question.TopicID != nil && achieved[*question.TopicID]
		switch {
		case achievedTopic && question.Assessed:
			ranks[i] = rankAssessedAchievedTopic
		case achievedTopic:
			ranks[i] = rankUnassessedAchievedTopic
		case question.Assessed:
			ranks[i] = rankAssessedOpenTopic
		default:
			ranks[i] = rankUnassessedOpenTopic
		}
	}
	return ranks
}

// FocusedBank is the part of the bank a focused practice may draw from: the questions of that topic and no
// others (rule 6 and 8). It never falls back to another topic when there are too few.
func FocusedBank(bank []RoadmapBankQuestion, topic uuid.UUID) []RoadmapBankQuestion {
	out := make([]RoadmapBankQuestion, 0, len(bank))
	for _, question := range bank {
		if question.TopicID != nil && *question.TopicID == topic {
			out = append(out, question)
		}
	}
	return out
}

// FocusedRanks ranks a focused bank: questions the Candidate has no result for come before those they have.
func FocusedRanks(bank []RoadmapBankQuestion) []int {
	ranks := make([]int, len(bank))
	for i, question := range bank {
		if question.Assessed {
			ranks[i] = 1
		}
	}
	return ranks
}

// SamplingQuestions drops the Roadmap fields, for the metric sampler.
func SamplingQuestions(bank []RoadmapBankQuestion) []SamplingQuestion {
	out := make([]SamplingQuestion, len(bank))
	for i, question := range bank {
		out[i] = question.SamplingQuestion
	}
	return out
}
