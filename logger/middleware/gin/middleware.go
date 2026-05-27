// Package ginlogger provides a Gin-compatible middleware for logging trace ID propagation.
package ginlogger

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/double12gzh/zap-demo/logger"
)

// DefaultRequestIDHeader is the default HTTP header used for Trace ID propagation.
const DefaultRequestIDHeader = "X-Request-Id"

// NewRequestID generates a new random UUID string
func NewRequestID() string {
	return uuid.New().String()
}

// RequestIDMiddleware is a Gin middleware that injects, reads, and logs trace IDs dynamically
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := logger.GetLogger()
		traceKey := l.Config().TraceKey
		if traceKey == "" {
			traceKey = DefaultRequestIDHeader
		}

		reqID := c.GetHeader(traceKey)
		if reqID == "" {
			reqID = NewRequestID()
		}

		l = l.WithFields(zap.String(traceKey, reqID))
		ctx := logger.NewContextWithValue(c.Request.Context(), l)

		c.Request = c.Request.WithContext(ctx)
		c.Header(traceKey, reqID)
		c.Next()
	}
}
