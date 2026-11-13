package service

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/textsafe"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func invalid(message string) error { return apperror.New(apperror.CodeValidation, message) }

// normalizeContent validates a question written by a Recruiter.
func normalizeContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if !utf8.ValidString(content) {
		return "", invalid("content must be valid UTF-8")
	}
	// PostgreSQL text cannot store NUL; reject it here instead of failing later with a 500.
	if strings.ContainsRune(content, 0) {
		return "", invalid("content must not contain null characters")
	}
	if n := utf8.RuneCountInString(content); n < 1 || n > domain.MaxQuestionRunes {
		return "", invalid("content must contain 1 to 1000 characters")
	}
	return content, nil
}

// normalizeExpectedAnswer validates and cleans a model-written expected answer. Out-of-bounds text is an
// error; text that looks like a prompt injection is not stored (cleared: false, answer nil) so the question
// stays flagged as needing grading configuration instead of carrying text that will later be read by the
// grader.
func normalizeExpectedAnswer(raw string) (answer *string, err error) {
	if !utf8.ValidString(raw) {
		return nil, invalid("expected answer must be valid UTF-8")
	}
	if strings.ContainsRune(raw, 0) {
		return nil, invalid("expected answer must not contain null characters")
	}
	text := textsafe.SanitizeOutput(raw)
	n := utf8.RuneCountInString(text)
	if n < 1 {
		return nil, invalid("expected answer is empty")
	}
	if n > domain.MaxExpectedAnswerRunes || len(strings.Fields(text)) > domain.MaxExpectedAnswerWords {
		return nil, invalid("expected answer is too long")
	}
	if textsafe.LooksLikeInjection(text) {
		return nil, nil
	}
	return &text, nil
}

func normalizeQuestionPatch(p domain.QuestionPatch) (domain.QuestionPatch, error) {
	if p.Content == nil && !p.SetRubric && !p.SetExpectedAnswer {
		return p, invalid("at least one field is required")
	}
	var err error
	if p.Content != nil {
		text, e := normalizeContent(*p.Content)
		if e != nil {
			return p, e
		}
		p.Content = &text
	}
	if p.SetExpectedAnswer && p.ExpectedAnswer != nil {
		p.ExpectedAnswer, err = normalizeExpectedAnswer(*p.ExpectedAnswer)
		if err != nil {
			return p, err
		}
		if p.ExpectedAnswer == nil {
			return p, invalid("expected answer contains unsafe instructions")
		}
	}
	if p.PreviousRevision != "" && len(p.PreviousRevision) != 64 {
		return p, invalid("invalid previous revision")
	}
	return p, nil
}

// validateCandidate turns an untrusted model proposal into the questions to store: at least size
// distinct valid questions, of which the first size are kept. In grading mode every question also needs a
// rubric id taken from the allowed set (or none) and an expected answer within bounds. Every failure is an
// invalid-output error, which the retry loop is allowed to retry.
func validateCandidate(in domain.GenerationInput, c domain.GenerationCandidate, size int) ([]domain.GeneratedQuestion, error) {
	// The cause names the rule and counts only; question text is never placed in an error.
	badOutput := func(msg string, cause error) error {
		return apperror.Wrap(apperror.CodeInvalidQuestionGenerationOutput, msg, cause)
	}
	if c.Questions == nil {
		return nil, badOutput("question generation returned no questions", errors.New("questions array is missing"))
	}
	allowed := make(map[uuid.UUID]struct{}, len(in.Rubrics))
	for _, rubric := range in.Rubrics {
		allowed[rubric.ID] = struct{}{}
	}
	topics := make(map[uuid.UUID]struct{}, len(in.Topics))
	for _, topic := range in.Topics {
		topics[topic.ID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(c.Questions)+len(in.Existing))
	for _, existing := range in.Existing {
		seen[strings.ToLower(strings.TrimSpace(existing))] = struct{}{}
	}
	out := make([]domain.GeneratedQuestion, 0, len(c.Questions))
	for index, raw := range c.Questions {
		q, err := normalizeContent(raw.Content)
		if err != nil {
			return nil, badOutput("question generation returned an unusable question", fmt.Errorf("question %d: %w", index+1, err))
		}
		key := strings.ToLower(q)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		item := domain.GeneratedQuestion{Content: q}
		if len(topics) > 0 {
			// Every question of a Roadmap bank names exactly one of the topics the backend allowed.
			if raw.TopicID == nil {
				return nil, badOutput("question generation did not map a question to a topic", fmt.Errorf("question %d: topic id is missing", index+1))
			}
			if _, ok := topics[*raw.TopicID]; !ok {
				return nil, badOutput("question generation chose a topic that is not allowed", fmt.Errorf("question %d: topic id not in the allowed set", index+1))
			}
			id := *raw.TopicID
			item.TopicID = &id
		}
		if in.Grading {
			if raw.RubricID != nil {
				if _, ok := allowed[*raw.RubricID]; !ok {
					return nil, badOutput("question generation chose a rubric that is not allowed", fmt.Errorf("question %d: rubric id not in the allowed set", index+1))
				}
				id := *raw.RubricID
				item.RubricID = &id
			}
			if raw.ExpectedAnswer != nil {
				item.ExpectedAnswer, err = normalizeExpectedAnswer(*raw.ExpectedAnswer)
				if err != nil {
					return nil, badOutput("question generation returned an unusable expected answer", fmt.Errorf("question %d: %w", index+1, err))
				}
			}
		}
		out = append(out, item)
	}
	if len(out) < size {
		return nil, badOutput("question generation returned too few distinct questions",
			fmt.Errorf("%d distinct questions, %d required", len(out), size))
	}
	// Extra distinct questions are harmless; keep exactly size so the bank matches the config.
	if len(topics) > 0 {
		return trimKeepingTopics(out, size), nil
	}
	return out[:size], nil
}

// trimKeepingTopics keeps size of the questions without starving a topic: the first MinQuestionsPerTopic
// questions of each topic rank first, the rest follow in the order the model wrote them, and the kept ones
// stay in their original order. A model that front-loads one topic therefore cannot push another below the
// distribution rule just because the proposal was longer than the bank.
func trimKeepingTopics(questions []domain.GeneratedQuestion, size int) []domain.GeneratedQuestion {
	if len(questions) <= size {
		return questions
	}
	taken := make(map[uuid.UUID]int)
	priority, rest := make([]int, 0, size), make([]int, 0, len(questions))
	for i, question := range questions {
		if question.TopicID != nil && taken[*question.TopicID] < domain.MinQuestionsPerTopic {
			taken[*question.TopicID]++
			priority = append(priority, i)
			continue
		}
		rest = append(rest, i)
	}
	keep := make(map[int]bool, size)
	for _, i := range append(priority, rest...) {
		if len(keep) == size {
			break
		}
		keep[i] = true
	}
	out := make([]domain.GeneratedQuestion, 0, size)
	for i, question := range questions {
		if keep[i] {
			out = append(out, question)
		}
	}
	return out
}

// ValidateBank checks a complete proposal against the distribution rule of a Roadmap bank: every topic has at
// least MinQuestionsPerTopic distinct questions. It is separate from validateCandidate because a batch of a
// larger bank is validated before the others are written. A bank without topics has no such rule.
func ValidateBank(in domain.GenerationInput, questions []domain.GeneratedQuestion) error {
	if len(in.Topics) == 0 {
		return nil
	}
	counts := domain.CountByTopic(questions)
	for index, topic := range in.Topics {
		if counts[topic.ID] < domain.MinQuestionsPerTopic {
			return apperror.Wrap(apperror.CodeInvalidQuestionGenerationOutput, "question generation did not cover every topic",
				fmt.Errorf("topic %d has %d distinct questions, at least %d required", index+1, counts[topic.ID], domain.MinQuestionsPerTopic))
		}
	}
	return nil
}

// FitBatchToTopics checks one batch of a Roadmap bank against the quotas the batch was asked for and keeps
// exactly each topic's quota of its questions, in the order the model wrote them. A batch that leaves a topic
// short is invalid output, so the routed call retries it or falls back to the next provider while only that
// batch is lost; extra questions of a topic are dropped. A question of a topic that was not asked for in this
// batch is dropped too (it only counts against the final size).
func FitBatchToTopics(questions []domain.GeneratedQuestion, quotas []domain.RoadmapTopic) ([]domain.GeneratedQuestion, error) {
	want := make(map[uuid.UUID]int, len(quotas))
	for _, topic := range quotas {
		want[topic.ID] = topic.Quota
	}
	taken := make(map[uuid.UUID]int, len(quotas))
	out := make([]domain.GeneratedQuestion, 0, len(questions))
	for _, question := range questions {
		if question.TopicID == nil || taken[*question.TopicID] >= want[*question.TopicID] {
			continue
		}
		taken[*question.TopicID]++
		out = append(out, question)
	}
	for index, topic := range quotas {
		if taken[topic.ID] < topic.Quota {
			return nil, apperror.Wrap(apperror.CodeInvalidQuestionGenerationOutput, "question generation did not write the questions asked for a topic",
				fmt.Errorf("topic %d of the batch has %d questions, %d asked for", index+1, taken[topic.ID], topic.Quota))
		}
	}
	return out, nil
}
