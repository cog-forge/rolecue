package service

import (
	"context"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// decisionFake adds the final decision to the shared fake. It records the call and returns the
// configured error; the detail read that follows goes through the embedded fake.
type decisionFake struct {
	fakeRepository
	decideCalls int
	decision    domain.Decision
	reason      *string
	decideErr   error
}

func (f *decisionFake) DecideApplication(_ context.Context, _, _, _ uuid.UUID, decision domain.Decision, reason *string) error {
	f.decideCalls++
	f.decision = decision
	f.reason = reason
	return f.decideErr
}

func interviewedDetail() domain.DetailRecord {
	record := detailRecord(nil)
	record.Item.Status = domain.StatusInterviewed
	record.Item.InterviewEligible = false
	opens := time.Now().Add(-time.Hour)
	record.OpensAt = &opens
	score := int32(81)
	record.Interview = &domain.InterviewSummary{
		ID: uuid.New(), StartedAt: time.Now().Add(-2 * time.Hour), Status: "completed", Score: &score, RecordStatus: "none",
	}
	return record
}

// appErrorCode returns the apperror code of err, or an empty code when err is not an AppError.
func appErrorCode(err error) apperror.Code {
	if appErr, ok := err.(*apperror.AppError); ok {
		return appErr.Code
	}
	return ""
}

func TestDecideIsRecruiterOnlyAndValidatesTheDecision(t *testing.T) {
	repo := &decisionFake{}
	svc, err := New(repo, &fakeCVLinker{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(context.Background(), candidateID, "candidate", postingID, applicationID, domain.DecisionApprove, nil); appErrorCode(err) != apperror.CodeForbidden {
		t.Fatalf("candidate got %v, want FORBIDDEN", err)
	}
	for _, bad := range []domain.Decision{"", "maybe"} {
		if _, err := svc.Decide(context.Background(), recruiterID, "recruiter", postingID, applicationID, bad, nil); appErrorCode(err) != apperror.CodeValidation {
			t.Fatalf("decision %q got %v, want VALIDATION_ERROR", bad, err)
		}
	}
	tooLong := string(make([]byte, domain.MaxRejectReasonLength+1))
	if _, err := svc.Decide(context.Background(), recruiterID, "recruiter", postingID, applicationID, domain.DecisionReject, &tooLong); appErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("long reason got %v, want VALIDATION_ERROR", err)
	}
	if repo.decideCalls != 0 {
		t.Fatalf("invalid input reached the repository %d times", repo.decideCalls)
	}
}

func TestDecideRejectKeepsTheTrimmedReasonAndApproveDropsIt(t *testing.T) {
	repo := &decisionFake{fakeRepository: fakeRepository{detail: interviewedDetail()}}
	svc, err := New(repo, &fakeCVLinker{})
	if err != nil {
		t.Fatal(err)
	}
	reason := "  Missed the system design bar.  "
	if _, err := svc.Decide(context.Background(), recruiterID, "recruiter", postingID, applicationID, domain.DecisionReject, &reason); err != nil {
		t.Fatal(err)
	}
	if repo.decision != domain.DecisionReject || repo.reason == nil || *repo.reason != "Missed the system design bar." {
		t.Fatalf("reject passed decision %q reason %v", repo.decision, repo.reason)
	}
	if _, err := svc.Decide(context.Background(), recruiterID, "recruiter", postingID, applicationID, domain.DecisionApprove, &reason); err != nil {
		t.Fatal(err)
	}
	if repo.decision != domain.DecisionApprove || repo.reason != nil {
		t.Fatalf("approve passed decision %q reason %v, want no reason", repo.decision, repo.reason)
	}
}

func TestDecideReturnsTheDetailAfterTheDecision(t *testing.T) {
	repo := &decisionFake{fakeRepository: fakeRepository{detail: interviewedDetail()}}
	svc, err := New(repo, &fakeCVLinker{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Decide(context.Background(), recruiterID, "recruiter", postingID, applicationID, domain.DecisionApprove, nil)
	if err != nil {
		t.Fatal(err)
	}
	if repo.detailCalls != 1 {
		t.Fatalf("detail read %d times, want once after the decision", repo.detailCalls)
	}
	if got.Interview == nil || got.Interview.Status != "completed" || got.Interview.Score == nil || *got.Interview.Score != 81 {
		t.Fatalf("interview = %+v, want completed with score 81", got.Interview)
	}
	if got.FinalReview.OpensAt == nil || !got.FinalReview.CanDecide {
		t.Fatalf("final review = %+v, want the window open", got.FinalReview)
	}
}

func TestDecideGateErrorsPassThroughWithoutADetailRead(t *testing.T) {
	for _, want := range []*apperror.AppError{domain.ErrFinalReviewLocked, domain.ErrApplicationNotInterviewed, domain.ErrApplicationNotFound} {
		repo := &decisionFake{decideErr: want}
		svc, err := New(repo, &fakeCVLinker{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Decide(context.Background(), recruiterID, "recruiter", postingID, applicationID, domain.DecisionReject, nil); appErrorCode(err) != want.Code {
			t.Fatalf("got %v, want %s", err, want.Code)
		}
		if repo.detailCalls != 0 {
			t.Fatalf("%s still read the detail", want.Code)
		}
	}
}

func TestDecideNeedsTheDecisionRepository(t *testing.T) {
	svc, err := New(&fakeRepository{}, &fakeCVLinker{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(context.Background(), recruiterID, "recruiter", postingID, applicationID, domain.DecisionApprove, nil); appErrorCode(err) != apperror.CodeInternal {
		t.Fatalf("got %v, want INTERNAL_ERROR", err)
	}
}
