package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestModerationWeightsAreReadOnlyAndUseRuntimeDefaults(t *testing.T) {
	repo := &fakeRepository{metrics: []domain.Metric{accuracy, depth}}
	svc := newService(t, repo, 80)
	view, err := svc.ModerationWeights(context.Background(), "admin", uuid.New())
	if err != nil || !view.UsingDefaults || view.Editable || len(view.Weights) != 2 || view.Weights[0].Percentage != 40 {
		t.Fatalf("unexpected default review: %+v %v", view, err)
	}
	repo.weights = []domain.Weight{{MetricID: accuracy.ID, Name: accuracy.Name, Percentage: 80}}
	view, err = svc.ModerationWeights(context.Background(), "admin", uuid.New())
	if err != nil || !view.Configured || view.UsingDefaults || view.Editable || len(view.Weights) != 1 {
		t.Fatalf("unexpected custom review: %+v %v", view, err)
	}
	for _, role := range []string{"recruiter", "candidate", ""} {
		_, err = svc.ModerationWeights(context.Background(), role, uuid.New())
		assertCode(t, err, apperror.CodeForbidden)
	}
	if repo.replaceCalls != 0 || repo.clearCalls != 0 {
		t.Fatal("review wrote weights")
	}
}
