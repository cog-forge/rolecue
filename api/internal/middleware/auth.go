package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	authservice "github.com/cog-forge/rolecue/api/internal/features/auth/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const currentUserIDKey = "auth.middleware.currentUserID"
const currentUserKey = "auth.middleware.currentUser"

// RequireAuth rejects requests without a trusted origin or a valid session. Rejections are logged by
// response.Error, with the reason in the cause chain (for example "no session cookie" or "session_expired").
func RequireAuth(service authservice.Authenticator, cors config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			allowed := false
			for _, origin := range cors.AllowOrigins {
				if origin != "*" && origin != "" && c.GetHeader("Origin") == origin {
					allowed = true
				}
			}
			if !allowed {
				response.Error(c, apperror.Wrap(apperror.CodeForbidden, "a trusted Origin is required", fmt.Errorf("origin not in allowed list for %s", c.Request.Method)))
				c.Abort()
				return
			}
		}
		user, cookies, err := service.Authenticate(c.Request.Context(), c.Request)
		for _, cookie := range cookies {
			c.Writer.Header().Add("Set-Cookie", cookie)
		}
		if err != nil {
			response.Error(c, err)
			c.Abort()
			return
		}
		c.Set(currentUserIDKey, user.ID)
		c.Set(currentUserKey, user)
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (authdomain.User, bool) {
	if c == nil {
		return authdomain.User{}, false
	}
	value, exists := c.Get(currentUserKey)
	user, ok := value.(authdomain.User)
	return user, exists && ok
}
func CurrentUserID(c *gin.Context) (uuid.UUID, bool) {
	if c == nil {
		return uuid.Nil, false
	}
	value, exists := c.Get(currentUserIDKey)
	id, ok := value.(uuid.UUID)
	return id, exists && ok
}

// RequireOnboarding protects business APIs; bootstrap/profile remain available.
func RequireOnboarding() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok || (user.Role != "admin" && !user.OnboardingCompleted) {
			cause := fmt.Errorf("onboarding incomplete role=%s onboarding_completed=%t user_id=%s", user.Role, user.OnboardingCompleted, user.ID)
			if !ok {
				cause = errors.New("no authenticated user in context")
			}
			response.Error(c, apperror.Wrap(apperror.CodeForbidden, "complete onboarding before using your workspace", cause))
			c.Abort()
			return
		}
		c.Next()
	}
}
