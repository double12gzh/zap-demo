// Package demo_context demonstrates context-aware logging with field propagation.
package demo_context

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/double12gzh/zap-demo/logger"
)

// L is the logger instance for singleton verification.
var L *logger.Logger

var demoOnce sync.Once

// Demo demonstrates how to use context-aware logging.
// Fields stored in context are automatically carried through the call chain.
func Demo() {
	demoOnce.Do(func() {
		L = logger.GetLogger()
	})

	// Create a root context and store fields into it
	ctx := context.Background()
	ctx = logger.StoreFieldsInContext(ctx, zap.String("module", "demo_context"))

	// Log using the package-level function — fields from context are automatically included
	logger.Info(ctx, "demo_context initialized with context fields")

	// Pass context to child function
	childWork(ctx)
}

func childWork(ctx context.Context) {
	// Add more fields to the context for this scope
	ctx = logger.StoreFieldsInContext(ctx, zap.String("step", "child_work"))

	// Fields from both parent and child scopes are included
	logger.Info(ctx, "child work executed with inherited context fields")
}
