package handler

import (
	"context"
	"strconv"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// InterviewHistoryService is the Candidate's session history use case. The interview service implements it.
type InterviewHistoryService interface {
	ListHistory(ctx context.Context, candidateID uuid.UUID, role string, filter domain.HistoryFilter) (domain.HistoryPage, error)
}

type InterviewHistoryHandler struct{ service InterviewHistoryService }

func NewInterviewHistoryHandler(service InterviewHistoryService) *InterviewHistoryHandler {
	return &InterviewHistoryHandler{service: service}
}

// interviewHistoryQueryValue returns the one value of a query parameter. ok is false when the parameter
// appears more than once.
func interviewHistoryQueryValue(c *gin.Context, name string) (value string, present bool, ok bool) {
	raw, exists := c.GetQueryArray(name)
	if !exists {
		return "", false, true
	}
	return raw[0], true, len(raw) == 1
}

// interviewHistoryFilter reads the type filter and the pagination. Each parameter may appear at most once;
// malformed values are validation errors. Ranges are checked by the service.
func interviewHistoryFilter(c *gin.Context) (domain.HistoryFilter, bool) {
	filter := domain.HistoryFilter{Page: 1, PageSize: domain.DefaultHistoryPageSize}
	fail := func(message string) (domain.HistoryFilter, bool) {
		response.Error(c, apperror.New(apperror.CodeValidation, message))
		return domain.HistoryFilter{}, false
	}
	if raw, present, once := interviewHistoryQueryValue(c, "type"); present {
		if !once {
			return fail("invalid interview type filter")
		}
		filter.Type = domain.Type(raw)
	}
	for _, param := range []struct {
		name  string
		value *int32
	}{{"page", &filter.Page}, {"page_size", &filter.PageSize}} {
		raw, present, once := interviewHistoryQueryValue(c, param.name)
		if !present {
			continue
		}
		value, err := strconv.ParseInt(raw, 10, 32)
		if !once || err != nil {
			return fail("invalid pagination parameters")
		}
		*param.value = int32(value)
	}
	return filter, true
}

// List godoc
// @Summary List the Candidate's interview sessions
// @Description Candidate only. Returns one page of your own sessions, newest first (started_at, then id). status is in_progress until the session is completed, then its evaluation state. Recruitment rows link to the Application through application_id. Never returns questions, answers, feedback text or recording data. Page size is capped at 100.
// @Tags interviews
// @Produce json
// @Security SessionCookie
// @Param type query string false "practice or recruitment"
// @Param page query int false "Page number" default(1) minimum(1)
// @Param page_size query int false "Page size" default(20) minimum(1) maximum(100)
// @Success 200 {object} response.Envelope{data=domain.HistoryPage}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /interviews [get]
func (h *InterviewHistoryHandler) List(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	filter, ok := interviewHistoryFilter(c)
	if !ok {
		return
	}
	result, err := h.service.ListHistory(c.Request.Context(), user.ID, user.Role, filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}
