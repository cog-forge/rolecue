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
	seen := make(map[string]struct{}, len(c.Questions))
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
	return out[:size], nil
}
