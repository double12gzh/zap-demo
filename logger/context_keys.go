package logger

import (
	"context"

	"go.uber.org/zap"
)

type ctxLogFieldsKey struct{}

func WithLogFields(ctx context.Context, fields ...zap.Field) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	existingFields := FieldsFromContext(ctx)
	return context.WithValue(ctx, ctxLogFieldsKey{}, append(existingFields, fields...))
}

func FieldsFromContext(ctx context.Context) []zap.Field {
	if ctx == nil {
		return nil
	}
	val := ctx.Value(ctxLogFieldsKey{})
	if val == nil {
		return nil
	}
	if fs, ok := val.([]zap.Field); ok {
		return fs
	}
	return nil
}
