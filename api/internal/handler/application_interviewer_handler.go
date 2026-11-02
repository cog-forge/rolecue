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

type companyInterviewerService interface {
	CompanyInterviewer(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID) (domain.CompanyInterviewer, error)
}

// CompanyInterviewer godoc
// @Summary Read the locked company interviewer for eligible preflight
// @Description Candidate-owned, interview-eligible Application before its deadline on a funded open or intake_closed posting. Returns only a GET model_access grant (up to five minutes, bounded by the deadline) and display names. No capacity or coin consumption. Foreign or ineligible requests return 404. Responses must not be cached.
// @Tags applications
// @Produce json
// @Security SessionCookie
// @Param id path string true "Application ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.CompanyInterviewer}
// @Failure 404 {object} response.Envelope
// @Router /applications/{id}/interviewer [get]
func (h *ApplicationHandler) CompanyInterviewer(c *gin.Context) { h.companyInterviewer(c, false) }

// SessionCompanyInterviewer godoc
// @Summary Read the company interviewer during your recruitment session
// @Description Same eligibility policy as preflight, additionally requires an unfinished recruitment session belonging to the Candidate. Practice, foreign, completed and ineligible sessions return 404.
// @Tags interviews
// @Produce json
// @Security SessionCookie
// @Param id path string true "Session ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.CompanyInterviewer}
// @Failure 404 {object} response.Envelope
// @Router /interviews/{id}/interviewer [get]
func (h *ApplicationHandler) SessionCompanyInterviewer(c *gin.Context) { h.companyInterviewer(c, true) }

func (h *ApplicationHandler) companyInterviewer(c *gin.Context, session bool) {
	c.Header("Cache-Control", "no-store")
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, domain.ErrApplicationNotFound)
		return
	}
	service, ok := h.service.(companyInterviewerService)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInternal, "company interviewer is not configured"))
		return
	}
	applicationID, sessionID := id, uuid.Nil
	if session {
		applicationID, sessionID = uuid.Nil, id
	}
	result, err := service.CompanyInterviewer(c.Request.Context(), user.ID, user.Role, applicationID, sessionID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}
