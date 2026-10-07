package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/auth"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
)

const currentUserIDKey = "auth.middleware.currentUserID"
const currentUserKey = "auth.middleware.currentUser"

func RequireAuth(service auth.AuthService, cors config.CORSConfig) gin.HandlerFunc {
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
				response.Error(c, apperror.New(apperror.CodeForbidden, "a trusted Origin is required"))
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

func CurrentUser(c *gin.Context) (auth.User, bool) {
	if c == nil {
		return auth.User{}, false
	}
	value, exists := c.Get(currentUserKey)
	user, ok := value.(auth.User)
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
