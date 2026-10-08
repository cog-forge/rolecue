package handler

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	authservice "github.com/cog-forge/rolecue/api/internal/features/auth/service"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct{ authService authservice.AuthService }

func NewAuthHandler(service authservice.AuthService) *AuthHandler {
	return &AuthHandler{authService: service}
}
func (h *AuthHandler) RequireAuth(cfgOrigins []string) gin.HandlerFunc {
	return middleware.RequireAuth(h.authService, config.CORSConfig{AllowOrigins: cfgOrigins})
}

// Me godoc
// @Summary Current application user
// @Tags auth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Envelope{data=authdomain.User}
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, authdomain.ErrInvalidSession)
		return
	}
	response.OK(c, user)
}
