package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/swp391-group3/ai-interview-practice/api/internal/config"
	"github.com/swp391-group3/ai-interview-practice/api/internal/features/auth"
	"github.com/swp391-group3/ai-interview-practice/api/internal/middleware"
	"github.com/swp391-group3/ai-interview-practice/api/pkg/response"
)

type AuthHandler struct{ authService auth.AuthService }

func NewAuthHandler(service auth.AuthService) *AuthHandler { return &AuthHandler{authService: service} }
func (h *AuthHandler) RequireAuth(cfgOrigins []string) gin.HandlerFunc {
	return middleware.RequireAuth(h.authService, config.CORSConfig{AllowOrigins: cfgOrigins})
}

// Me godoc
// @Summary Current application user
// @Tags auth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Envelope{data=auth.User}
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, auth.ErrInvalidSession)
		return
	}
	response.OK(c, user)
}
