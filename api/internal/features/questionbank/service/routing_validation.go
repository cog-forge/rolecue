package service

import "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"

// ValidateGenerated applies the storage validator before a routing attempt is accepted.
func ValidateGenerated(c domain.GenerationCandidate, size int) ([]string, error) {
	return validateCandidate(c, size)
}
