package service

import "github.com/cog-forge/rolecue/api/internal/features/interview/domain"

// ValidateFollowUp applies the existing live validator before accepting a provider attempt.
func ValidateFollowUp(p *domain.FollowUpProposal, in domain.FollowUpContext) error {
	return validateProposal(p, in)
}
