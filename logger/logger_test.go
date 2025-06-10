package logger

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestLoggerInitialization(t *testing.T) {
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
	err := os.WriteFile(configPath, []byte(configContent), 0644)
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
