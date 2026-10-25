package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// maxAnswerLength bounds one spoken answer, as text, in characters.
const maxAnswerLength = 8000

// normalizeAnswer trims the answer and checks it is valid UTF-8 of 1 to maxAnswerLength characters. The
// trimmed text is what is stored and compared on a retry.
func normalizeAnswer(answer string) (string, error) {
	if !utf8.ValidString(answer) {
		return "", apperror.New(apperror.CodeValidation, "answer must be valid UTF-8")
	}
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return "", apperror.New(apperror.CodeValidation, "answer is required")
	}
	if utf8.RuneCountInString(trimmed) > maxAnswerLength {
		return "", apperror.New(apperror.CodeValidation, fmt.Sprintf("answer must be at most %d characters", maxAnswerLength))
	}
	return trimmed, nil
}
