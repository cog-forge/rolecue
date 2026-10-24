package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const signedCVURL = "https://storage.example.test/signed?sig=abc"

// fakeCVLinker records what the service asked to sign and returns a link that expires ttl from now.
type fakeCVLinker struct {
	err   error
	calls int
	key   string
	ttl   time.Duration
}

func (f *fakeCVLinker) PresignGet(_ context.Context, key string, ttl time.Duration) (storage.SignedRequest, error) {
	f.calls++
	f.key, f.ttl = key, ttl
	if f.err != nil {
		return storage.SignedRequest{}, f.err
	}
	return storage.SignedRequest{URL: signedCVURL, Method: http.MethodGet, ExpiresAt: time.Now().Add(ttl)}, nil
}

func detailRecord(cvPath *string) domain.DetailRecord {
	return domain.DetailRecord{
		Item: domain.ListItem{
			ID: applicationID, JobPostingID: postingID, JobPostingTitle: "Backend Engineer",
			Status:            domain.StatusInterviewEligible,
			Candidate:         domain.ListCandidate{ID: candidateID, Name: "Ada", Email: "ada@example.test"},
			InterviewEligible: true,
		},
		CVPath:        cvPath,
		PostingStatus: "open",
		InterviewSlot: ptr(int32(3)),
		CVApprovals:   1,
	}
}

func TestGetDetailLinksTheCVForFiveMinutes(t *testing.T) {
	cvPath := "cvs/application-1.pdf"
	linker := &fakeCVLinker{}
	repo := &fakeRepository{detail: detailRecord(&cvPath)}
	svc, err := New(repo, linker)
	if err != nil {
		t.Fatal(err)
	}

	before := time.Now()
	got, err := svc.GetDetail(context.Background(), recruiterID, "recruiter", postingID, applicationID)
	if err != nil {
		t.Fatalf("GetDetail() error = %v", err)
	}
	if repo.detailCalls != 1 || repo.detailRecruiter != recruiterID || repo.detailPosting != postingID || repo.detailApp != applicationID {
		t.Fatalf("repository called %d times with %s/%s/%s, want the caller's IDs", repo.detailCalls,
			repo.detailRecruiter, repo.detailPosting, repo.detailApp)
	}
	if linker.calls != 1 || linker.key != cvPath || linker.ttl != 5*time.Minute {
		t.Fatalf("signed key %q for %v (%d calls), want %q for 5m", linker.key, linker.ttl, linker.calls, cvPath)
	}
	if got.CV == nil || got.CV.URL != signedCVURL {
		t.Fatalf("CV = %+v, want the signed link", got.CV)
	}
	if got.CV.ExpiresAt.Before(before.Add(5*time.Minute)) || got.CV.ExpiresAt.After(time.Now().Add(5*time.Minute)) {
		t.Fatalf("CV expires at %v, want about five minutes from now", got.CV.ExpiresAt)
	}
	if got.ID != applicationID || got.JobPostingID != postingID || got.JobPostingTitle != "Backend Engineer" || got.Status != domain.StatusInterviewEligible {
		t.Fatalf("list fields = %+v", got.ListItem)
	}
	if got.JobPosting.ID != postingID || got.JobPosting.Title != "Backend Engineer" || got.JobPosting.Status != "open" ||
		got.JobPosting.InterviewSlot == nil || *got.JobPosting.InterviewSlot != 3 || got.JobPosting.CVApprovals != 1 {
		t.Fatalf("JobPosting = %+v", got.JobPosting)
	}
}

func TestGetDetailWithoutCVIsNullAndNeverSigns(t *testing.T) {
	linker := &fakeCVLinker{}
	svc, err := New(&fakeRepository{detail: detailRecord(nil)}, linker)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetDetail(context.Background(), recruiterID, "recruiter", postingID, applicationID)
	if err != nil {
		t.Fatalf("GetDetail() error = %v", err)
	}
	if got.CV != nil || linker.calls != 0 {
		t.Fatalf("CV = %+v with %d signing calls, want null and no signing", got.CV, linker.calls)
	}
}

func TestGetDetailIsRecruiterOnly(t *testing.T) {
	for _, role := range []string{"candidate", "admin", ""} {
		repo := &fakeRepository{}
		linker := &fakeCVLinker{}
		svc, err := New(repo, linker)
		if err != nil {
			t.Fatal(err)
		}
		_, err = svc.GetDetail(context.Background(), recruiterID, role, postingID, applicationID)
		assertCode(t, err, apperror.CodeForbidden)
		if repo.detailCalls != 0 || linker.calls != 0 {
			t.Fatalf("role %q reached the repository or the signer", role)
		}
	}
}

func TestGetDetailPassesNotFoundThrough(t *testing.T) {
	for _, want := range []*apperror.AppError{
		domain.ErrApplicationNotFound,
		apperror.New(apperror.CodeJobPostingNotFound, "job posting not found"),
	} {
		_, err := newService(t, &fakeRepository{err: want}).GetDetail(context.Background(), recruiterID, "recruiter", postingID, applicationID)
		assertCode(t, err, want.Code)
		if err != want {
			t.Fatalf("error = %v, want the repository error %v unchanged", err, want)
		}
	}
}

func TestGetDetailSigningFailureReturnsNoBody(t *testing.T) {
	cvPath := "cvs/application-1.pdf"
	linker := &fakeCVLinker{err: errors.New("object storage unavailable")}
	svc, err := New(&fakeRepository{detail: detailRecord(&cvPath)}, linker)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetDetail(context.Background(), recruiterID, "recruiter", postingID, applicationID)
	assertCode(t, err, apperror.CodeInternal)
	if got.ID != uuid.Nil || got.CV != nil || got.JobPosting.ID != uuid.Nil {
		t.Fatalf("GetDetail() returned %+v with the error, want an empty detail", got)
	}
}
