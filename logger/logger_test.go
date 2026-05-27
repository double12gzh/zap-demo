package logger

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLoggerInitialization(t *testing.T) {
	ResetForTesting(t)

	// Test default config
	config := defaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, LogLevelInfo, config.Level)
	assert.Equal(t, filepath.Join("logs", "app.log"), config.Filename)
	assert.Equal(t, filepath.Join("logs", "error.log"), config.ErrorFilename)

	// Test logger initialization
	err := InitLogger(config)
	assert.NoError(t, err)

	// Test getting logger
	log := GetLogger()
	assert.NotNil(t, log)

	// Test logger methods
	log.Info("Test info message")
	log.Debug("Test debug message")
	log.Warn("Test warning message")
	log.Error("Test error message")

	// Test sugared logger
	sugar := log.GetSugaredLogger()
	assert.NotNil(t, sugar)
	sugar.Infof("Test sugared logger: %s", "info")
}

func TestLoggerWithFields(t *testing.T) {
	ResetForTesting(t)

	config := defaultConfig()
	err := InitLogger(config)
	assert.NoError(t, err)

	log := GetLogger()

	// Test WithFields
	fields := []zap.Field{
		zap.String("key1", "value1"),
		zap.Int("key2", 42),
	}
	logWithFields := log.WithFields(fields...)
	assert.NotNil(t, logWithFields)

	// Test WithFieldsMap
	fieldsMap := map[string]any{
		"key3": "value3",
		"key4": 123,
	}
	logWithFieldsMap := log.WithFieldsMap(fieldsMap)
	assert.NotNil(t, logWithFieldsMap)
}

func TestLoggerContext(t *testing.T) {
	ResetForTesting(t)

	config := defaultConfig()
	err := InitLogger(config)
	assert.NoError(t, err)

	log := GetLogger()

	// Test context with logger
	ctx := context.Background()
	ctxWithLogger := NewContextWithValue(ctx, log)
	loggerFromCtx := FromContext(ctxWithLogger)
	assert.NotNil(t, loggerFromCtx)

	// Test context with fields
	fields := []zap.Field{
		zap.String("ctx_key", "ctx_value"),
	}
	ctxWithFields := StoreFieldsInContext(ctx, fields...)
	fieldsFromCtx := GetFieldsFromContext(ctxWithFields)
	assert.Equal(t, 1, len(fieldsFromCtx))

	// Test WithContext
	logWithCtx := log.WithContext(ctxWithFields)
	assert.NotNil(t, logWithCtx)
}

func TestLoggerYamlConfig(t *testing.T) {
	ResetForTesting(t)

	// Create a temporary YAML config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
logger:
  level: debug
  filename: test.log
  error_filename: error.log
  time_format: "2006-01-02 15:04:05"
  max_size: 100
  max_backups: 5
  max_age: 30
  buffer_size: 4096
  compress: true
  console: true
  disable_caller: false
  disable_stacktrace: false
  enable_async: true
  async_buffer_size: 262144
  async_flush_interval: 1000
`

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	assert.NoError(t, err)

	// Test loading config from YAML
	config, err := LoadConfigFromYaml(configPath)
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, LogLevelDebug, config.Level)
	assert.Equal(t, "test.log", config.Filename)
	assert.Equal(t, "error.log", config.ErrorFilename)

	// Test initializing logger from YAML
	err = InitLoggerFromYaml(configPath)
	assert.NoError(t, err)
}

func TestLoggerClose(t *testing.T) {
	ResetForTesting(t)

	config := defaultConfig()
	err := InitLogger(config)
	assert.NoError(t, err)

	log := GetLogger()
	assert.NotNil(t, log)

	// Test logger close
	err = log.Close()
	assert.NoError(t, err)
}

func TestLogLevel(t *testing.T) {
	// Test LogLevel string conversion
	assert.Equal(t, "debug", LogLevelDebug.String())
	assert.Equal(t, "info", LogLevelInfo.String())
	assert.Equal(t, "warn", LogLevelWarn.String())
	assert.Equal(t, "error", LogLevelError.String())
	assert.Equal(t, "panic", LogLevelPanic.String())
	assert.Equal(t, "fatal", LogLevelFatal.String())
}

func TestDynamicLogLevel(t *testing.T) {
	ResetForTesting(t)

	config := defaultConfig()
	config.Level = LogLevelInfo
	err := InitLogger(config)
	assert.NoError(t, err)

	log := GetLogger()
	assert.NotNil(t, log)

	// Initially level is info
	assert.Equal(t, LogLevelInfo, log.Config().Level)

	// Change level to debug dynamically
	err = log.SetLevel(LogLevelDebug)
	assert.NoError(t, err)
	assert.Equal(t, LogLevelDebug, log.Config().Level)

	// Test invalid log level
	err = log.SetLevel(LogLevel("invalid"))
	assert.Error(t, err)
}

// TestSetLevelConcurrency verifies that concurrent SetLevel + Config reads
// do not trigger a data race (run with -race).
func TestSetLevelConcurrency(t *testing.T) {
	ResetForTesting(t)

	config := defaultConfig()
	config.Level = LogLevelInfo
	err := InitLogger(config)
	require.NoError(t, err)

	log := GetLogger()

	levels := []LogLevel{LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError}
	var wg sync.WaitGroup

	// Concurrently set levels
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = log.SetLevel(levels[idx%len(levels)])
		}(i)
	}

	// Concurrently read config
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = log.Config().Level
		}()
	}

	// Concurrently write logs
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			log.Info("concurrent log", zap.Int("i", idx))
		}(i)
	}

	wg.Wait()
}

// TestCloseAndWrite verifies that writing after Close does not panic.
func TestCloseAndWrite(t *testing.T) {
	ResetForTesting(t)

	tempDir := t.TempDir()
	config := &Config{
		Level:    LogLevelInfo,
		Filename: filepath.Join(tempDir, "close_test.log"),
		Console:  false,
	}
	err := InitLogger(config)
	require.NoError(t, err)

	log := GetLogger()
	err = log.Close()
	require.NoError(t, err)

	// Writing after close should not panic
	assert.NotPanics(t, func() {
		log.Info("after close")
		log.Error("error after close")
	})
}

// TestErrorFileOnlyErrors verifies that the error log file only contains
// Error-level and above messages.
func TestErrorFileOnlyErrors(t *testing.T) {
	ResetForTesting(t)

	tempDir := t.TempDir()
	config := &Config{
		Level:         LogLevelDebug,
		Filename:      filepath.Join(tempDir, "all.log"),
		ErrorFilename: filepath.Join(tempDir, "error.log"),
		Console:       false,
		BufferSize:    0, // disable buffering so writes are immediate
	}

	l, err := NewLogger(config)
	require.NoError(t, err)

	// Write messages at various levels
	l.Debug("debug msg")
	l.Info("info msg")
	l.Warn("warn msg")
	l.Error("error msg")
	_ = l.Sync()

	// Read the error log file
	errorLogData, err := os.ReadFile(filepath.Join(tempDir, "error.log")) //nolint:gosec // test fixture path
	require.NoError(t, err)

	errorLogContent := string(errorLogData)

	// Error file should contain error messages
	assert.Contains(t, errorLogContent, "error msg")

	// Error file should NOT contain debug/info/warn messages
	assert.NotContains(t, errorLogContent, "debug msg")
	assert.NotContains(t, errorLogContent, "info msg")
	assert.NotContains(t, errorLogContent, "warn msg")

	// Main log file should contain all messages
	allLogData, err := os.ReadFile(filepath.Join(tempDir, "all.log")) //nolint:gosec // test fixture path
	require.NoError(t, err)

	allLogContent := string(allLogData)
	assert.Contains(t, allLogContent, "debug msg")
	assert.Contains(t, allLogContent, "info msg")
	assert.Contains(t, allLogContent, "warn msg")
	assert.Contains(t, allLogContent, "error msg")

	_ = l.Close()
}
