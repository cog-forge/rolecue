package middleware

import (
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

// RequestIDHeader carries the correlation ID in both directions.
const RequestIDHeader = "X-Request-ID"

// A client-supplied ID is accepted only in this shape, so it cannot inject log lines or headers.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestIDMiddleware gives every request a correlation ID, echoes it in the response header and
// stores a request-scoped logger on the request context. Feature code reaches it through
// logger.FromContext(ctx), and every line it writes then carries the request_id.
func RequestIDMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !validRequestID.MatchString(id) {
			id = uuid.NewString()
		}
		c.Header(RequestIDHeader, id)
		ctx := logger.WithRequestID(c.Request.Context(), id)
		ctx = logger.ToContext(ctx, log.WithContext(ctx))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
