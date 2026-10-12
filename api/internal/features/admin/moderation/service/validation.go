package service

import (
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func invalid(message string) error { return apperror.New(apperror.CodeValidation, message) }

func validateFilter(f domain.ListFilter) error {
	if f.Status != nil && !f.Status.Valid() {
		return invalid("invalid status filter")
	}
	if f.Limit < 1 || f.Limit > 100 || f.Offset < 0 {
		return invalid("invalid pagination parameters")
	}
	if f.From != nil && f.To != nil && !f.From.Before(*f.To) {
		return invalid("from must be earlier than to")
	}
	return nil
}

// normalizeReason validates the reject reason the Recruiter will read.
func normalizeReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if !utf8.ValidString(reason) {
		return "", invalid("reject_reason must be valid UTF-8")
	}
	// PostgreSQL text cannot store NUL; reject it here instead of failing later with a 500.
	if strings.ContainsRune(reason, 0) {
		return "", invalid("reject_reason must not contain null characters")
	}
	if n := utf8.RuneCountInString(reason); n < 1 || n > domain.MaxRejectReasonRunes {
		return "", invalid("reject_reason must contain 1 to 1000 characters")
	}
	return reason, nil
}
