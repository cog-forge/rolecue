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
// @Description Visibility: the Candidate who owns a practice interview sees all of it (criterion names of a non-system rubric are blanked). The Candidate who owns a recruitment interview sees only interview_id, type, status, started_at, completed_at, end_reason and question_count; overall_score is omitted and summary, scoring, breakdown, questions, graded_questions, summary_source and injection_flagged are absent or null. The Recruiter of the posting sees a recruitment interview in full. Everyone else gets INTERVIEW_NOT_FOUND. status is in_progress until the interview is completed, then the evaluation status (pending, completed or failed). Rubric grading (KAN-336): each graded core question has criterion results (points over max_points, a verbatim evidence quote from an answer to that question, feedback), its question metrics (100 x earned / max of its criteria of each metric) and a total for reading only. breakdown holds the session metrics (the mean over the graded questions that assess each metric) with the weight the overall used; a weighted metric no graded question assessed has a null score. overall_score is the weighted mean of the session metrics (2 decimals). scoring.outcome is complete, partial (fewer questions graded than assigned) or withheld (overall_score null; scoring.withheld_reason says why: missing_metric_coverage, no_answers, no_gradable_questions). scoring.method legacy marks an evaluation made before rubric grading, without criterion results. summary, scoring, breakdown and graded_questions are null until the evaluation completed. summary_source is always "ai". injection_flagged is true when the evaluation suspected the Candidate answers tried to steer it. Expected answers and scoring guides are never returned.
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
