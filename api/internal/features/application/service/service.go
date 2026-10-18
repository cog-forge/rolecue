package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const (
	recruiterRole = "recruiter"
	candidateRole = "candidate"
)

// Repository is the persistence contract of CV screening. The capacity rule
// (domain.InterviewCapacity) runs inside ApproveCV under the posting row lock, because only there
// are the counts final.
type Repository interface {
	ApproveCV(ctx context.Context, recruiterID, postingID, applicationID uuid.UUID) (domain.Screening, error)
	RejectCV(ctx context.Context, recruiterID, postingID, applicationID uuid.UUID, reason *string) (domain.Screening, error)
	CandidateEligibility(ctx context.Context, candidateID, applicationID uuid.UUID) (domain.EligibilityRecord, error)
}

type Service struct {
	repository Repository
}

func New(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("application repository is required")
	}
	return &Service{repository: repository}, nil
}

// ApproveCV passes CV screening: the Application becomes interview_eligible with a 24-hour
// interview deadline. It consumes no interview slot; the first interview start does (KAN-117).
func (s *Service) ApproveCV(ctx context.Context, recruiterID uuid.UUID, role string, postingID, applicationID uuid.UUID) (domain.Screening, error) {
	if role != recruiterRole {
		return domain.Screening{}, apperror.New(apperror.CodeForbidden, "only recruiters can screen CVs")
	}
	return s.repository.ApproveCV(ctx, recruiterID, postingID, applicationID)
}

// RejectCV fails CV screening; the Application becomes terminally rejected. The reason is optional
// (decided 2026-10-18).
func (s *Service) RejectCV(ctx context.Context, recruiterID uuid.UUID, role string, postingID, applicationID uuid.UUID, reason *string) (domain.Screening, error) {
	if role != recruiterRole {
		return domain.Screening{}, apperror.New(apperror.CodeForbidden, "only recruiters can screen CVs")
	}
	normalized, err := normalizeReason(reason)
	if err != nil {
		return domain.Screening{}, err
	}
	return s.repository.RejectCV(ctx, recruiterID, postingID, applicationID, normalized)
}

// InterviewEligibility is the server-side check a recruitment interview start must pass
// (KAN-50/KAN-117 call it): the Application is the Candidate's own, CV-approved, not yet
// interviewed or decided, and its deadline has not passed. It reads without locks, so the start
// must repeat the check inside its own transaction (posting lock first, then the Application, the
// order screening uses). Posting status needs no check: a posting closes terminally only when every
// Application is approved or rejected, so none is still interview_eligible.
func (s *Service) InterviewEligibility(ctx context.Context, candidateID uuid.UUID, role string, applicationID uuid.UUID) (domain.Eligibility, error) {
	if role != candidateRole {
		return domain.Eligibility{}, apperror.New(apperror.CodeForbidden, "only candidates can start recruitment interviews")
	}
	record, err := s.repository.CandidateEligibility(ctx, candidateID, applicationID)
	if err != nil {
		return domain.Eligibility{}, err
	}
	if record.Status != domain.StatusInterviewEligible || record.InterviewDeadline == nil || !record.WithinDeadline {
		return domain.Eligibility{}, domain.ErrApplicationNotEligible
	}
	return domain.Eligibility{
		ApplicationID: record.ApplicationID, JobPostingID: record.JobPostingID, InterviewDeadline: *record.InterviewDeadline,
	}, nil
}

func normalizeReason(reason *string) (*string, error) {
	if reason == nil {
		return nil, nil
	}
	if !utf8.ValidString(*reason) {
		return nil, apperror.New(apperror.CodeValidation, "reject_reason must be valid UTF-8")
	}
	trimmed := strings.TrimSpace(*reason)
	if trimmed == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(trimmed) > domain.MaxRejectReasonLength {
		return nil, apperror.New(apperror.CodeValidation, fmt.Sprintf("reject_reason must be at most %d characters", domain.MaxRejectReasonLength))
	}
	return &trimmed, nil
}
