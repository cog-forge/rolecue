package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

var (
	recruiterID   = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	postingID     = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	applicationID = uuid.MustParse("33333333-3333-4333-8333-333333333333")
	candidateID   = uuid.MustParse("44444444-4444-4444-8444-444444444444")
)

type fakeRepository struct {
	screening   domain.Screening
	eligibility domain.EligibilityRecord
	err         error

	approveCalls, rejectCalls, eligibilityCalls int
	recruiterID, postingID, applicationID       uuid.UUID
	candidateID                                 uuid.UUID
	reason                                      *string
}

func (f *fakeRepository) ApproveCV(_ context.Context, recruiterID, postingID, applicationID uuid.UUID) (domain.Screening, error) {
	f.approveCalls++
	f.recruiterID, f.postingID, f.applicationID = recruiterID, postingID, applicationID
	return f.screening, f.err
}

func (f *fakeRepository) RejectCV(_ context.Context, recruiterID, postingID, applicationID uuid.UUID, reason *string) (domain.Screening, error) {
	f.rejectCalls++
	f.recruiterID, f.postingID, f.applicationID, f.reason = recruiterID, postingID, applicationID, reason
	return f.screening, f.err
}

func (f *fakeRepository) CandidateEligibility(_ context.Context, candidateID, applicationID uuid.UUID) (domain.EligibilityRecord, error) {
	f.eligibilityCalls++
	f.candidateID, f.applicationID = candidateID, applicationID
	return f.eligibility, f.err
}

func newService(t *testing.T, repo *fakeRepository) *Service {
	t.Helper()
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return svc
}

func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != want {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

func ptr[T any](v T) *T { return &v }

func TestNewRequiresRepository(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) error = nil, want an error")
	}
}

func TestOnlyRecruitersScreenCVs(t *testing.T) {
	for _, role := range []string{"candidate", "admin", ""} {
		repo := &fakeRepository{}
		svc := newService(t, repo)
		_, err := svc.ApproveCV(context.Background(), recruiterID, role, postingID, applicationID)
		assertCode(t, err, apperror.CodeForbidden)
		_, err = svc.RejectCV(context.Background(), recruiterID, role, postingID, applicationID, nil)
		assertCode(t, err, apperror.CodeForbidden)
		if repo.approveCalls+repo.rejectCalls != 0 {
			t.Fatalf("role %q reached the repository", role)
		}
	}
}

func TestApproveCVScopesToTheRecruiter(t *testing.T) {
	deadline := time.Now().Add(domain.InterviewWindow)
	repo := &fakeRepository{screening: domain.Screening{ID: applicationID, Status: domain.StatusInterviewEligible, InterviewDeadline: &deadline}}
	got, err := newService(t, repo).ApproveCV(context.Background(), recruiterID, "recruiter", postingID, applicationID)
	if err != nil || got.Status != domain.StatusInterviewEligible || got.InterviewDeadline == nil {
		t.Fatalf("ApproveCV() = %+v, %v", got, err)
	}
	if repo.recruiterID != recruiterID || repo.postingID != postingID || repo.applicationID != applicationID {
		t.Fatalf("repository called with %+v, want the caller's IDs", repo)
	}
}

func TestApproveCVPassesRepositoryErrorsThrough(t *testing.T) {
	for _, want := range []*apperror.AppError{
		domain.ErrCapacityFull, domain.ErrCapacityNotFunded, domain.ErrIntakeNotOpen,
		domain.ErrApplicationNotFound, domain.ErrApplicationNotPending,
		apperror.New(apperror.CodeJobPostingNotFound, "job posting not found"),
	} {
		_, err := newService(t, &fakeRepository{err: want}).ApproveCV(context.Background(), recruiterID, "recruiter", postingID, applicationID)
		assertCode(t, err, want.Code)
	}
}

func TestRejectCVNormalizesTheReason(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason *string
		want   *string
	}{
		{"no reason", nil, nil},
		{"blank reason", ptr("   \n\t"), nil},
		{"trimmed", ptr("  Missing Go experience  "), ptr("Missing Go experience")},
		{"exactly the limit", ptr(strings.Repeat("ả", domain.MaxRejectReasonLength)), ptr(strings.Repeat("ả", domain.MaxRejectReasonLength))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{screening: domain.Screening{Status: domain.StatusRejected}}
			if _, err := newService(t, repo).RejectCV(context.Background(), recruiterID, "recruiter", postingID, applicationID, tc.reason); err != nil {
				t.Fatal(err)
			}
			if repo.rejectCalls != 1 || (repo.reason == nil) != (tc.want == nil) || (tc.want != nil && *repo.reason != *tc.want) {
				t.Fatalf("stored reason = %v, want %v", repo.reason, tc.want)
			}
		})
	}
}

func TestRejectCVRejectsBadReasons(t *testing.T) {
	for name, reason := range map[string]string{
		"too long":     strings.Repeat("a", domain.MaxRejectReasonLength+1),
		"invalid utf8": "bad \xff byte",
	} {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepository{}
			_, err := newService(t, repo).RejectCV(context.Background(), recruiterID, "recruiter", postingID, applicationID, &reason)
			assertCode(t, err, apperror.CodeValidation)
			if repo.rejectCalls != 0 {
				t.Fatal("an invalid reason reached the repository")
			}
		})
	}
}

func TestInterviewEligibility(t *testing.T) {
	deadline := time.Now().Add(time.Hour)
	eligible := domain.EligibilityRecord{
		ApplicationID: applicationID, JobPostingID: postingID, Status: domain.StatusInterviewEligible,
		InterviewDeadline: &deadline, WithinDeadline: true,
	}
	got, err := newService(t, &fakeRepository{eligibility: eligible}).InterviewEligibility(context.Background(), candidateID, "candidate", applicationID)
	if err != nil || got.ApplicationID != applicationID || got.JobPostingID != postingID || !got.InterviewDeadline.Equal(deadline) {
		t.Fatalf("InterviewEligibility() = %+v, %v, want eligible", got, err)
	}

	for name, mutate := range map[string]func(*domain.EligibilityRecord){
		"deadline passed": func(r *domain.EligibilityRecord) { r.WithinDeadline = false },
		"no deadline":     func(r *domain.EligibilityRecord) { r.InterviewDeadline = nil },
		"pending":         func(r *domain.EligibilityRecord) { r.Status = domain.StatusPending },
		"interviewed":     func(r *domain.EligibilityRecord) { r.Status = domain.StatusInterviewed },
		"approved":        func(r *domain.EligibilityRecord) { r.Status = domain.StatusApproved },
		"rejected":        func(r *domain.EligibilityRecord) { r.Status = domain.StatusRejected },
	} {
		t.Run(name, func(t *testing.T) {
			record := eligible
			mutate(&record)
			_, err := newService(t, &fakeRepository{eligibility: record}).InterviewEligibility(context.Background(), candidateID, "candidate", applicationID)
			assertCode(t, err, apperror.CodeApplicationNotEligible)
		})
	}
}

func TestInterviewEligibilityIsCandidateOnlyAndScoped(t *testing.T) {
	for _, role := range []string{"recruiter", "admin", ""} {
		repo := &fakeRepository{}
		_, err := newService(t, repo).InterviewEligibility(context.Background(), candidateID, role, applicationID)
		assertCode(t, err, apperror.CodeForbidden)
		if repo.eligibilityCalls != 0 {
			t.Fatalf("role %q reached the repository", role)
		}
	}
	repo := &fakeRepository{err: domain.ErrApplicationNotFound}
	_, err := newService(t, repo).InterviewEligibility(context.Background(), candidateID, "candidate", applicationID)
	assertCode(t, err, apperror.CodeApplicationNotFound)
	if repo.candidateID != candidateID || repo.applicationID != applicationID {
		t.Fatalf("repository called with %+v, want the caller's IDs", repo)
	}
}
