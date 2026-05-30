// Package logger provides a high-performance structured logging library
// built on top of uber-go/zap with log rotation, async writing, dynamic
// log level adjustment, and context-aware field propagation.
package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
)

type loggerKey struct{}

// NewContextWithValue returns a new context with the provided logger.
func NewContextWithValue(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}

// FromContext returns the logger from the context.
// If no logger is found, it returns the global default logger.
// It automatically extracts context-scoped fields using WithContext.
func FromContext(ctx context.Context) *Logger {
	var l *Logger
	if ctx != nil {
		if val, ok := ctx.Value(loggerKey{}).(*Logger); ok {
			l = val
		}
	}
	if l == nil {
		l = GetLogger()
	}
	return l.WithContext(ctx)
}

// Config log config
type Config struct {
	Level              LogLevel `json:"level" yaml:"level"`                               // log level: debug, info, warn, error, panic, fatal
	Filename           string   `json:"filename" yaml:"filename"`                         // log file path
	ErrorFilename      string   `json:"error_filename" yaml:"error_filename"`             // error log file path, if empty, use main log file
	TimeFormat         string   `json:"time_format" yaml:"time_format"`                   // time format
	MaxSize            int      `json:"max_size" yaml:"max_size"`                         // max size of log file(MB)
	MaxBackups         int      `json:"max_backups" yaml:"max_backups"`                   // max number of log file backups
	MaxAge             int      `json:"max_age" yaml:"max_age"`                           // max number of days to keep log files
	BufferSize         int      `json:"buffer_size" yaml:"buffer_size"`                   // output buffer size
	Compress           bool     `json:"compress" yaml:"compress"`                         // compress old log files
	Console            bool     `json:"console" yaml:"console"`                           // output log to console
	DisableCaller      bool     `json:"disable_caller" yaml:"disable_caller"`             // disable caller info
	DisableStacktrace  bool     `json:"disable_stacktrace" yaml:"disable_stacktrace"`     // disable stacktrace
	EnableAsync        bool     `json:"enable_async" yaml:"enable_async"`                 // enable async logging
	AsyncBufferSize    int      `json:"async_buffer_size" yaml:"async_buffer_size"`       // async buffer size
	AsyncFlushInterval int      `json:"async_flush_interval" yaml:"async_flush_interval"` // async flush interval in milliseconds
	TraceKey           string   `json:"trace_key" yaml:"trace_key"`                       // trace id header/context key
}

// Logger is the core structured logger backed by zap.
type Logger struct {
	config   *Config
	configMu sync.RWMutex // protects config fields that can be mutated at runtime
	level    zap.AtomicLevel

	fileCore      zapcore.Core
	consoleCore   zapcore.Core
	errorCore     zapcore.Core
	logger        *zap.Logger
	sugaredLogger *zap.SugaredLogger
	stopAsync     chan struct{}
	stopOnce      sync.Once
}

// InitLogger initializes the global logger singleton.
// It is safe for concurrent use but will only initialize once.
// Subsequent calls are no-ops. Use ResetForTesting in tests to re-initialize.
func InitLogger(config *Config) (err error) {
	once.Do(func() {
		logger, err = NewLogger(config)
	})

	return err
}

// GetLogger returns the global logger singleton.
// It panics if InitLogger has not been called yet.
// For a non-panicking alternative, use NewLogger directly.
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

	atomicLevel := zap.NewAtomicLevelAt(level)

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
		level:  atomicLevel,
	}

	var cores []zapcore.Core

	// main log file core
	if c.Filename != "" {
		fileWriteSyncer, err := createLogWriter(c.Filename, c)
		if err != nil {
			return nil, err
		}
		fileWriteSyncer = wrapWriteSyncer(fileWriteSyncer, c)
		l.fileCore = createLogCore(fileWriteSyncer, encoderConfig, atomicLevel)
		cores = append(cores, l.fileCore)
	}

	// error log file core
	if c.ErrorFilename != "" {
		errorWriteSyncer, err := createLogWriter(c.ErrorFilename, c)
		if err != nil {
			return nil, err
		}
		errorWriteSyncer = wrapWriteSyncer(errorWriteSyncer, c)
		l.errorCore = createLogCore(errorWriteSyncer, encoderConfig, zapcore.ErrorLevel)
		cores = append(cores, l.errorCore)
	}

	// console output core
	if c.Console {
		consoleEncoderConfig := encoderConfig
		consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

		consoleWriteSyncer := zapcore.AddSync(os.Stdout)

		l.consoleCore = zapcore.NewCore(
			zapcore.NewConsoleEncoder(consoleEncoderConfig),
			consoleWriteSyncer,
			atomicLevel,
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

	if c.EnableAsync && c.AsyncFlushInterval > 0 {
		l.stopAsync = make(chan struct{})
		go func(lg *Logger, stop chan struct{}, interval int) {
			ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_ = lg.Sync()
				case <-stop:
					return
				}
			}
		}(l, l.stopAsync, c.AsyncFlushInterval)
	}

	return l, nil
}

// defaultConfig return default config
func defaultConfig() *Config {
	return &Config{
		Level:              LogLevelInfo,
		Filename:           filepath.Join("logs", "app.log"), // 默认 logs 目录
		ErrorFilename:      filepath.Join("logs", "error.log"),
		TimeFormat:         timeFormat,
		MaxSize:            maxSize,
		MaxBackups:         maxBackups,
		MaxAge:             maxAge,
		BufferSize:         bufferSize,
		Compress:           true,
		Console:            true,
		DisableCaller:      false,
		DisableStacktrace:  false,
		EnableAsync:        false,
		AsyncBufferSize:    256 * 1024,
		AsyncFlushInterval: 1000,
		TraceKey:           "X-Request-Id",
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
	if cfg.TraceKey == "" {
		cfg.TraceKey = def.TraceKey
	}
	return cfg
}

// GetLogger returns the underlying zap.Logger.
func (l *Logger) GetLogger() *zap.Logger {
	return l.logger
}

// GetSugaredLogger returns the underlying zap.SugaredLogger.
func (l *Logger) GetSugaredLogger() *zap.SugaredLogger {
	return l.sugaredLogger
}

// Config returns a copy of the logger configuration.
// It is safe for concurrent use with SetLevel.
func (l *Logger) Config() Config {
	l.configMu.RLock()
	defer l.configMu.RUnlock()
	return *l.config
}

// SetLevel dynamically changes the log level.
// It is safe for concurrent use.
func (l *Logger) SetLevel(lvl LogLevel) error {
	parsedLevel, err := zapcore.ParseLevel(lvl.String())
	if err != nil {
		return err
	}
	l.level.SetLevel(parsedLevel)
	l.configMu.Lock()
	l.config.Level = lvl
	l.configMu.Unlock()
	return nil
}

// WithFields add fields to logger
func (l *Logger) WithFields(fields ...zap.Field) *Logger {
	if len(fields) == 0 {
		return l
	}

	newLogger := l.logger.With(fields...)
	return &Logger{
		config:        l.config,
		level:         l.level,
		fileCore:      l.fileCore,
		consoleCore:   l.consoleCore,
		errorCore:     l.errorCore,
		logger:        newLogger,
		sugaredLogger: newLogger.Sugar(),
	}
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
	fields := GetFieldsFromContext(ctx)
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
	l.stopOnce.Do(func() {
		if l.stopAsync != nil {
			close(l.stopAsync)
		}
	})
	err := l.Sync()
	// Ignore sync errors for os.Stdout and os.Stderr
	if err != nil && (strings.Contains(err.Error(), "invalid argument") || strings.Contains(err.Error(), "/dev/stdout")) {
		return nil
	}
	return err
}

// createLogWriter creates a raw log writer backed by lumberjack (without buffering).
// Buffering is handled separately by wrapWriteSyncer to avoid double-wrapping.
func createLogWriter(filename string, config *Config) (zapcore.WriteSyncer, error) {
	// ensure log directory exists
	logDir := filepath.Dir(filename)
	if err := os.MkdirAll(logDir, 0o750); err != nil {
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

	return zapcore.AddSync(writer), nil
}

// wrapWriteSyncer wraps a WriteSyncer with a single BufferedWriteSyncer layer.
// If EnableAsync is true, uses AsyncBufferSize; otherwise if BufferSize > 0,
// uses BufferSize for synchronous buffering. This prevents double-wrapping.
func wrapWriteSyncer(ws zapcore.WriteSyncer, config *Config) zapcore.WriteSyncer {
	if config.EnableAsync {
		return &zapcore.BufferedWriteSyncer{
			WS:   ws,
			Size: config.AsyncBufferSize,
		}
	}
	if config.BufferSize > 0 {
		bufSize := config.BufferSize
		if bufSize < 4096 {
			bufSize = 4096 // Minimum buffer size
		}
		return &zapcore.BufferedWriteSyncer{
			WS:   ws,
			Size: bufSize,
		}
	}
	return ws
}

// createLogCore create a log core
func createLogCore(writer zapcore.WriteSyncer, encoderConfig zapcore.EncoderConfig, level zapcore.LevelEnabler) zapcore.Core {
	return zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		writer,
		level,
	)
}

// Info logs a message at InfoLevel using the Logger extracted from Context.
func Info(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).logger.WithOptions(zap.AddCallerSkip(1)).Info(msg, fields...)
}

// Debug logs a message at DebugLevel using the Logger extracted from Context.
func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).logger.WithOptions(zap.AddCallerSkip(1)).Debug(msg, fields...)
}

// Warn logs a message at WarnLevel using the Logger extracted from Context.
func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).logger.WithOptions(zap.AddCallerSkip(1)).Warn(msg, fields...)
}

// Error logs a message at ErrorLevel using the Logger extracted from Context.
func Error(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).logger.WithOptions(zap.AddCallerSkip(1)).Error(msg, fields...)
}

// Infof formats and logs a message at InfoLevel using the Logger extracted from Context.
func Infof(ctx context.Context, template string, args ...any) {
	// 创建一个带有正确 callerSkip 的 logger
	// 使用原始的 zap logger 而不是 sugared logger，这样可以更好地控制 caller 信息
	FromContext(ctx).logger.WithOptions(zap.AddCallerSkip(1)).Info(fmt.Sprintf(template, args...))
}

// Debugf formats and logs a message at DebugLevel using the Logger extracted from Context.
func Debugf(ctx context.Context, template string, args ...any) {
	FromContext(ctx).logger.WithOptions(zap.AddCallerSkip(1)).Debug(fmt.Sprintf(template, args...))
}

// Warnf formats and logs a message at WarnLevel using the Logger extracted from Context.
func Warnf(ctx context.Context, template string, args ...any) {
	FromContext(ctx).logger.WithOptions(zap.AddCallerSkip(1)).Warn(fmt.Sprintf(template, args...))
}

// Errorf formats and logs a message at ErrorLevel using the Logger extracted from Context.
func Errorf(ctx context.Context, template string, args ...any) {
	FromContext(ctx).logger.WithOptions(zap.AddCallerSkip(1)).Error(fmt.Sprintf(template, args...))
}
