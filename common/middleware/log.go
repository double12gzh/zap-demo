package middleware

import (
	"net/http"

	"go.uber.org/zap"

	"zap-demo/logger"
)

const RequestIDHeader = "X-Request-Id"

func NewRequestID() string {
	return "unique-request-id"
}

// RequestIDMiddleware 注入/生成 request id 并写入 context
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(RequestIDHeader)
		if reqID == "" {
			reqID = NewRequestID()
		}
		ctx := logger.WithLogFields(r.Context(), zap.String(RequestIDHeader, reqID))
		r = r.WithContext(ctx)
		w.Header().Set(RequestIDHeader, reqID)
		next.ServeHTTP(w, r)
	})
}
