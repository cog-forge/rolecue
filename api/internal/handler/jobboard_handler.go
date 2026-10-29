package handler

import (
	"context"
	"strconv"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jobboard/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type JobBoardService interface {
	List(ctx context.Context, role string, filter domain.BoardFilter) ([]domain.BoardPosting, int64, error)
	Get(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.BoardPostingDetail, error)
}
type JobBoardHandler struct{ service JobBoardService }

func NewJobBoardHandler(service JobBoardService) *JobBoardHandler {
	return &JobBoardHandler{service: service}
}

// jobBoardFilter reads the job board filters. Each parameter may appear at most once.
func jobBoardFilter(c *gin.Context) (domain.BoardFilter, bool) {
	filter := domain.BoardFilter{Limit: 20}
	fail := func(msg string) (domain.BoardFilter, bool) {
		response.Error(c, apperror.New(apperror.CodeValidation, msg))
		return domain.BoardFilter{}, false
	}
	for _, param := range []struct {
		name     string
		value    *int32
		min, max int64
	}{{"limit", &filter.Limit, 1, 100}, {"offset", &filter.Offset, 0, 2147483647}} {
		raw, exists := c.GetQueryArray(param.name)
		if !exists {
			continue
		}
		value, err := strconv.ParseInt(raw[0], 10, 32)
		if len(raw) != 1 || err != nil || value < param.min || value > param.max {
			return fail("invalid pagination parameters")
		}
		*param.value = int32(value)
	}
	for _, param := range []struct {
		name  string
		value **string
	}{{"search", &filter.Search}, {"location", &filter.Location}, {"employment_type", &filter.EmploymentType}} {
		raw, exists := c.GetQueryArray(param.name)
		if !exists {
			continue
		}
		if len(raw) != 1 {
			return fail("invalid " + param.name)
		}
		*param.value = &raw[0]
	}
	for _, param := range []struct {
		name  string
		value **time.Time
	}{
		{"published_from", &filter.PublishedFrom}, {"published_to", &filter.PublishedTo},
	} {
		if raw, exists := c.GetQueryArray(param.name); exists {
			if len(raw) != 1 {
				return fail("invalid " + param.name)
			}
			value, err := time.Parse(time.RFC3339, raw[0])
			if err != nil {
				return fail("invalid " + param.name)
			}
			*param.value = &value
		}
	}
	return filter, true
}

// List godoc
// @Summary Browse open Job Postings
// @Description Candidate only. Job Postings an Admin approved and whose Recruiter funded interview slots, that still accept Applications, newest first. Search the title and location (case-insensitive, part of the text) and filter by employment type (case-insensitive, whole value). Never includes the question bank, evaluation weights, interview capacity or the reject reason.
// @Tags job-board
// @Produce json
// @Security SessionCookie
// @Param limit query int false "Page size" default(20) minimum(1) maximum(100)
// @Param offset query int false "Number of items to skip" default(0) minimum(0) maximum(2147483647)
// @Param published_from query string false "Publication window start, inclusive (RFC3339)"
// @Param published_to query string false "Publication window end, exclusive (RFC3339)"
// @Param search query string false "Part of the title" maxLength(200)
// @Param location query string false "Part of the location" maxLength(200)
// @Param employment_type query string false "Employment type, e.g. Full-time" maxLength(200)
// @Success 200 {object} response.Envelope{data=response.Page[domain.BoardPosting]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-board [get]
func (h *JobBoardHandler) List(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	filter, ok := jobBoardFilter(c)
	if !ok {
		return
	}
	items, total, err := h.service.List(c.Request.Context(), user.Role, filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.Page[domain.BoardPosting]{
		Items:      items,
		Pagination: response.Pagination{Limit: filter.Limit, Offset: filter.Offset, Total: total},
	})
}

// Get godoc
// @Summary View an open Job Posting
// @Description Candidate only. The public content of an open Job Posting: company, details, JD text and requirements, plus whether you already applied. A posting that does not exist, is not open or is not funded yet returns the same 404.
// @Tags job-board
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.BoardPostingDetail}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-board/{id} [get]
func (h *JobBoardHandler) Get(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	id, ok := jobPostingID(c)
	if !ok {
		return
	}
	result, err := h.service.Get(c.Request.Context(), user.ID, user.Role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}
