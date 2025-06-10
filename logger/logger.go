package logger

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	callerSkip    = 1
	timeFormat    = time.RFC3339Nano
	timeKey       = "time"
	levelKey      = "level"
	messageKey    = "msg"
	callerKey     = "caller"
	stacktraceKey = "stacktrace"
	logDir        = "logs"
	logFile       = "app.log"
	errorLogFile  = "error.log"

	// log buffer size
	bufferSize = 256 * 1024

	// log file backup config
	maxBackups = 5
	maxAge     = 30
	maxSize    = 100
)

var (
	once   sync.Once
	logger *Logger
	// Add a pool for Logger instances
	loggerPool = sync.Pool{
		New: func() any {
			return &Logger{}
		},
	}
)

type loggerKey struct{}

// NewContextWithValue returns a new context with the provided logger.
func NewContextWithValue(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}

// FromContext returns the logger from the context.
// If no logger is found, it returns the global default logger.
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(loggerKey{}).(*Logger); ok {
		return l
	}
	return GetLogger()
}

// Config log config
type Config struct {
	Level             LogLevel `json:"level"`              // log level: debug, info, warn, error, panic, fatal
	Filename          string   `json:"filename"`           // log file path
	ErrorFilename     string   `json:"error_filename"`     // error log file path, if empty, use main log file
	TimeFormat        string   `json:"time_format"`        // time format
	MaxSize           int      `json:"max_size"`           // max size of log file(MB)
	MaxBackups        int      `json:"max_backups"`        // max number of log file backups
	MaxAge            int      `json:"max_age"`            // max number of days to keep log files
	BufferSize        int      `json:"buffer_size"`        // output buffer size
	Compress          bool     `json:"compress"`           // compress old log files
	Console           bool     `json:"console"`            // output log to console
	DisableCaller     bool     `json:"disable_caller"`     // disable caller info
	DisableStacktrace bool     `json:"disable_stacktrace"` // disable stacktrace
	// Performance optimization options
	EnableAsync        bool `json:"enable_async"`         // enable async logging
	AsyncBufferSize    int  `json:"async_buffer_size"`    // async buffer size
	AsyncFlushInterval int  `json:"async_flush_interval"` // async flush interval in milliseconds
}

// Logger
type Logger struct {
	config *Config

	fileCore      zapcore.Core
	consoleCore   zapcore.Core
	errorCore     zapcore.Core
	logger        *zap.Logger
	sugaredLogger *zap.SugaredLogger
}

func InitLogger(config *Config) (err error) {
	once.Do(func() {
		logger, err = NewLogger(config)
	})

	return err
}

func GetLogger() *Logger {
	if logger == nil {
		panic("logger not initialized, please call InitLogger first")
	}

	return logger
}

// NewLogger create a new logger
func NewLogger(config *Config) (*Logger, error) {
	c := mergeConfigWithDefault(config)
	level, err := zapcore.ParseLevel(c.Level.String())
	if err != nil {
		return nil, err
	}

	// optimized encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        timeKey,
		LevelKey:       levelKey,
		MessageKey:     messageKey,
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout(c.TimeFormat),
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}

	if !c.DisableCaller {
		encoderConfig.CallerKey = callerKey
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}
	if !c.DisableStacktrace {
		encoderConfig.StacktraceKey = stacktraceKey
	}

	l := &Logger{
		config: c,
	}

	var cores []zapcore.Core

	// main log file core
	if c.Filename != "" {
		fileWriteSyncer, err := createLogWriter(c.Filename, c)
		if err != nil {
			return nil, err
		}
		if c.EnableAsync {
			fileWriteSyncer = &zapcore.BufferedWriteSyncer{
				WS:   fileWriteSyncer,
				Size: c.AsyncBufferSize,
			}
		}
		l.fileCore = createLogCore(fileWriteSyncer, encoderConfig, level)
		cores = append(cores, l.fileCore)
	}

	// error log file core
	if c.ErrorFilename != "" {
		errorWriteSyncer, err := createLogWriter(c.ErrorFilename, c)
		if err != nil {
			return nil, err
		}
		if c.EnableAsync {
			errorWriteSyncer = &zapcore.BufferedWriteSyncer{
				WS:   errorWriteSyncer,
				Size: c.AsyncBufferSize,
			}
		}
		l.errorCore = createLogCore(errorWriteSyncer, encoderConfig, zapcore.ErrorLevel)
		cores = append(cores, l.errorCore)
	}

	// console output core
	if c.Console {
		consoleEncoderConfig := encoderConfig
		consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

		consoleWriteSyncer := zapcore.AddSync(os.Stdout)
		if c.EnableAsync {
			consoleWriteSyncer = &zapcore.BufferedWriteSyncer{
				WS:   consoleWriteSyncer,
				Size: c.AsyncBufferSize,
			}
		}

		l.consoleCore = zapcore.NewCore(
			zapcore.NewConsoleEncoder(consoleEncoderConfig),
			consoleWriteSyncer,
			level,
		)
		cores = append(cores, l.consoleCore)
	}

	if len(cores) == 0 {
		cores = append(cores, zapcore.NewNopCore())
	}

	core := zapcore.NewTee(cores...)

	opts := []zap.Option{}
	if !c.DisableCaller {
		opts = append(opts, zap.AddCaller())
		opts = append(opts, zap.AddCallerSkip(callerSkip))
	}

	if !c.DisableStacktrace {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	l.logger = zap.New(core, opts...)
	l.sugaredLogger = l.logger.Sugar()

	return l, nil
}

// defaultConfig return default config
func defaultConfig() *Config {
	return &Config{
		Level:             LogLevelInfo,
		Filename:          filepath.Join(logDir, logFile),
		ErrorFilename:     filepath.Join(logDir, errorLogFile),
		TimeFormat:        timeFormat,
		MaxSize:           maxSize,
		MaxBackups:        maxBackups,
		MaxAge:            maxAge,
		BufferSize:        bufferSize,
		Compress:          true,
		Console:           true,
		DisableCaller:     false,
		DisableStacktrace: false,
		// Default performance optimization options
		EnableAsync:        false,
		AsyncBufferSize:    256 * 1024, // 256KB
		AsyncFlushInterval: 1000,       // 1 second
	}
}

func mergeConfigWithDefault(cfg *Config) *Config {
	def := defaultConfig()
	if cfg == nil {
		return def
	}
	if cfg.Level == "" {
		cfg.Level = def.Level
	}
	if cfg.Filename == "" {
		cfg.Filename = def.Filename
	}
	if cfg.ErrorFilename == "" {
		cfg.ErrorFilename = def.ErrorFilename
	}
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = def.TimeFormat
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = def.MaxSize
	}
	if cfg.MaxBackups == 0 {
		cfg.MaxBackups = def.MaxBackups
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = def.MaxAge
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = def.BufferSize
	}
	return cfg
}

// GetLogger get the logger
func (l *Logger) GetLogger() *zap.Logger {
	return l.logger
}

func (l *Logger) GetSugaredLogger() *zap.SugaredLogger {
	return l.sugaredLogger
}

// WithField add field to logger
func (l *Logger) WithField(key string, value any) *Logger {
	var field zap.Field
	switch v := any(value).(type) {
	case string:
		field = zap.String(key, v)
	case int:
		field = zap.Int(key, v)
	case int64:
		field = zap.Int64(key, v)
	case float64:
		field = zap.Float64(key, v)
	case bool:
		field = zap.Bool(key, v)
	case error:
		field = zap.Error(v)
	case time.Time:
		field = zap.Time(key, v)
	case time.Duration:
		field = zap.Duration(key, v)
	default:
		field = zap.Any(key, v)
	}

	newLogger := l.logger.With(field)
	// Get a Logger instance from pool
	newL := loggerPool.Get().(*Logger)
	newL.logger = newLogger
	newL.sugaredLogger = newLogger.Sugar()
	newL.config = l.config
	newL.fileCore = l.fileCore
	newL.consoleCore = l.consoleCore
	newL.errorCore = l.errorCore
	return newL
}

// WithFields add fields to logger
func (l *Logger) WithFields(fields ...zap.Field) *Logger {
	if len(fields) == 0 {
		return l
	}

	newLogger := l.logger.With(fields...)
	// Get a Logger instance from pool
	newL := loggerPool.Get().(*Logger)
	newL.logger = newLogger
	newL.sugaredLogger = newLogger.Sugar()
	newL.config = l.config
	newL.fileCore = l.fileCore
	newL.consoleCore = l.consoleCore
	newL.errorCore = l.errorCore
	return newL
}

// WithFieldsMap add fields from map to logger
func (l *Logger) WithFieldsMap(fields map[string]any) *Logger {
	if len(fields) == 0 {
		return l
	}

	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}

	return l.WithFields(zapFields...)
}

// WithContext returns a logger with fields extracted from context.
// If no fields are found, returns the original logger.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	fields := FieldsFromContext(ctx)
	if len(fields) == 0 {
		return l
	}
	return l.WithFields(fields...)
}

// Info log info
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.logger.Info(msg, fields...)
}

// Debug log debug
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.logger.Debug(msg, fields...)
}

// Warn log warn
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.logger.Warn(msg, fields...)
}

// Error log error
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.logger.Error(msg, fields...)
}

// Infof log info with format, use sugared logger
func (l *Logger) Infof(template string, args ...any) {
	l.sugaredLogger.Infof(template, args...)
}

// Debugf log debug with format, use sugared logger
func (l *Logger) Debugf(template string, args ...any) {
	l.sugaredLogger.Debugf(template, args...)
}

// Warnf log warn with format, use sugared logger
func (l *Logger) Warnf(template string, args ...any) {
	l.sugaredLogger.Warnf(template, args...)
}

// Errorf log error with format, use sugared logger
func (l *Logger) Errorf(template string, args ...any) {
	l.sugaredLogger.Errorf(template, args...)
}

// Sync sync the logger
func (l *Logger) Sync() error {
	return l.logger.Sync()
}

// Close sync and close the logger
func (l *Logger) Close() error {
	err := l.Sync()
	// Put the Logger instance back to pool
	loggerPool.Put(l)
	return err
}

// createLogWriter create a log writer
func createLogWriter(filename string, config *Config) (zapcore.WriteSyncer, error) {
	// ensure log directory exists
	logDir := filepath.Dir(filename)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}

	// create log file writer
	writer := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
		LocalTime:  true,
	}

	// use buffered writer to improve performance
	if config.BufferSize > 0 {
		// Use a larger buffer size for better performance
		bufferSize := config.BufferSize
		if bufferSize < 4096 {
			bufferSize = 4096 // Minimum buffer size
		}
		return &zapcore.BufferedWriteSyncer{
			WS:   zapcore.AddSync(writer),
			Size: bufferSize,
		}, nil
	}

	return zapcore.AddSync(writer), nil
}

// createLogCore create a log core
func createLogCore(writer zapcore.WriteSyncer, encoderConfig zapcore.EncoderConfig, level zapcore.Level) zapcore.Core {
	return zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		writer,
		level,
	)
}
