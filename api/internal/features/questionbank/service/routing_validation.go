package service

import "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"

// ValidateGenerated applies the storage validator before a routing attempt is accepted. The input carries
// the allowed rubric set, so a routed attempt is checked against the same rules as the stored result.
func ValidateGenerated(in domain.GenerationInput, c domain.GenerationCandidate, size int) ([]domain.GeneratedQuestion, error) {
	return validateCandidate(in, c, size)
}
