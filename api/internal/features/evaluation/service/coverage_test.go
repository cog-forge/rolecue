package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/google/uuid"
)

// fakeCoverage is the bank-coverage reader of the weight views (KAN-377).
type fakeCoverage struct {
	report domain.CoverageReport
	err    error
	calls  int
	gotFor uuid.UUID
}

func (f *fakeCoverage) Coverage(_ context.Context, postingID uuid.UUID) (domain.CoverageReport, error) {
	f.calls++
	f.gotFor = postingID
	return f.report, f.err
}

func TestWeightViewsCarryTheBankCoverage(t *testing.T) {
	repo := &fakeRepository{metrics: catalog, status: "pending"}
	coverage := &fakeCoverage{report: domain.CoverageReport{Passes: false, Assessed: true, Covered: 2, Total: 3, QuestionLimit: 8}}
	svc := newService(t, repo, 100)
	svc.SetCoverageReader(coverage)
	ctx := context.Background()

	view, err := svc.Weights(ctx, recruiterID, "recruiter", postingID)
	if err != nil || view.Coverage == nil || view.Coverage.Covered != 2 || coverage.gotFor != postingID {
		t.Fatalf("Weights() coverage = %+v for %s, err %v; want the report for the posting", view.Coverage, coverage.gotFor, err)
	}
	replaced, err := svc.ReplaceWeights(ctx, recruiterID, "recruiter", postingID,
		[]domain.WeightInput{entry(accuracy, 40), entry(depth, 30), entry(clarity, 30)})
	if err != nil || replaced.Coverage == nil || replaced.Coverage.Total != 3 {
		t.Fatalf("ReplaceWeights() coverage = %+v, err %v", replaced.Coverage, err)
	}
	reset, err := svc.ResetWeights(ctx, recruiterID, "recruiter", postingID)
	if err != nil || reset.Coverage == nil {
		t.Fatalf("ResetWeights() coverage = %+v, err %v", reset.Coverage, err)
	}
	if coverage.calls != 3 {
		t.Fatalf("coverage read %d times, want once per view", coverage.calls)
	}
}

func TestWeightViewsWorkWithoutACoverageReader(t *testing.T) {
	repo := &fakeRepository{metrics: catalog, status: "pending"}
	view, err := newService(t, repo, 100).Weights(context.Background(), recruiterID, "recruiter", postingID)
	if err != nil || view.Coverage != nil {
		t.Fatalf("Weights() = coverage %+v, err %v; want a view without coverage", view.Coverage, err)
	}
}

func TestAFailedCoverageReadNeverFailsTheWeightChange(t *testing.T) {
	repo := &fakeRepository{metrics: catalog, status: "pending"}
	svc := newService(t, repo, 100)
	svc.SetCoverageReader(&fakeCoverage{err: errors.New("db down")})
	view, err := svc.ReplaceWeights(context.Background(), recruiterID, "recruiter", postingID,
		[]domain.WeightInput{entry(accuracy, 40), entry(depth, 30), entry(clarity, 30)})
	if err != nil || view.Coverage != nil || repo.replaceCalls != 1 {
		t.Fatalf("ReplaceWeights() = coverage %+v, err %v, saves %d; want the saved weights without coverage", view.Coverage, err, repo.replaceCalls)
	}
}

func TestOnlyTheRecruitersViewsCarryCoverage(t *testing.T) {
	repo := &fakeRepository{metrics: catalog, status: "pending"}
	coverage := &fakeCoverage{}
	svc := newService(t, repo, 100)
	svc.SetCoverageReader(coverage)
	view, err := svc.ModerationWeights(context.Background(), "admin", postingID)
	if err != nil || view.Coverage != nil || coverage.calls != 0 {
		t.Fatalf("ModerationWeights() coverage = %+v after %d reads, err %v; the Admin dossier carries its own coverage", view.Coverage, coverage.calls, err)
	}
}

func TestWeightChangesReportAWithdrawnSubmission(t *testing.T) {
	ctx := context.Background()
	inputs := []domain.WeightInput{entry(accuracy, 40), entry(depth, 30), entry(clarity, 30)}
	for _, withdraws := range []bool{true, false} {
		repo := &fakeRepository{metrics: catalog, status: "pending", withdraws: withdraws}
		svc := newService(t, repo, 100)
		replaced, err := svc.ReplaceWeights(ctx, recruiterID, "recruiter", postingID, inputs)
		if err != nil || replaced.SubmissionWithdrawn != withdraws {
			t.Fatalf("ReplaceWeights() withdrawn = %v, err %v; want %v", replaced.SubmissionWithdrawn, err, withdraws)
		}
		reset, err := svc.ResetWeights(ctx, recruiterID, "recruiter", postingID)
		if err != nil || reset.SubmissionWithdrawn != withdraws {
			t.Fatalf("ResetWeights() withdrawn = %v, err %v; want %v", reset.SubmissionWithdrawn, err, withdraws)
		}
		read, err := svc.Weights(ctx, recruiterID, "recruiter", postingID)
		if err != nil || read.SubmissionWithdrawn {
			t.Fatalf("Weights() withdrawn = %v, err %v; a read never reports a withdrawal", read.SubmissionWithdrawn, err)
		}
	}
}
