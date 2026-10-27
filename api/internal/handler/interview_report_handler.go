package handler

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type InterviewReportService interface {
	Report(ctx context.Context, userID uuid.UUID, role string, interviewID uuid.UUID) (domain.Report, error)
}

type InterviewReportHandler struct{ service InterviewReportService }

func NewInterviewReportHandler(service InterviewReportService) *InterviewReportHandler {
	return &InterviewReportHandler{service: service}
}

// Get godoc
// @Summary Get an interview's report
// @Description Visibility: the Candidate who owns a practice interview sees all of it. The Candidate who owns a recruitment interview sees only interview_id, type, status, overall_score, started_at and completed_at; summary, breakdown and questions are null. The Recruiter of the posting sees a recruitment interview in full. Everyone else gets INTERVIEW_NOT_FOUND. status is in_progress until the interview is completed, then the evaluation status (pending, completed or failed). summary and breakdown are null until the evaluation completed.
// @Tags interviews
// @Produce json
// @Security SessionCookie
// @Param id path string true "Interview ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.Report}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /interviews/{id}/report [get]
func (h *InterviewReportHandler) Get(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid interview ID"))
		return
	}
	report, err := h.service.Report(c.Request.Context(), user.ID, user.Role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, report)
}
