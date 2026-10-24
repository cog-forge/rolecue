package handler

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CapacityService is the interview capacity use case the Recruiter's posting page calls.
type CapacityService interface {
	GetCapacity(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) (domain.CapacityState, error)
}

// capacityService reads the capacity use case from the handler's application service.
func (h *ApplicationHandler) capacityService() (CapacityService, error) {
	service, ok := h.service.(CapacityService)
	if !ok {
		return nil, apperror.New(apperror.CodeInternal, "interview capacity is not available")
	}
	return service, nil
}

// CapacityState godoc
// @Summary Read the interview capacity of your Job Posting
// @Description Recruiter only, own postings. Returns the capacity separately from the submitted CV count: interview_slot (null until funded), funded, price_per_slot (1 coin per slot), cv_approvals (every Application ever CV-approved), consumed (interviews that used a slot), available_to_approve (slots no approval has taken), refundable (slots not consumed), posting_status and refunded (coins credited back, non-null only once the posting is closed). Errors: VALIDATION_ERROR (bad ID), FORBIDDEN (not a Recruiter), JOB_POSTING_NOT_FOUND (missing or not yours).
// @Tags applications
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.CapacityState}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/interview-capacity [get]
func (h *ApplicationHandler) CapacityState(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	postingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid job posting ID"))
		return
	}
	service, err := h.capacityService()
	if err != nil {
		response.Error(c, err)
		return
	}
	state, err := service.GetCapacity(c.Request.Context(), user.ID, user.Role, postingID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, state)
}
