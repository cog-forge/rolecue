package service

import "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"

// ValidateScores applies the persistence validator before accepting a provider attempt.
func ValidateScores(in domain.ScoreInput, out domain.ScoreOutput) error {
	_, err := buildEvaluation(in, out)
	return err
}
