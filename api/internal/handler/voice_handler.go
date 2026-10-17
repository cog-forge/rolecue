package handler

import (
	"context"
	"strconv"

	"github.com/cog-forge/rolecue/api/internal/features/voice/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type VoiceService interface {
	ListActive(ctx context.Context, role string, filter domain.ListFilter) ([]domain.Voice, int64, error)
}
type VoiceHandler struct{ service VoiceService }

func NewVoiceHandler(service VoiceService) *VoiceHandler {
	return &VoiceHandler{service: service}
}

// voiceFilter reads limit and offset. Each may appear at most once.
func voiceFilter(c *gin.Context) (domain.ListFilter, bool) {
	filter := domain.ListFilter{Limit: 20}
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
			response.Error(c, apperror.New(apperror.CodeValidation, "invalid pagination parameters"))
			return domain.ListFilter{}, false
		}
		*param.value = int32(value)
	}
	return filter, true
}

// List godoc
// @Summary List active voice profiles
// @Description Candidates and Recruiters. Only active voices, sorted by name, so they can be chosen for a practice interview or required by a Job Posting.
// @Tags voices
// @Produce json
// @Security SessionCookie
// @Param limit query int false "Page size" default(20) minimum(1) maximum(100)
// @Param offset query int false "Number of items to skip" default(0) minimum(0) maximum(2147483647)
// @Success 200 {object} response.Envelope{data=response.Page[domain.Voice]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /voices [get]
func (h *VoiceHandler) List(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	filter, ok := voiceFilter(c)
	if !ok {
		return
	}
	voices, total, err := h.service.ListActive(c.Request.Context(), user.Role, filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	if voices == nil {
		voices = []domain.Voice{}
	}
	response.OK(c, response.Page[domain.Voice]{Items: voices, Pagination: response.Pagination{Limit: filter.Limit, Offset: filter.Offset, Total: total}})
}
