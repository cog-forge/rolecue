package service

import (
	"fmt"
	"math"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// validate checks the whole set before anything is written, so a bad entry never leaves a partial
// set behind.
func (s *Service) validate(inputs []domain.WeightInput, metrics []domain.Metric) ([]domain.Weight, error) {
	if len(inputs) == 0 {
		return nil, apperror.New(apperror.CodeValidation, "at least one evaluation weight is required")
	}
	names := make(map[uuid.UUID]string, len(metrics))
	for _, metric := range metrics {
		names[metric.ID] = metric.Name
	}
	seen := make(map[uuid.UUID]bool, len(inputs))
	weights := make([]domain.Weight, 0, len(inputs))
	sum := 0.0
	for _, input := range inputs {
		if input.MetricID == nil || input.Percentage == nil {
			return nil, apperror.New(apperror.CodeValidation, "each weight needs a metric_id and a percentage")
		}
		id, percentage := *input.MetricID, *input.Percentage
		if seen[id] {
			return nil, apperror.New(apperror.CodeEvaluationMetricDuplicate, "each metric can be weighted only once")
		}
		seen[id] = true
		name, ok := names[id]
		if !ok {
			return nil, apperror.New(apperror.CodeEvaluationMetricUnknown, "unknown evaluation metric")
		}
		if math.IsNaN(percentage) || math.IsInf(percentage, 0) || percentage < 0 {
			return nil, apperror.New(apperror.CodeEvaluationWeightInvalid, "a weight must be a number that is not negative")
		}
		// Without a sum rule a single weight is still capped, so absurd values are never stored.
		limit := s.total
		if limit == 0 {
			limit = MaxWeightTotal
		}
		if percentage > limit {
			return nil, apperror.New(apperror.CodeEvaluationWeightInvalid, fmt.Sprintf("a weight must be at most %g", limit))
		}
		sum += percentage
		weights = append(weights, domain.Weight{MetricID: id, Name: name, Percentage: percentage})
	}
	if s.total > 0 && math.Abs(sum-s.total) > totalTolerance {
		return nil, apperror.New(apperror.CodeEvaluationWeightTotalMismatch, fmt.Sprintf("the weights must add up to %g", s.total))
	}
	return weights, nil
}
