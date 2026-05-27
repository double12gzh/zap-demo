package logger

import (
	"sync"
	"testing"
)

// ResetForTesting resets the global logger singleton so that InitLogger
// can be called again. This file is only compiled during testing.
func ResetForTesting(t *testing.T) {
	t.Helper()
	once = sync.Once{}
	logger = nil
}
