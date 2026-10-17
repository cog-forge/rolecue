package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OnboardingService interface {
	SelectRole(context.Context, uuid.UUID, string) (authdomain.User, error)
}
type OnboardingHandler struct{ service OnboardingService }

func NewOnboardingHandler(s OnboardingService) *OnboardingHandler {
	return &OnboardingHandler{service: s}
}

type RoleSelection struct {
	Role *string `json:"role" binding:"required"`
}

// SelectRole godoc
// @Summary Choose your onboarding role once
// @Description Same-role retries are idempotent until onboarding is completed. Changing a selected role or selecting after completion returns 409 ONBOARDING_ROLE_CONFLICT. Unknown fields reject the whole request.
// @Tags onboarding
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param request body RoleSelection true "Candidate or recruiter; body limit 16 KiB"
// @Success 200 {object} response.Envelope{data=authdomain.User}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /onboarding/role [post]
func (h *OnboardingHandler) SelectRole(c *gin.Context) {
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
	var request RoleSelection
	if err := decoder.Decode(&request); err != nil || request.Role == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	selected, err := h.service.SelectRole(c.Request.Context(), user.ID, *request.Role)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, selected)
}
