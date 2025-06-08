package middleware

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/double12gzh/zap-demo/logger"
)

const RequestIDHeader = "X-Request-Id"

func NewRequestID() string {
	return "unique-request-id"
}

// RequestIDMiddleware 注入/生成 request id 并写入 context
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader(RequestIDHeader)
		if reqID == "" {
			reqID = NewRequestID()
		}
		ctx := logger.WithLogFields(c.Request.Context(), zap.String(RequestIDHeader, reqID))
		c.Request = c.Request.WithContext(ctx)
		c.Header(RequestIDHeader, reqID)
		c.Next()
	}
}
