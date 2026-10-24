package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

var (
	accuracy = domain.Metric{ID: uuid.MustParse("00000000-0000-4000-8000-000000000101"), Name: "Technical Accuracy"}
	depth    = domain.Metric{ID: uuid.MustParse("00000000-0000-4000-8000-000000000102"), Name: "Depth of Understanding"}
	clarity  = domain.Metric{ID: uuid.MustParse("00000000-0000-4000-8000-000000000105"), Name: "Communication Clarity"}
)

// fakeRepository implements only the weights contract. It has no question-bank method, so the
// service cannot reach core_questions through it.
type fakeRepository struct {
	metrics    []domain.Metric
	status     string
	weights    []domain.Weight
	statusErr  error
	replaceErr error
	clearErr   error

	replaced     []domain.Weight
	replaceCalls int
	clearCalls   int
	recruiterID  uuid.UUID
	postingID    uuid.UUID
}

func (f *fakeRepository) ListMetrics(context.Context) ([]domain.Metric, error) {
	return f.metrics, nil
}

func (f *fakeRepository) PostingStatus(_ context.Context, recruiterID, postingID uuid.UUID) (string, error) {
	f.recruiterID, f.postingID = recruiterID, postingID
	return f.status, f.statusErr
}

func (f *fakeRepository) ListWeights(context.Context, uuid.UUID) ([]domain.Weight, error) {
	return f.weights, nil
}

func (f *fakeRepository) ReplaceWeights(_ context.Context, _, _ uuid.UUID, weights []domain.Weight) ([]domain.Weight, error) {
	f.replaceCalls++
	if f.replaceErr != nil {
		return nil, f.replaceErr
	}
	f.replaced = weights
	return weights, nil
}

func (f *fakeRepository) ClearWeights(_ context.Context, recruiterID, postingID uuid.UUID) error {
	f.clearCalls++
	if f.clearErr != nil {
		return f.clearErr
	}
	f.recruiterID, f.postingID = recruiterID, postingID
	f.weights = nil
	return nil
}

func newService(t *testing.T, repo *fakeRepository, total float64) *Service {
	t.Helper()
	svc, err := New(repo, total)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return svc
}

func entry(metric domain.Metric, percentage float64) domain.WeightInput {
	id := metric.ID
	return domain.WeightInput{MetricID: &id, Percentage: &percentage}
}

func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != want {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}
