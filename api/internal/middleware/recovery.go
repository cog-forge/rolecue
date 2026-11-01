package middleware

import (
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// RecoveryMiddleware recovers from panics during request processing and logs the stack trace
func RecoveryMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.WithContext(c.Request.Context()).Error("Panic recovered in HTTP request",
					logger.String("panic", logger.Sanitize(fmt.Sprint(err))),
					logger.String("path", c.Request.URL.Path),
				)
				response.Error(c, apperror.New(apperror.CodeInternal, "Internal server error"))
				c.Abort()
			}
		}()
		c.Next()
	}
}
