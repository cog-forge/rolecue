package service

import (
	"context"
	"math"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// maxCandidatePageSize caps one page of the Candidate's Applications.
const maxCandidatePageSize = 100

// ListMine returns one page of the Candidate's own Applications, newest first. can_start_interview
// is computed here from the stored status and deadline.
func (s *Service) ListMine(ctx context.Context, candidateID uuid.UUID, role string, filter domain.CandidateListFilter) (domain.CandidatePage, error) {
	if role != candidateRole {
		return domain.CandidatePage{}, apperror.New(apperror.CodeForbidden, "only candidates can list their applications")
	}
	if filter.Page < 1 || filter.PageSize < 1 || filter.PageSize > maxCandidatePageSize {
		return domain.CandidatePage{}, apperror.New(apperror.CodeValidation, "invalid pagination parameters")
	}
	offset := int64(filter.Page-1) * int64(filter.PageSize)
	if offset > math.MaxInt32 {
		return domain.CandidatePage{}, apperror.New(apperror.CodeValidation, "invalid pagination parameters")
	}
	repository, ok := s.repository.(CandidateRepository)
	if !ok {
		return domain.CandidatePage{}, apperror.New(apperror.CodeInternal, "candidate applications are not configured")
	}
	items, total, err := repository.ListCandidateApplications(ctx, candidateID, filter.PageSize, int32(offset))
	if err != nil {
		return domain.CandidatePage{}, err
	}
	if items == nil {
		items = []domain.CandidateApplication{}
	}
	now := time.Now()
	for i := range items {
		candidatePreflight(&items[i], now)
	}
	return domain.CandidatePage{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

// GetMine reads one of the Candidate's own Applications. Another Candidate's Application is not
// found.
func (s *Service) GetMine(ctx context.Context, candidateID uuid.UUID, role string, applicationID uuid.UUID) (domain.CandidateApplication, error) {
	if role != candidateRole {
		return domain.CandidateApplication{}, apperror.New(apperror.CodeForbidden, "only candidates can read their applications")
	}
	repository, ok := s.repository.(CandidateRepository)
	if !ok {
		return domain.CandidateApplication{}, apperror.New(apperror.CodeInternal, "candidate applications are not configured")
	}
	application, err := repository.GetCandidateApplication(ctx, candidateID, applicationID)
	if err != nil {
		return domain.CandidateApplication{}, err
	}
	candidatePreflight(&application, time.Now())
	return application, nil
}

// Read-time expiry is authoritative even between expiry worker passes; reads never mutate capacity.
func candidatePreflight(a *domain.CandidateApplication, now time.Time) {
	if !a.ReadAt.IsZero() {
		now = a.ReadAt
	}
	var blocker string
	if a.Status == domain.StatusInterviewEligible && a.InterviewDeadline != nil && !now.Before(*a.InterviewDeadline) {
		a.Status = domain.StatusRejected
		blocker = "deadline_expired"
	} else {
		switch a.Status {
		case domain.StatusPending:
			blocker = "awaiting_cv_review"
			if a.PostingUnavailable {
				blocker = "posting_unavailable"
			}
		case domain.StatusInterviewed:
			blocker = "awaiting_final_decision"
		case domain.StatusApproved, domain.StatusRejected:
			blocker = "application_decided"
		case domain.StatusInterviewEligible:
			if a.PostingUnavailable {
				blocker = "posting_unavailable"
			} else if a.InterviewDeadline == nil {
				blocker = "not_eligible"
			}
		}
	}
	a.CanStartInterview = blocker == "" && domain.CanStartInterview(a.Status, a.InterviewDeadline, now)
	a.Blocker = nil
	if blocker != "" {
		a.Blocker = &blocker
	}
}
