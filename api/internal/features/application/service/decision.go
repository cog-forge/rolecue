package service

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// DecisionRepository is the persistence contract of the final decision. The Repository implementation
// provides it; the service asserts it at call time, as with the capacity and candidate contracts.
type DecisionRepository interface {
	DecideApplication(ctx context.Context, recruiterID, postingID, applicationID uuid.UUID, decision domain.Decision, reason *string) error
}

// Decide records the Recruiter's final decision on an interviewed Application (KAN-115). The gate is
// checked under the locks in the repository: the Application must be interviewed and now must be at or
// after the posting's latest interview deadline. Approve ignores a reason, since only a rejection keeps
// one. It returns the Application Detail after the decision, so the client has the new state.
func (s *Service) Decide(ctx context.Context, recruiterID uuid.UUID, role string, postingID, applicationID uuid.UUID, decision domain.Decision, reason *string) (domain.Detail, error) {
	if role != recruiterRole {
		return domain.Detail{}, apperror.New(apperror.CodeForbidden, "only recruiters can decide applications")
	}
	if !domain.ValidDecision(decision) {
		return domain.Detail{}, apperror.New(apperror.CodeValidation, "decision must be approve or reject")
	}
	normalized, err := normalizeReason(reason)
	if err != nil {
		return domain.Detail{}, err
	}
	if decision == domain.DecisionApprove {
		normalized = nil
	}
	repository, ok := s.repository.(DecisionRepository)
	if !ok {
		return domain.Detail{}, apperror.New(apperror.CodeInternal, "final decisions are not configured")
	}
	if err := repository.DecideApplication(ctx, recruiterID, postingID, applicationID, decision, normalized); err != nil {
		return domain.Detail{}, err
	}
	return s.GetDetail(ctx, recruiterID, role, postingID, applicationID)
}
