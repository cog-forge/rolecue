package service

import (
	"context"
	"fmt"
	"math"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const (
	recruiterRole = "recruiter"
	// totalTolerance absorbs float rounding when the weights are summed (percentage is a double).
	totalTolerance = 1e-9
	// MaxWeightTotal bounds the configured total to a sane range.
	MaxWeightTotal = 1000
)

// Repository is the persistence contract of posting evaluation weights. It never touches the
// question bank: weights and core questions are separate by design.
type Repository interface {
	ListMetrics(ctx context.Context) ([]domain.Metric, error)
	PostingStatus(ctx context.Context, recruiterID, postingID uuid.UUID) (string, error)
	ListWeights(ctx context.Context, postingID uuid.UUID) ([]domain.Weight, error)
	ReplaceWeights(ctx context.Context, recruiterID, postingID uuid.UUID, weights []domain.Weight) ([]domain.Weight, error)
	ClearWeights(ctx context.Context, recruiterID, postingID uuid.UUID) error
}

type Service struct {
	repository Repository
	total      float64
}

// New builds the service. total is the sum the weights of a posting must reach; 0 turns the sum
// check off, because the scoring formula is still an open product decision.
func New(repository Repository, total float64) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("evaluation repository is required")
	}
	if math.IsNaN(total) || total < 0 || total > MaxWeightTotal {
		return nil, fmt.Errorf("evaluation weight total must be between 0 and %d", MaxWeightTotal)
	}
	return &Service{repository: repository, total: total}, nil
}

func requireRecruiter(role string) error {
	if role != recruiterRole {
		return apperror.New(apperror.CodeForbidden, "only recruiters can manage evaluation weights")
	}
	return nil
}

// Metrics returns the metrics catalog, so clients never hardcode the evaluation dimensions.
func (s *Service) Metrics(ctx context.Context) ([]domain.Metric, error) {
	metrics, err := s.repository.ListMetrics(ctx)
	if err != nil {
		return nil, err
	}
	if metrics == nil {
		metrics = []domain.Metric{}
	}
	return metrics, nil
}

// Weights returns the weights of a posting the Recruiter owns, or the default weights when the
// Recruiter set none.
func (s *Service) Weights(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) (domain.PostingWeights, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.PostingWeights{}, err
	}
	status, err := s.repository.PostingStatus(ctx, recruiterID, postingID)
	if err != nil {
		return domain.PostingWeights{}, err
	}
	weights, err := s.repository.ListWeights(ctx, postingID)
	if err != nil {
		return domain.PostingWeights{}, err
	}
	if len(weights) > 0 {
		return s.view(status, weights), nil
	}
	return s.defaultView(ctx, status)
}

// defaultView is the view of a posting without saved weights: the equal-split defaults, computed
// on read and never stored.
func (s *Service) defaultView(ctx context.Context, status string) (domain.PostingWeights, error) {
	metrics, err := s.repository.ListMetrics(ctx)
	if err != nil {
		return domain.PostingWeights{}, err
	}
	view := s.view(status, s.defaults(metrics))
	view.Configured, view.UsingDefaults = false, true
	return view, nil
}

// defaults splits the configured total equally across the metrics catalog (team decision,
// 2026-10-18). Without a sum rule (total 0) every metric gets the same relative weight of 1.
// Scoring (KAN-61) must use these when a posting has no saved weights.
func (s *Service) defaults(metrics []domain.Metric) []domain.Weight {
	weights := make([]domain.Weight, 0, len(metrics))
	if len(metrics) == 0 {
		return weights
	}
	each := 1.0
	if s.total > 0 {
		each = s.total / float64(len(metrics))
	}
	for _, metric := range metrics {
		weights = append(weights, domain.Weight{MetricID: metric.ID, Name: metric.Name, Percentage: each})
	}
	return weights
}

// ReplaceWeights validates the whole set and replaces the posting's weights with it in one write.
func (s *Service) ReplaceWeights(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, inputs []domain.WeightInput) (domain.PostingWeights, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.PostingWeights{}, err
	}
	status, err := s.repository.PostingStatus(ctx, recruiterID, postingID)
	if err != nil {
		return domain.PostingWeights{}, err
	}
	// Checked again under the row lock; this answers early without validating for nothing.
	if !domain.EditableStatus(status) {
		return domain.PostingWeights{}, domain.ErrWeightsLocked
	}
	metrics, err := s.repository.ListMetrics(ctx)
	if err != nil {
		return domain.PostingWeights{}, err
	}
	weights, err := s.validate(inputs, metrics)
	if err != nil {
		return domain.PostingWeights{}, err
	}
	saved, err := s.repository.ReplaceWeights(ctx, recruiterID, postingID, weights)
	if err != nil {
		return domain.PostingWeights{}, err
	}
	return s.view(status, saved), nil
}

// ResetWeights deletes the posting's saved weights, so it is scored with the equal-split defaults
// again (Decision 13a). Resetting a posting that already uses the defaults changes nothing.
func (s *Service) ResetWeights(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) (domain.PostingWeights, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.PostingWeights{}, err
	}
	status, err := s.repository.PostingStatus(ctx, recruiterID, postingID)
	if err != nil {
		return domain.PostingWeights{}, err
	}
	// Checked again under the row lock, like a replace.
	if !domain.EditableStatus(status) {
		return domain.PostingWeights{}, domain.ErrWeightsLocked
	}
	if err := s.repository.ClearWeights(ctx, recruiterID, postingID); err != nil {
		return domain.PostingWeights{}, err
	}
	return s.defaultView(ctx, status)
}

func (s *Service) view(status string, weights []domain.Weight) domain.PostingWeights {
	if weights == nil {
		weights = []domain.Weight{}
	}
	return domain.PostingWeights{
		Configured:    len(weights) > 0,
		Editable:      domain.EditableStatus(status),
		PostingStatus: status,
		Total:         s.total,
		Weights:       weights,
	}
}

// ModerationWeights exposes a read-only evaluation summary to the moderation service after it
// checks the posting exists. It uses the same effective defaults as recruitment scoring.
func (s *Service) ModerationWeights(ctx context.Context, role string, postingID uuid.UUID) (domain.PostingWeights, error) {
	if role != "admin" {
		return domain.PostingWeights{}, apperror.New(apperror.CodeForbidden, "only administrators can review evaluation weights")
	}
	weights, err := s.repository.ListWeights(ctx, postingID)
	if err != nil {
		return domain.PostingWeights{}, err
	}
	if len(weights) > 0 {
		return s.view("", weights), nil
	}
	return s.defaultView(ctx, "")
}
