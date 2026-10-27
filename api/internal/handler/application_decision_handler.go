package handler

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DecisionService is the final decision use case of an interviewed Application (KAN-115).
type DecisionService interface {
	Decide(ctx context.Context, recruiterID uuid.UUID, role string, postingID, applicationID uuid.UUID, decision domain.Decision, reason *string) (domain.Detail, error)
}

// DecisionRequest is the Recruiter's final decision on an Application.
type DecisionRequest struct {
	// Decision is approve or reject.
	Decision string `json:"decision" enums:"approve,reject" example:"approve"`
	// Reason is optional, at most 500 characters, and kept only on a rejection.
	Reason *string `json:"reason" example:"Missed the technical bar."`
}

// decisionService reads the decision use case from the handler's application service.
func (h *ApplicationHandler) decisionService() (DecisionService, error) {
	service, ok := h.service.(DecisionService)
	if !ok {
		return nil, apperror.New(apperror.CodeInternal, "final decisions are not available")
	}
	return service, nil
}

// Decide godoc
// @Summary Record the final decision on an interviewed Application
// @Description Recruiter only, own postings. Approve or reject an Application whose status is interviewed. The decision is open only once the review window has opened: now is at or after the latest interview deadline of the Job Posting (final_review.opens_at in Application Detail); before that the answer is FINAL_REVIEW_LOCKED. A reject may carry a reason of at most 500 characters; an approve ignores it. The posting closes in the same transaction when no Application is left live. Returns the Application Detail after the decision. Errors: VALIDATION_ERROR (bad IDs or body, unknown decision, reason over 500 characters), FORBIDDEN (not a Recruiter), APPLICATION_NOT_FOUND (missing, not on this posting, or the posting is not yours), APPLICATION_NOT_INTERVIEWED (not interviewed yet), FINAL_REVIEW_LOCKED (review window not open).
// @Tags applications
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Param applicationId path string true "Application ID" format(uuid)
// @Param body body DecisionRequest true "Final decision"
// @Success 200 {object} response.Envelope{data=domain.Detail}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/applications/{applicationId}/decision [post]
func (h *ApplicationHandler) Decide(c *gin.Context) {
	userID, role, postingID, applicationID, ok := applicationTarget(c)
	if !ok {
		return
	}
	var request DecisionRequest
	if !applicationJSON(c, &request) {
		return
	}
	service, err := h.decisionService()
	if err != nil {
		response.Error(c, err)
		return
	}
	detail, err := service.Decide(c.Request.Context(), userID, role, postingID, applicationID,
		domain.Decision(request.Decision), request.Reason)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, detail)
}
