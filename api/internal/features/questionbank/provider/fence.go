package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/textsafe"
)

const (
	// maxNoteRunes and maxEvidenceRunes cap the job-description text that goes into the prompt.
	maxNoteRunes     = 2000
	maxEvidenceRunes = 4000
)

// fencedRequirements copies the requirements with each evidence_text, a JD excerpt written by a person,
// cleaned, capped and fenced as untrusted data (KAN-300). It never returns nil.
func fencedRequirements(in []domain.ConfirmedRequirement) []domain.ConfirmedRequirement {
	out := make([]domain.ConfirmedRequirement, 0, len(in))
	for _, requirement := range in {
		if requirement.EvidenceText != nil {
			fenced := textsafe.Fence("UNTRUSTED_JD_EVIDENCE", *requirement.EvidenceText, maxEvidenceRunes)
			requirement.EvidenceText = &fenced
		}
		out = append(out, requirement)
	}
	return out
}
