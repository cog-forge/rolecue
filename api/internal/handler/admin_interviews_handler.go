package handler

import (
	"context"
	"strconv"

	"github.com/cog-forge/rolecue/api/internal/features/admin/interviews/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AdminInterviewsService interface {
	List(ctx context.Context, role string, filter domain.ListFilter) (domain.InterviewPage, error)
	Get(ctx context.Context, role string, id uuid.UUID) (domain.InterviewDetail, error)
}

type AdminInterviewsHandler struct{ service AdminInterviewsService }

func NewAdminInterviewsHandler(service AdminInterviewsService) *AdminInterviewsHandler {
	return &AdminInterviewsHandler{service: service}
}

func adminInterviewsRole(c *gin.Context) (string, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return "", false
	}
	return user.Role, true
}

// adminInterviewsQueryValue returns the one value of a query parameter. A parameter that appears
// more than once is rejected by the caller through ok=false.
func adminInterviewsQueryValue(c *gin.Context, name string) (value string, present bool, ok bool) {
	raw, exists := c.GetQueryArray(name)
	if !exists {
		return "", false, true
	}
	return raw[0], true, len(raw) == 1
}

// adminInterviewsFilter reads the list filters and pagination. Each parameter may appear at most
// once; malformed values are validation errors. Ranges are checked by the service.
func adminInterviewsFilter(c *gin.Context) (domain.ListFilter, bool) {
	filter := domain.ListFilter{Page: 1, PageSize: domain.DefaultPageSize}
	fail := func(message string) (domain.ListFilter, bool) {
		response.Error(c, apperror.New(apperror.CodeValidation, message))
		return domain.ListFilter{}, false
	}
	if raw, present, once := adminInterviewsQueryValue(c, "type"); present {
		if !once {
			return fail("invalid interview type filter")
		}
		filter.Type = raw
	}
	for _, name := range []string{"candidate_id", "job_posting_id"} {
		raw, present, once := adminInterviewsQueryValue(c, name)
		if !present {
			continue
		}
		id, err := uuid.Parse(raw)
		if !once || err != nil {
			return fail("invalid " + name + " filter")
		}
		if name == "candidate_id" {
			filter.CandidateID = &id
		} else {
			filter.JobPostingID = &id
		}
	}
	for _, param := range []struct {
		name  string
		value *int32
	}{{"page", &filter.Page}, {"page_size", &filter.PageSize}} {
		raw, present, once := adminInterviewsQueryValue(c, param.name)
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
// @Summary List interview sessions
// @Description Admin only. Operational metadata only: no recording path, audio, video, transcript or answer text. Sorted by interview ID. Page size is capped at 100.
// @Tags admin-interviews
// @Produce json
// @Security SessionCookie
// @Param type query string false "practice or recruitment"
// @Param candidate_id query string false "Candidate (owner) ID"
// @Param job_posting_id query string false "Job Posting ID"
// @Param page query int false "Page number" default(1) minimum(1)
// @Param page_size query int false "Page size" default(20) minimum(1) maximum(100)
// @Success 200 {object} response.Envelope{data=domain.InterviewPage}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/interviews [get]
func (h *AdminInterviewsHandler) List(c *gin.Context) {
	role, ok := adminInterviewsRole(c)
	if !ok {
		return
	}
	filter, ok := adminInterviewsFilter(c)
	if !ok {
		return
	}
	result, err := h.service.List(c.Request.Context(), role, filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Get godoc
// @Summary Get an interview session's operational detail
// @Description Admin only. Read-only. Returns identifiers, names, score and counts. Never returns a recording path, audio, video, transcript, answer or feedback text.
// @Tags admin-interviews
// @Produce json
// @Security SessionCookie
// @Param id path string true "Interview ID"
// @Success 200 {object} response.Envelope{data=domain.InterviewDetail}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/interviews/{id} [get]
func (h *AdminInterviewsHandler) Get(c *gin.Context) {
	role, ok := adminInterviewsRole(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid interview ID"))
		return
	}
	result, err := h.service.Get(c.Request.Context(), role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}
