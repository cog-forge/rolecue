package service

import (
	"context"
	"math"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

var (
	recruiterID = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	postingID   = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	catalog     = []domain.Metric{accuracy, depth, clarity}
)

func TestNewRejectsBadTotals(t *testing.T) {
	for _, total := range []float64{-1, math.NaN(), MaxWeightTotal + 1} {
		if _, err := New(&fakeRepository{}, total); err == nil {
			t.Fatalf("New(total=%v) error = nil, want an error", total)
		}
	}
	if _, err := New(nil, 100); err == nil {
		t.Fatal("New(nil repository) error = nil, want an error")
	}
}

func TestMetricsReturnsEmptyListNotNil(t *testing.T) {
	got, err := newService(t, &fakeRepository{}, 100).Metrics(context.Background())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("Metrics() = %v, %v, want an empty non-nil list", got, err)
	}
}

func TestOnlyRecruitersManageWeights(t *testing.T) {
	for _, role := range []string{"candidate", "admin", ""} {
		repo := &fakeRepository{metrics: catalog, status: "pending"}
		svc := newService(t, repo, 100)
		_, err := svc.Weights(context.Background(), recruiterID, role, postingID)
		assertCode(t, err, apperror.CodeForbidden)
		_, err = svc.ReplaceWeights(context.Background(), recruiterID, role, postingID, []domain.WeightInput{entry(accuracy, 100)})
		assertCode(t, err, apperror.CodeForbidden)
		_, err = svc.ResetWeights(context.Background(), recruiterID, role, postingID)
		assertCode(t, err, apperror.CodeForbidden)
		if repo.replaceCalls != 0 || repo.clearCalls != 0 {
			t.Fatalf("role %q wrote weights", role)
		}
	}
}

func TestWeightsNotConfiguredReturnsEqualDefaults(t *testing.T) {
	repo := &fakeRepository{status: "open", metrics: []domain.Metric{accuracy, depth, clarity, {ID: uuid.New(), Name: "Answer Relevance"}}}
	got, err := newService(t, repo, 100).Weights(context.Background(), recruiterID, "recruiter", postingID)
	if err != nil {
		t.Fatalf("Weights() error = %v", err)
	}
	if got.Configured || !got.UsingDefaults || got.Editable || got.PostingStatus != "open" || got.Total != 100 || len(got.Weights) != 4 {
		t.Fatalf("Weights() = %+v, want not configured, using defaults, not editable, 4 weights", got)
	}
	for _, weight := range got.Weights {
		if weight.Percentage != 25 || weight.Name == "" {
			t.Fatalf("default weight = %+v, want 25 (100 split across 4 metrics) with its name", weight)
		}
	}
	if repo.recruiterID != recruiterID || repo.postingID != postingID {
		t.Fatal("the posting lookup was not scoped to the recruiter")
	}
	if repo.replaceCalls != 0 {
		t.Fatal("default weights were saved")
	}
}

func TestDefaultsWithoutSumRuleOrMetrics(t *testing.T) {
	got, err := newService(t, &fakeRepository{status: "pending", metrics: catalog}, 0).
		Weights(context.Background(), recruiterID, "recruiter", postingID)
	if err != nil || !got.UsingDefaults || len(got.Weights) != 3 || got.Weights[0].Percentage != 1 {
		t.Fatalf("Weights() with total 0 = %+v, %v, want equal relative weights of 1", got, err)
	}
	got, err = newService(t, &fakeRepository{status: "pending"}, 100).
		Weights(context.Background(), recruiterID, "recruiter", postingID)
	if err != nil || !got.UsingDefaults || got.Weights == nil || len(got.Weights) != 0 {
		t.Fatalf("Weights() with an empty catalog = %+v, %v, want an empty non-nil list", got, err)
	}
}

func TestWeightsConfigured(t *testing.T) {
	saved := []domain.Weight{{MetricID: accuracy.ID, Name: accuracy.Name, Percentage: 100}}
	got, err := newService(t, &fakeRepository{status: "pending", weights: saved}, 100).
		Weights(context.Background(), recruiterID, "recruiter", postingID)
	if err != nil || !got.Configured || got.UsingDefaults || !got.Editable || len(got.Weights) != 1 {
		t.Fatalf("Weights() = %+v, %v, want a configured, editable set", got, err)
	}
}

func TestNotFoundPassesThrough(t *testing.T) {
	notFound := apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")
	repo := &fakeRepository{statusErr: notFound, metrics: catalog}
	svc := newService(t, repo, 100)
	_, err := svc.Weights(context.Background(), recruiterID, "recruiter", postingID)
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	_, err = svc.ReplaceWeights(context.Background(), recruiterID, "recruiter", postingID, []domain.WeightInput{entry(accuracy, 100)})
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	_, err = svc.ResetWeights(context.Background(), recruiterID, "recruiter", postingID)
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	if repo.clearCalls != 0 {
		t.Fatal("weights of a posting that was not found were cleared")
	}
}

func TestReplaceSavesTheWholeSet(t *testing.T) {
	repo := &fakeRepository{metrics: catalog, status: "pending"}
	got, err := newService(t, repo, 100).ReplaceWeights(context.Background(), recruiterID, "recruiter", postingID,
		[]domain.WeightInput{entry(accuracy, 33.3), entry(depth, 33.3), entry(clarity, 33.4)})
	if err != nil {
		t.Fatalf("ReplaceWeights() error = %v", err)
	}
	if repo.replaceCalls != 1 || len(repo.replaced) != 3 || repo.replaced[0].Name != accuracy.Name {
		t.Fatalf("replaced = %+v, want the three weights with their names", repo.replaced)
	}
	if !got.Configured || !got.Editable || len(got.Weights) != 3 {
		t.Fatalf("ReplaceWeights() = %+v, want the saved set", got)
	}
}

func TestReplaceAllowedWhileRejected(t *testing.T) {
	repo := &fakeRepository{metrics: catalog, status: "rejected"}
	got, err := newService(t, repo, 100).ReplaceWeights(context.Background(), recruiterID, "recruiter", postingID,
		[]domain.WeightInput{entry(accuracy, 100)})
	if err != nil || repo.replaceCalls != 1 || !got.Editable || got.PostingStatus != "rejected" {
		t.Fatalf("ReplaceWeights() on a rejected posting = %+v, %v, want it saved and still editable", got, err)
	}
	if !domain.EditableStatus("pending") || !domain.EditableStatus("rejected") {
		t.Fatal("EditableStatus must allow pending and rejected")
	}
}

func TestReplaceLockedOutsideTheEditWindow(t *testing.T) {
	for _, status := range []string{"open", "intake_closed", "closed"} {
		repo := &fakeRepository{metrics: catalog, status: status}
		_, err := newService(t, repo, 100).ReplaceWeights(context.Background(), recruiterID, "recruiter", postingID,
			[]domain.WeightInput{entry(accuracy, 100)})
		assertCode(t, err, apperror.CodeEvaluationWeightsLocked)
		if repo.replaceCalls != 0 {
			t.Fatalf("status %s: weights were written", status)
		}
		if domain.EditableStatus(status) {
			t.Fatalf("EditableStatus(%s) = true, want false", status)
		}
	}
}

func TestReplaceLockedUnderTheRowLock(t *testing.T) {
	repo := &fakeRepository{metrics: catalog, status: "pending", replaceErr: domain.ErrWeightsLocked}
	_, err := newService(t, repo, 100).ReplaceWeights(context.Background(), recruiterID, "recruiter", postingID,
		[]domain.WeightInput{entry(accuracy, 100)})
	assertCode(t, err, apperror.CodeEvaluationWeightsLocked)
}

func TestReplaceValidation(t *testing.T) {
	unknownID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	missingPercentage := domain.WeightInput{MetricID: &accuracy.ID}
	for _, tc := range []struct {
		name   string
		inputs []domain.WeightInput
		want   apperror.Code
	}{
		{"empty", nil, apperror.CodeValidation},
		{"missing percentage", []domain.WeightInput{missingPercentage}, apperror.CodeValidation},
		{"missing metric", []domain.WeightInput{{Percentage: new(float64)}}, apperror.CodeValidation},
		{"unknown metric", []domain.WeightInput{entry(domain.Metric{ID: unknownID}, 100)}, apperror.CodeEvaluationMetricUnknown},
		{"duplicate metric", []domain.WeightInput{entry(accuracy, 50), entry(accuracy, 50)}, apperror.CodeEvaluationMetricDuplicate},
		{"negative", []domain.WeightInput{entry(accuracy, -10), entry(depth, 110)}, apperror.CodeEvaluationWeightInvalid},
		{"not a number", []domain.WeightInput{entry(accuracy, math.NaN())}, apperror.CodeEvaluationWeightInvalid},
		{"infinite", []domain.WeightInput{entry(accuracy, math.Inf(1))}, apperror.CodeEvaluationWeightInvalid},
		{"above the total", []domain.WeightInput{entry(accuracy, 150)}, apperror.CodeEvaluationWeightInvalid},
		{"sum too low", []domain.WeightInput{entry(accuracy, 40), entry(depth, 40)}, apperror.CodeEvaluationWeightTotalMismatch},
		{"sum too high", []domain.WeightInput{entry(accuracy, 60), entry(depth, 60)}, apperror.CodeEvaluationWeightTotalMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{metrics: catalog, status: "pending"}
			_, err := newService(t, repo, 100).ReplaceWeights(context.Background(), recruiterID, "recruiter", postingID, tc.inputs)
			assertCode(t, err, tc.want)
			if repo.replaceCalls != 0 {
				t.Fatal("an invalid set was written")
			}
		})
	}
}

func TestZeroTotalTurnsTheSumCheckOff(t *testing.T) {
	repo := &fakeRepository{metrics: catalog, status: "pending"}
	svc := newService(t, repo, 0)
	got, err := svc.ReplaceWeights(context.Background(), recruiterID, "recruiter", postingID,
		[]domain.WeightInput{entry(accuracy, 7), entry(depth, 250)})
	if err != nil || len(got.Weights) != 2 || got.Total != 0 {
		t.Fatalf("ReplaceWeights() = %+v, %v, want any non-negative weights accepted", got, err)
	}
	_, err = svc.ReplaceWeights(context.Background(), recruiterID, "recruiter", postingID, []domain.WeightInput{entry(accuracy, -1)})
	assertCode(t, err, apperror.CodeEvaluationWeightInvalid)
	_, err = svc.ReplaceWeights(context.Background(), recruiterID, "recruiter", postingID, []domain.WeightInput{entry(accuracy, 1e308)})
	assertCode(t, err, apperror.CodeEvaluationWeightInvalid)
}

func TestTotalComesFromConfiguration(t *testing.T) {
	repo := &fakeRepository{metrics: catalog, status: "pending"}
	if _, err := newService(t, repo, 1).ReplaceWeights(context.Background(), recruiterID, "recruiter", postingID,
		[]domain.WeightInput{entry(accuracy, 0.25), entry(depth, 0.75)}); err != nil {
		t.Fatalf("ReplaceWeights() with total 1 error = %v", err)
	}
}

func TestResetReturnsTheEqualSplitDefaults(t *testing.T) {
	for _, status := range []string{"pending", "rejected"} {
		saved := []domain.Weight{{MetricID: accuracy.ID, Name: accuracy.Name, Percentage: 100}}
		repo := &fakeRepository{metrics: []domain.Metric{accuracy, depth, clarity, {ID: uuid.New(), Name: "Answer Relevance"}}, status: status, weights: saved}
		got, err := newService(t, repo, 100).ResetWeights(context.Background(), recruiterID, "recruiter", postingID)
		if err != nil {
			t.Fatalf("status %s: ResetWeights() error = %v", status, err)
		}
		if repo.clearCalls != 1 || repo.recruiterID != recruiterID || repo.postingID != postingID {
			t.Fatalf("status %s: clear calls = %d for %s/%s, want one scoped to the recruiter's posting", status, repo.clearCalls, repo.recruiterID, repo.postingID)
		}
		if got.Configured || !got.UsingDefaults || !got.Editable || got.PostingStatus != status || len(got.Weights) != 4 {
			t.Fatalf("status %s: ResetWeights() = %+v, want the editable default view", status, got)
		}
		for _, weight := range got.Weights {
			if weight.Percentage != 25 {
				t.Fatalf("status %s: default weight = %+v, want 25 (100 split across 4 metrics)", status, weight)
			}
		}
		if repo.replaceCalls != 0 {
			t.Fatalf("status %s: the defaults were saved", status)
		}
	}
}

func TestResetLockedOutsideTheEditWindow(t *testing.T) {
	for _, status := range []string{"open", "intake_closed", "closed"} {
		repo := &fakeRepository{metrics: catalog, status: status}
		_, err := newService(t, repo, 100).ResetWeights(context.Background(), recruiterID, "recruiter", postingID)
		assertCode(t, err, apperror.CodeEvaluationWeightsLocked)
		if repo.clearCalls != 0 {
			t.Fatalf("status %s: weights were cleared", status)
		}
	}
}

func TestResetLockedUnderTheRowLock(t *testing.T) {
	repo := &fakeRepository{metrics: catalog, status: "pending", clearErr: domain.ErrWeightsLocked}
	_, err := newService(t, repo, 100).ResetWeights(context.Background(), recruiterID, "recruiter", postingID)
	assertCode(t, err, apperror.CodeEvaluationWeightsLocked)
}
