package service

import (
	"fmt"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func topicInput(n int) domain.GenerationInput {
	in := domain.GenerationInput{Grading: true}
	for i := 0; i < n; i++ {
		in.Topics = append(in.Topics, domain.RoadmapTopic{ID: uuid.New(), Title: fmt.Sprintf("Topic %d", i)})
	}
	return in
}

// mapped writes counts[i] distinct questions for topic i.
func mapped(in domain.GenerationInput, counts ...int) []domain.GeneratedQuestion {
	var out []domain.GeneratedQuestion
	for i, n := range counts {
		for j := 0; j < n; j++ {
			id := in.Topics[i].ID
			out = append(out, domain.GeneratedQuestion{Content: fmt.Sprintf("Question %d of topic %d?", j, i), TopicID: &id, ExpectedAnswer: ptr("Key points.")})
		}
	}
	return out
}

func ptr(s string) *string { return &s }

func TestValidateCandidateRequiresAnAllowedTopicPerQuestion(t *testing.T) {
	in := topicInput(2)
	good := mapped(in, 3, 2)
	if _, err := validateCandidate(in, domain.GenerationCandidate{Questions: good}, 5); err != nil {
		t.Fatal(err)
	}
	missing := append([]domain.GeneratedQuestion(nil), good...)
	missing[0].TopicID = nil
	foreign := append([]domain.GeneratedQuestion(nil), good...)
	other := uuid.New()
	foreign[1].TopicID = &other
	for name, questions := range map[string][]domain.GeneratedQuestion{"no topic": missing, "foreign topic": foreign} {
		_, err := validateCandidate(in, domain.GenerationCandidate{Questions: questions}, 5)
		assertCode(t, err, apperror.CodeInvalidQuestionGenerationOutput)
		_ = name
	}
}

func TestValidateCandidateIgnoresTopicsWhenTheInputHasNone(t *testing.T) {
	in := gradedInput()
	id := uuid.New()
	items := []domain.GeneratedQuestion{{Content: "A?", ExpectedAnswer: ptr("Key points."), TopicID: &id}}
	out, err := validateCandidate(in, domain.GenerationCandidate{Questions: items}, 1)
	if err != nil || out[0].TopicID != nil {
		t.Fatalf("err=%v topic=%v: a topic id must not survive when no topics were allowed", err, out[0].TopicID)
	}
}

func TestValidateCandidateTrimsToSizeKeepingEveryTopicsFirstFive(t *testing.T) {
	in := topicInput(3)
	// The model wrote 40 questions, front-loading the first topic: 20 / 10 / 10.
	questions := mapped(in, 20, 10, 10)
	out, err := validateCandidate(in, domain.GenerationCandidate{Questions: questions}, 15)
	if err != nil {
		t.Fatal(err)
	}
	counts := domain.CountByTopic(out)
	if len(out) != 15 || counts[in.Topics[0].ID] != 5 || counts[in.Topics[1].ID] != 5 || counts[in.Topics[2].ID] != 5 {
		t.Fatalf("len=%d counts=%v, want 15 questions, 5 per topic", len(out), counts)
	}
}

func TestValidateBankChecksTheDistribution(t *testing.T) {
	in := topicInput(3)
	if err := ValidateBank(in, mapped(in, 5, 6, 5)); err != nil {
		t.Fatal(err)
	}
	for name, counts := range map[string][]int{
		"one topic short":    {5, 4, 5},
		"a topic with none":  {8, 8, 0},
		"all in one":         {16, 0, 0},
		"distinct dedup gap": {5, 5, 4},
	} {
		if err := ValidateBank(in, mapped(in, counts...)); !apperror.IsCode(err, apperror.CodeInvalidQuestionGenerationOutput) {
			t.Errorf("%s: %v, want invalid output", name, err)
		}
	}
	if err := ValidateBank(domain.GenerationInput{}, nil); err != nil {
		t.Fatalf("a bank without topics has no distribution rule: %v", err)
	}
}

func TestFitBatchToTopicsKeepsExactlyTheQuotaOfEachTopic(t *testing.T) {
	in := topicInput(2)
	quotas := []domain.RoadmapTopic{{ID: in.Topics[0].ID, Quota: 3}, {ID: in.Topics[1].ID, Quota: 2}}
	// The model wrote 5 for the first topic and 2 for the second: the first is cut to its quota.
	got, err := FitBatchToTopics(mapped(in, 5, 2), quotas)
	if err != nil {
		t.Fatal(err)
	}
	counts := domain.CountByTopic(got)
	if len(got) != 5 || counts[in.Topics[0].ID] != 3 || counts[in.Topics[1].ID] != 2 {
		t.Fatalf("len=%d counts=%v, want 3 and 2", len(got), counts)
	}
	if got[0].Content != "Question 0 of topic 0?" {
		t.Fatal("the model's order must be kept")
	}
}

func TestFitBatchToTopicsRejectsABatchThatLeavesATopicShort(t *testing.T) {
	in := topicInput(2)
	quotas := []domain.RoadmapTopic{{ID: in.Topics[0].ID, Quota: 3}, {ID: in.Topics[1].ID, Quota: 2}}
	for name, counts := range map[string][]int{"short": {3, 1}, "missing": {5, 0}} {
		if _, err := FitBatchToTopics(mapped(in, counts...), quotas); !apperror.IsCode(err, apperror.CodeInvalidQuestionGenerationOutput) {
			t.Errorf("%s: %v, want invalid output", name, err)
		}
	}
}

func TestFitBatchToTopicsDropsQuestionsOfTopicsNotAskedForInThisBatch(t *testing.T) {
	in := topicInput(3)
	quotas := []domain.RoadmapTopic{{ID: in.Topics[0].ID, Quota: 2}}
	got, err := FitBatchToTopics(mapped(in, 2, 4, 1), quotas)
	if err != nil || len(got) != 2 {
		t.Fatalf("len=%d err=%v, want only the two questions asked for", len(got), err)
	}
}
