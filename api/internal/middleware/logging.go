package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

// LoggingMiddleware automatically logs requests using the Zap logger
func LoggingMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		log.WithContext(c.Request.Context()).Info("HTTP Request",
			logger.String("method", c.Request.Method),
			logger.String("path", path),
			logger.Int("status", status),
			logger.Duration("latency", latency),
			logger.String("client_ip", c.ClientIP()),
		)
	}
}
