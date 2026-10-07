package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
)

// RecoveryMiddleware recovers from panics during request processing and logs the stack trace
func RecoveryMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("Panic recovered in HTTP request",
					logger.Any("error", err),
					logger.String("path", c.Request.URL.Path),
				)
				response.Error(c, apperror.New(apperror.CodeInternal, "Internal server error"))
				c.Abort()
			}
		}()
		c.Next()
	}
}
