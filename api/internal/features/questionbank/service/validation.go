package service

import (
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
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

// validateCandidate turns an untrusted model proposal into the questions to store: at least size
// distinct valid questions, of which the first size are kept. Every failure is an invalid-output
// error, which the retry loop is allowed to retry.
func validateCandidate(c domain.GenerationCandidate, size int) ([]string, error) {
	badOutput := func(msg string) error {
		return apperror.New(apperror.CodeInvalidQuestionGenerationOutput, msg)
	}
	if c.Questions == nil {
		return nil, badOutput("question generation returned no questions")
	}
	seen := make(map[string]struct{}, len(c.Questions))
	out := make([]string, 0, len(c.Questions))
	for _, raw := range c.Questions {
		q, err := normalizeContent(raw)
		if err != nil {
			return nil, badOutput("question generation returned an unusable question")
		}
		key := strings.ToLower(q)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, q)
	}
	if len(out) < size {
		return nil, badOutput("question generation returned too few distinct questions")
	}
	// Extra distinct questions are harmless; keep exactly size so the bank matches the config.
	return out[:size], nil
}
