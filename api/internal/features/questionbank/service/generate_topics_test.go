package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// bankFor writes counts[i] distinct questions for topic i of the topics the generator was given.
func bankFor(in domain.GenerationInput, counts ...int) domain.GenerationCandidate {
	c := domain.GenerationCandidate{Questions: []domain.GeneratedQuestion{}}
	for i, n := range counts {
		for j := 0; j < n; j++ {
			id := in.Topics[i].ID
			c.Questions = append(c.Questions, domain.GeneratedQuestion{Content: fmt.Sprintf("Question %d of topic %d?", j, i), TopicID: &id, ExpectedAnswer: ptr("Key points.")})
		}
	}
	return c
}

func roadmapTopics(n int) []domain.RoadmapTopic {
	out := make([]domain.RoadmapTopic, n)
	for i := range out {
		out[i] = domain.RoadmapTopic{ID: uuid.New(), Title: fmt.Sprintf("Topic %d", i)}
	}
	return out
}

func TestDraftPracticeBankSizesTheBankForEveryTopicAndGivesEachItsShare(t *testing.T) {
	repo := readyRepo()
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return domain.GenerationCandidate{}, nil }}
	gen.fn = func(int) (domain.GenerationCandidate, error) { return bankFor(gen.gotIn, 5, 5, 5, 5, 5, 5), nil }
	// The configured size (20) cannot hold five questions for each of six topics: the bank is raised to 30.
	got, err := newService(t, repo, gen, 0, 20).DraftPracticeBank(context.Background(), uuid.New(), uuid.New(), roadmapTopics(6))
	if err != nil {
		t.Fatal(err)
	}
	if gen.gotNum != 30 || len(got) != 30 {
		t.Fatalf("asked for %d, got %d, want 30", gen.gotNum, len(got))
	}
	total := 0
	for _, topic := range gen.gotIn.Topics {
		if topic.Quota != 5 {
			t.Fatalf("quota = %d, want 5 each", topic.Quota)
		}
		total += topic.Quota
	}
	if total != 30 || !gen.gotIn.Grading || repo.activeFor != nil {
		t.Fatalf("quota total=%d grading=%v", total, gen.gotIn.Grading)
	}
}

func TestDraftPracticeBankKeepsAConfiguredSizeThatAlreadyFits(t *testing.T) {
	gen := &fakeGenerator{}
	gen.fn = func(int) (domain.GenerationCandidate, error) { return bankFor(gen.gotIn, 13, 13, 12, 12), nil }
	got, err := newService(t, readyRepo(), gen, 0, 50).DraftPracticeBank(context.Background(), uuid.New(), uuid.New(), roadmapTopics(4))
	if err != nil || gen.gotNum != 50 || len(got) != 50 {
		t.Fatalf("asked %d got %d err=%v, want 50", gen.gotNum, len(got), err)
	}
}

func TestDraftPracticeBankRetriesAnUnevenDistributionOnceThenAccepts(t *testing.T) {
	gen := &fakeGenerator{}
	gen.fn = func(call int) (domain.GenerationCandidate, error) {
		if call == 1 {
			return bankFor(gen.gotIn, 7, 4, 4), nil // 15 questions, but two topics are short
		}
		return bankFor(gen.gotIn, 5, 5, 5), nil
	}
	got, err := newService(t, readyRepo(), gen, 1, 15).DraftPracticeBank(context.Background(), uuid.New(), uuid.New(), roadmapTopics(3))
	if err != nil || gen.calls != 2 || len(got) != 15 {
		t.Fatalf("calls=%d len=%d err=%v, want the second attempt accepted", gen.calls, len(got), err)
	}
}

func TestDraftPracticeBankFailsWhenAttemptsRunOutBelowTheDistributionRule(t *testing.T) {
	gen := &fakeGenerator{}
	gen.fn = func(int) (domain.GenerationCandidate, error) { return bankFor(gen.gotIn, 7, 4, 4), nil }
	repo := readyRepo()
	_, err := newService(t, repo, gen, 1, 15).DraftPracticeBank(context.Background(), uuid.New(), uuid.New(), roadmapTopics(3))
	assertCode(t, err, apperror.CodeInvalidQuestionGenerationOutput)
	if gen.calls != 2 || repo.saveCalls != 0 {
		t.Fatalf("calls=%d saves=%d: an unusable distribution is never stored", gen.calls, repo.saveCalls)
	}
}

func TestDraftPracticeBankNeverStoresAndToleratesAnExistingBank(t *testing.T) {
	repo := readyRepo()
	repo.existing = []domain.CoreQuestion{{ID: uuid.New(), Content: "old?"}}
	gen := &fakeGenerator{}
	gen.fn = func(int) (domain.GenerationCandidate, error) { return bankFor(gen.gotIn, 5, 5, 5), nil }
	if _, err := newService(t, repo, gen, 0, 15).DraftPracticeBank(context.Background(), uuid.New(), uuid.New(), roadmapTopics(3)); err != nil {
		t.Fatal(err)
	}
	if repo.saveCalls != 0 {
		t.Fatal("DraftPracticeBank must not store: the caller publishes it with the topics")
	}
}

func TestDraftPracticeBankRefusesWithoutTopicsOrWhenDisabled(t *testing.T) {
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return domain.GenerationCandidate{}, nil }}
	_, err := newService(t, readyRepo(), gen, 0, 15).DraftPracticeBank(context.Background(), uuid.New(), uuid.New(), nil)
	assertCode(t, err, apperror.CodeValidation)
	_, err = newService(t, readyRepo(), gen, 0, 0).DraftPracticeBank(context.Background(), uuid.New(), uuid.New(), roadmapTopics(3))
	assertCode(t, err, apperror.CodeQuestionBankUnavailable)
	if gen.calls != 0 {
		t.Fatal("no model call without topics or when generation is disabled")
	}
}

func TestDraftPracticeBankOfAForeignProfileCallsNoModel(t *testing.T) {
	repo := readyRepo()
	repo.ownsErr = apperror.New(apperror.CodeJDNotFound, "not found")
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return domain.GenerationCandidate{}, nil }}
	_, err := newService(t, repo, gen, 0, 15).DraftPracticeBank(context.Background(), uuid.New(), uuid.New(), roadmapTopics(3))
	assertCode(t, err, apperror.CodeJDNotFound)
	if gen.calls != 0 {
		t.Fatal("a foreign profile must not cost a model call")
	}
}

func TestDraftTopicQuestionsWritesAtLeastTheTopicMinimumAndNeverRepeatsAStoredQuestion(t *testing.T) {
	repo := readyRepo()
	repo.existing = []domain.CoreQuestion{{ID: uuid.New(), Content: "Question 0 of topic 0?"}, {ID: uuid.New(), Content: "Other?"}}
	topic := roadmapTopics(1)[0]
	gen := &fakeGenerator{}
	gen.fn = func(call int) (domain.GenerationCandidate, error) {
		c := bankFor(gen.gotIn, 6) // the model repeats a stored question as its first
		c.Questions[0].Content = "question 0 of topic 0?"
		return c, nil
	}
	// Two questions are missing, but a batch of the minimum (5) is asked for.
	got, err := newService(t, repo, gen, 0, 50).DraftTopicQuestions(context.Background(), uuid.New(), uuid.New(), topic, 2)
	if err != nil {
		t.Fatal(err)
	}
	if gen.gotNum != domain.MinQuestionsPerTopic || len(got) != domain.MinQuestionsPerTopic {
		t.Fatalf("asked for %d, got %d, want %d", gen.gotNum, len(got), domain.MinQuestionsPerTopic)
	}
	for _, question := range got {
		if question.Content == "question 0 of topic 0?" || question.TopicID == nil || *question.TopicID != topic.ID {
			t.Fatalf("a stored question was repeated or the topic is wrong: %+v", question)
		}
	}
	if len(gen.gotIn.Existing) != 2 || repo.saveCalls != 0 {
		t.Fatalf("existing=%d saves=%d, want 2 existing given to the model and nothing stored", len(gen.gotIn.Existing), repo.saveCalls)
	}
}

func TestDraftTopicQuestionsAsksForMoreWhenManyAreMissing(t *testing.T) {
	topic := roadmapTopics(1)[0]
	gen := &fakeGenerator{}
	gen.fn = func(int) (domain.GenerationCandidate, error) { return bankFor(gen.gotIn, 8), nil }
	got, err := newService(t, readyRepo(), gen, 0, 50).DraftTopicQuestions(context.Background(), uuid.New(), uuid.New(), topic, 8)
	if err != nil || gen.gotNum != 8 || len(got) != 8 {
		t.Fatalf("asked %d got %d err=%v, want 8", gen.gotNum, len(got), err)
	}
}

func TestDraftTopicQuestionsRefusesNothingToDoAndForeignProfiles(t *testing.T) {
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return domain.GenerationCandidate{}, nil }}
	topic := roadmapTopics(1)[0]
	if _, err := newService(t, readyRepo(), gen, 0, 50).DraftTopicQuestions(context.Background(), uuid.New(), uuid.New(), topic, 0); err == nil {
		t.Fatal("a top-up of zero questions was accepted")
	}
	repo := readyRepo()
	repo.ownsErr = apperror.New(apperror.CodeJDNotFound, "not found")
	_, err := newService(t, repo, gen, 0, 50).DraftTopicQuestions(context.Background(), uuid.New(), uuid.New(), topic, 3)
	assertCode(t, err, apperror.CodeJDNotFound)
	if gen.calls != 0 {
		t.Fatal("no model call without a valid request")
	}
}
