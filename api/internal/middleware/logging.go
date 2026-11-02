package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/response"
)

// SlowRequestThreshold marks a request in the access log as slow.
const SlowRequestThreshold = 2 * time.Second

// LoggingMiddleware writes one access-log line per request. The line carries the request ID, the
// matched route (not the raw path, which holds IDs), the caller's user ID and role when
// authenticated, the application error code when the handler failed, and a slow flag. The level
// follows the status: 5xx error, 4xx warn, otherwise info.
func LoggingMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		fields := []zap.Field{
			logger.String("method", c.Request.Method),
			logger.String("path", c.Request.URL.Path),
			logger.String("route", route),
			logger.Int("status", status),
			logger.Duration("latency", latency),
			zap.Int("bytes", c.Writer.Size()),
			logger.String("client_ip", c.ClientIP()),
		}
		if user, ok := CurrentUser(c); ok {
			fields = append(fields, logger.String("user_id", user.ID.String()), logger.String("role", user.Role))
		}
		if code := c.GetString(response.ErrorCodeKey); code != "" {
			fields = append(fields, logger.Code(code))
		}
		if latency >= SlowRequestThreshold {
			fields = append(fields, zap.Bool("slow", true))
		}

		reqLog := log.WithContext(c.Request.Context())
		switch {
		case status >= http.StatusInternalServerError:
			reqLog.Error("HTTP Request", fields...)
		case status >= http.StatusBadRequest:
			reqLog.Warn("HTTP Request", fields...)
		default:
			reqLog.Info("HTTP Request", fields...)
		}
	}
}
