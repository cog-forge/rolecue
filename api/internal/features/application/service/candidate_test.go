package service

import (
	"context"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestListMineComputesCanStartInterviewAndPages(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	repo := newApplyRepository()
	repo.items = []domain.CandidateApplication{
		{ID: uuid.New(), Status: domain.StatusInterviewEligible, InterviewDeadline: &future},
		{ID: uuid.New(), Status: domain.StatusInterviewEligible, InterviewDeadline: &past},
		{ID: uuid.New(), Status: domain.StatusPending},
		{ID: uuid.New(), Status: domain.StatusRejected},
		{ID: uuid.New(), Status: domain.StatusApproved},
	}
	repo.total = 42
	svc := applyService(t, repo, newApplyCVStore())

	page, err := svc.ListMine(context.Background(), candidateID, candidateRole, domain.CandidateListFilter{Page: 3, PageSize: 5})
	if err != nil {
		t.Fatalf("ListMine() error = %v", err)
	}
	if repo.listLimit != 5 || repo.listOffset != 10 {
		t.Fatalf("limit=%d offset=%d, want 5 and 10", repo.listLimit, repo.listOffset)
	}
	if page.Total != 42 || page.Page != 3 || page.PageSize != 5 || len(page.Items) != 5 {
		t.Fatalf("page = %+v", page)
	}
	want := []bool{true, false, false, false, false}
	for i, item := range page.Items {
		if item.CanStartInterview != want[i] {
			t.Fatalf("item %d (%s) can_start_interview = %v, want %v", i, item.Status, item.CanStartInterview, want[i])
		}
	}
}

func TestListMineReturnsEmptyItemsNotNil(t *testing.T) {
	svc := applyService(t, newApplyRepository(), newApplyCVStore())
	page, err := svc.ListMine(context.Background(), candidateID, candidateRole, domain.CandidateListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListMine() error = %v", err)
	}
	if page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("items = %#v, want an empty list", page.Items)
	}
}

func TestListMineRejectsForeignRolesAndBadPaging(t *testing.T) {
	svc := applyService(t, newApplyRepository(), newApplyCVStore())
	if _, err := svc.ListMine(context.Background(), candidateID, "recruiter", domain.CandidateListFilter{Page: 1, PageSize: 20}); !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("recruiter ListMine() error = %v, want FORBIDDEN", err)
	}
	for _, filter := range []domain.CandidateListFilter{
		{Page: 0, PageSize: 20},
		{Page: 1, PageSize: 0},
		{Page: 1, PageSize: 101},
		{Page: 2147483647, PageSize: 100},
	} {
		if _, err := svc.ListMine(context.Background(), candidateID, candidateRole, filter); !apperror.IsCode(err, apperror.CodeValidation) {
			t.Fatalf("ListMine(%+v) error = %v, want VALIDATION_ERROR", filter, err)
		}
	}
}

func TestListMinePropagatesRepositoryFailure(t *testing.T) {
	repo := newApplyRepository()
	repo.listErr = apperror.New(apperror.CodeInternal, "boom")
	svc := applyService(t, repo, newApplyCVStore())
	if _, err := svc.ListMine(context.Background(), candidateID, candidateRole, domain.CandidateListFilter{Page: 1, PageSize: 20}); !apperror.IsCode(err, apperror.CodeInternal) {
		t.Fatalf("ListMine() error = %v, want INTERNAL_ERROR", err)
	}
}

func TestGetMineComputesCanStartInterview(t *testing.T) {
	future := time.Now().Add(time.Hour)
	repo := newApplyRepository()
	repo.application = domain.CandidateApplication{ID: applicationID, Status: domain.StatusInterviewEligible, InterviewDeadline: &future}
	svc := applyService(t, repo, newApplyCVStore())

	got, err := svc.GetMine(context.Background(), candidateID, candidateRole, applicationID)
	if err != nil {
		t.Fatalf("GetMine() error = %v", err)
	}
	if !got.CanStartInterview {
		t.Fatal("eligible Application before its deadline must be startable")
	}
}

func TestGetMineNotStartableWhenDeadlinePassedOrNotEligible(t *testing.T) {
	past := time.Now().Add(-time.Minute)
	for _, application := range []domain.CandidateApplication{
		{Status: domain.StatusInterviewEligible, InterviewDeadline: &past},
		{Status: domain.StatusPending},
		{Status: domain.StatusRejected},
		{Status: domain.StatusApproved},
	} {
		repo := newApplyRepository()
		repo.application = application
		svc := applyService(t, repo, newApplyCVStore())
		got, err := svc.GetMine(context.Background(), candidateID, candidateRole, applicationID)
		if err != nil {
			t.Fatalf("GetMine() error = %v", err)
		}
		if got.CanStartInterview {
			t.Fatalf("status %s must not be startable", application.Status)
		}
	}
}

func TestGetMineReportsNotFoundAndForbidden(t *testing.T) {
	repo := newApplyRepository()
	repo.getErr = domain.ErrApplicationNotFound
	svc := applyService(t, repo, newApplyCVStore())
	if _, err := svc.GetMine(context.Background(), candidateID, candidateRole, applicationID); !apperror.IsCode(err, apperror.CodeApplicationNotFound) {
		t.Fatalf("GetMine() error = %v, want APPLICATION_NOT_FOUND", err)
	}
	if _, err := svc.GetMine(context.Background(), candidateID, "recruiter", applicationID); !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("recruiter GetMine() error = %v, want FORBIDDEN", err)
	}
}
