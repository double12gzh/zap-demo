package common

import (
	"context"

	"go.uber.org/zap"
)

type ctxKey string

const (
	requestIDKey ctxKey = "common.contextx.request_id"
)

func SetRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}
func GetRequestID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(requestIDKey).(string)
	return v, ok
}

func ContextLoggerFields(ctx context.Context) []zap.Field {
	var fields []zap.Field
	if v, ok := GetRequestID(ctx); ok {
		fields = append(fields, zap.String(string(requestIDKey), v))
	}
	return fields
}
