package logger

import (
	"context"

	"go.uber.org/zap"
)

type ctxLogFieldsKey struct{}

func StoreFieldsInContext(ctx context.Context, fields ...zap.Field) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	existingFields := GetFieldsFromContext(ctx)

	// Create a new slice to prevent slice capacity sharing data races across branched goroutines
	newFields := make([]zap.Field, len(existingFields)+len(fields))
	copy(newFields, existingFields)
	copy(newFields[len(existingFields):], fields)

	return context.WithValue(ctx, ctxLogFieldsKey{}, newFields)
}

func GetFieldsFromContext(ctx context.Context) []zap.Field {
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
