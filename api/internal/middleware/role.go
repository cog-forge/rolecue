package middleware

import (
	"slices"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// RequireRole rejects users whose role is not listed. It must run after RequireAuth, which loads
// the role from Postgres; a whole route group (e.g. /admin) is guarded once instead of per handler.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
			c.Abort()
			return
		}
		if !slices.Contains(roles, user.Role) {
			response.Error(c, apperror.New(apperror.CodeForbidden, "you do not have access to this resource"))
			c.Abort()
			return
		}
		c.Next()
	}
}
