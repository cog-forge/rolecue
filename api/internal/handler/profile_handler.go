package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	profiledomain "github.com/cog-forge/rolecue/api/internal/features/profile/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ProfileService interface {
	Get(context.Context, uuid.UUID) (profiledomain.Profile, error)
	Update(context.Context, uuid.UUID, string, profiledomain.Patch) (profiledomain.Profile, error)
}
type ProfileHandler struct{ service ProfileService }

func NewProfileHandler(service ProfileService) *ProfileHandler {
	return &ProfileHandler{service: service}
}

// Get godoc
// @Summary Get your own profile
// @Tags profile
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Envelope{data=profiledomain.Profile}
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /profile [get]
func (h *ProfileHandler) Get(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	p, err := h.service.Get(c.Request.Context(), user.ID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, p)
}

// Update godoc
// @Summary Update allowed fields in your own profile
// @Description While onboarding is pending, PATCH requires full_name and (for recruiters) company_name/company_website and atomically marks completion.
// @Description Omitted keys are unchanged. Null removes image/company values; full_name cannot be null. Only recruiters may change company fields. Trusted Origin required. Unknown fields reject the whole request.
// @Tags profile
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param request body profiledomain.Patch true "Changed profile fields (at least one); body limit 16 KiB"
// @Success 200 {object} response.Envelope{data=profiledomain.Profile}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /profile [patch]
func (h *ProfileHandler) Update(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	if c.ContentType() != "application/json" {
		response.Error(c, apperror.New(apperror.CodeValidation, "Content-Type must be application/json"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var patch *profiledomain.Patch
	if err := decoder.Decode(&patch); err != nil || patch == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	p, err := h.service.Update(c.Request.Context(), user.ID, user.Role, *patch)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, p)
}
