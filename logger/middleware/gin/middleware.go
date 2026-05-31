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

// RequestIDMiddleware is a Gin middleware that injects, reads, and propagates trace IDs.
//
// The trace ID is stored as a zap.Field in the request context via StoreFieldsInContext,
// so that any logger retrieved via logger.FromContext or logger.WithContext will automatically
// include the trace ID field in every log entry — without requiring the caller to hold a
// specific logger instance.
//
// The TraceKey is captured once at middleware creation time to avoid repeated lock contention.
func RequestIDMiddleware() gin.HandlerFunc {
	l := logger.GetLogger()
	// Capture traceKey once at creation time — TraceKey is immutable after Init.
	traceKey := l.Config().TraceKey
	if traceKey == "" {
		traceKey = DefaultRequestIDHeader
	}

	return func(c *gin.Context) {
		reqID := c.GetHeader(traceKey)
		if reqID == "" {
			reqID = NewRequestID()
		}

		// Store trace ID as a zap field in context, not as a logger instance.
		// This allows any logger.WithContext(ctx) call downstream to automatically
		// include the trace ID, regardless of which logger instance is used.
		ctx := logger.StoreFieldsInContext(c.Request.Context(), zap.String(traceKey, reqID))
		c.Request = c.Request.WithContext(ctx)
		c.Header(traceKey, reqID)
		c.Next()
	}
}
