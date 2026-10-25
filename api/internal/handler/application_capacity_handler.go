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
	FundCapacity(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, slots int32) (domain.CapacityState, error)
}

// FundCapacityRequest is the funding purchase: the number of interview slots, 1 to 10000.
type FundCapacityRequest struct {
	// Slots is the number of interview slots to buy at 1 coin each.
	Slots int32 `json:"slots" example:"5"`
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

// FundCapacity godoc
// @Summary Fund the interview capacity of your Job Posting
// @Description Recruiter only, own postings. Buys slots interview slots at 1 coin each in one transaction with the slot write: the coins leave the Recruiter's wallet and interview_slot is set. Funding is one-time. Sending the same slots again returns the current state and charges nothing; a different count returns INTERVIEW_CAPACITY_ALREADY_FUNDED. The posting must be open (INTAKE_NOT_OPEN otherwise). A wallet below the price returns INSUFFICIENT_BALANCE and changes nothing. Errors: VALIDATION_ERROR (bad ID or body, or slots outside 1 to 10000), FORBIDDEN (not a Recruiter), JOB_POSTING_NOT_FOUND (missing or not yours), INTAKE_NOT_OPEN, INTERVIEW_CAPACITY_ALREADY_FUNDED, INSUFFICIENT_BALANCE.
// @Tags applications
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Param body body FundCapacityRequest true "Interview slots to buy"
// @Success 200 {object} response.Envelope{data=domain.CapacityState}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 422 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/interview-capacity [post]
func (h *ApplicationHandler) FundCapacity(c *gin.Context) {
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
	var request FundCapacityRequest
	if !applicationJSON(c, &request) {
		return
	}
	service, err := h.capacityService()
	if err != nil {
		response.Error(c, err)
		return
	}
	state, err := service.FundCapacity(c.Request.Context(), user.ID, user.Role, postingID, request.Slots)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, state)
}
