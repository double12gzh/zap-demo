// Package logger provides a high-performance structured logging library
// built on top of uber-go/zap with log rotation, async writing, dynamic
// log level adjustment, and context-aware field propagation.
package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
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
	defaultBufferSize = 256 * 1024

	// DisableBuffer is a sentinel value for Config.BufferSize that explicitly
	// disables all file-write buffering, flushing every log entry immediately.
	DisableBuffer = -1

	// log file backup config
	maxBackups = 5
	maxAge     = 30
	maxSize    = 100
)

var (
	once       sync.Once
	logger     *Logger
	projectDir string
	_pool      = buffer.NewPool()
)

func init() {
	if wd, err := os.Getwd(); err == nil {
		projectDir = wd + string(os.PathSeparator)
	}
}

type loggerKey struct{}

// BoolPtr returns a pointer to the given bool value.
// Use this to set bool fields in Config explicitly, distinguishing between
// "not set" (nil, uses default) and "explicitly false".
func BoolPtr(b bool) *bool { return &b }

// NewContextWithValue returns a new context with the provided logger.
func NewContextWithValue(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}

// FromContext returns the logger from the context.
// If ctx is nil or no logger is found, it returns the global default logger.
// It automatically extracts context-scoped fields using WithContext.
func FromContext(ctx context.Context) *Logger {
	if ctx == nil {
		return GetLogger()
	}
	var l *Logger
	if val, ok := ctx.Value(loggerKey{}).(*Logger); ok {
		l = val
	}
	if l == nil {
		l = GetLogger()
	}
	return l.WithContext(ctx)
}

// Config log config
type Config struct {
	Level              LogLevel `json:"level" yaml:"level" mapstructure:"level"`                                              // log level: debug, info, warn, error, panic, fatal
	Filename           string   `json:"filename" yaml:"filename" mapstructure:"filename"`                                     // log file path
	ErrorFilename      string   `json:"error_filename" yaml:"error_filename" mapstructure:"error_filename"`                   // error log file path, if empty, use main log file
	TimeFormat         string   `json:"time_format" yaml:"time_format" mapstructure:"time_format"`                            // time format
	MaxSize            int      `json:"max_size" yaml:"max_size" mapstructure:"max_size"`                                     // max size of log file(MB)
	MaxBackups         int      `json:"max_backups" yaml:"max_backups" mapstructure:"max_backups"`                            // max number of log file backups
	MaxAge             int      `json:"max_age" yaml:"max_age" mapstructure:"max_age"`                                        // max number of days to keep log files
	BufferSize         int      `json:"buffer_size" yaml:"buffer_size" mapstructure:"buffer_size"`                            // output buffer size; 0 = default (256KB), -1 = disable buffering
	Compress           *bool    `json:"compress" yaml:"compress" mapstructure:"compress"`                                     // compress old log files; nil = use default (true)
	Console            *bool    `json:"console" yaml:"console" mapstructure:"console"`                                        // output log to console; nil = use default (true)
	DisableCaller      *bool    `json:"disable_caller" yaml:"disable_caller" mapstructure:"disable_caller"`                   // disable caller info; nil = use default (false)
	DisableStacktrace  *bool    `json:"disable_stacktrace" yaml:"disable_stacktrace" mapstructure:"disable_stacktrace"`       // disable stacktrace; nil = use default (false)
	EnableAsync        *bool    `json:"enable_async" yaml:"enable_async" mapstructure:"enable_async"`                         // enable async logging; nil = use default (false)
	AsyncBufferSize    int      `json:"async_buffer_size" yaml:"async_buffer_size" mapstructure:"async_buffer_size"`          // async buffer size
	AsyncFlushInterval int      `json:"async_flush_interval" yaml:"async_flush_interval" mapstructure:"async_flush_interval"` // async flush interval in milliseconds
	TraceKey           string   `json:"trace_key" yaml:"trace_key" mapstructure:"trace_key"`                                  // trace id header/context key
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
	pkgLogger     *zap.Logger // pre-cached with AddCallerSkip(1), for package-level funcs
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

	disableCaller := c.DisableCaller != nil && *c.DisableCaller
	disableStacktrace := c.DisableStacktrace != nil && *c.DisableStacktrace
	enableAsync := c.EnableAsync != nil && *c.EnableAsync

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

	if !disableCaller {
		encoderConfig.CallerKey = callerKey
		encoderConfig.EncodeCaller = customCallerEncoder
	}
	if !disableStacktrace {
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
	if c.Console != nil && *c.Console {
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
	if !disableCaller {
		opts = append(opts, zap.AddCaller())
		opts = append(opts, zap.AddCallerSkip(callerSkip))
	}

	if !disableStacktrace {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	l.logger = zap.New(core, opts...)
	l.pkgLogger = l.logger.WithOptions(zap.AddCallerSkip(1))
	l.sugaredLogger = l.logger.Sugar()

	if enableAsync && c.AsyncFlushInterval > 0 {
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
		Filename:           filepath.Join("logs", "app.log"),
		ErrorFilename:      filepath.Join("logs", "error.log"),
		TimeFormat:         timeFormat,
		MaxSize:            maxSize,
		MaxBackups:         maxBackups,
		MaxAge:             maxAge,
		BufferSize:         defaultBufferSize,
		Compress:           BoolPtr(true),
		Console:            BoolPtr(true),
		DisableCaller:      BoolPtr(false),
		DisableStacktrace:  BoolPtr(false),
		EnableAsync:        BoolPtr(false),
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
	// BufferSize: 0 = not set (use default), -1 = explicitly disable, >0 = custom size
	if cfg.BufferSize == 0 {
		cfg.BufferSize = def.BufferSize
	}
	// *bool fields: nil = not set (use default)
	if cfg.Compress == nil {
		cfg.Compress = def.Compress
	}
	if cfg.Console == nil {
		cfg.Console = def.Console
	}
	if cfg.DisableCaller == nil {
		cfg.DisableCaller = def.DisableCaller
	}
	if cfg.DisableStacktrace == nil {
		cfg.DisableStacktrace = def.DisableStacktrace
	}
	if cfg.EnableAsync == nil {
		cfg.EnableAsync = def.EnableAsync
	}
	if cfg.AsyncBufferSize == 0 {
		cfg.AsyncBufferSize = def.AsyncBufferSize
	}
	if cfg.AsyncFlushInterval == 0 {
		cfg.AsyncFlushInterval = def.AsyncFlushInterval
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

// Named returns a child logger with a qualified name component,
// useful for categorizing logs by subsystem (e.g. "db", "grpc").
func (l *Logger) Named(name string) *Logger {
	newLogger := l.logger.Named(name)
	return &Logger{
		config:        l.config,
		level:         l.level,
		fileCore:      l.fileCore,
		consoleCore:   l.consoleCore,
		errorCore:     l.errorCore,
		logger:        newLogger,
		pkgLogger:     newLogger.WithOptions(zap.AddCallerSkip(1)),
		sugaredLogger: newLogger.Sugar(),
	}
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
		pkgLogger:     newLogger.WithOptions(zap.AddCallerSkip(1)),
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

// --- Structured log methods (zero-alloc hot path) ---

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

// Fatal logs a message at Fatal level, then calls os.Exit(1).
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.logger.Fatal(msg, fields...)
}

// Panic logs a message at Panic level, then panics.
func (l *Logger) Panic(msg string, fields ...zap.Field) {
	l.logger.Panic(msg, fields...)
}

// --- Formatted log methods ---

// Infof log info with format
func (l *Logger) Infof(template string, args ...any) {
	l.logger.Info(fmt.Sprintf(template, args...))
}

// Debugf log debug with format
func (l *Logger) Debugf(template string, args ...any) {
	l.logger.Debug(fmt.Sprintf(template, args...))
}

// Warnf log warn with format
func (l *Logger) Warnf(template string, args ...any) {
	l.logger.Warn(fmt.Sprintf(template, args...))
}

// Errorf log error with format
func (l *Logger) Errorf(template string, args ...any) {
	l.logger.Error(fmt.Sprintf(template, args...))
}

// Fatalf logs a formatted message at Fatal level, then calls os.Exit(1).
func (l *Logger) Fatalf(template string, args ...any) {
	l.logger.Fatal(fmt.Sprintf(template, args...))
}

// Panicf logs a formatted message at Panic level, then panics.
func (l *Logger) Panicf(template string, args ...any) {
	l.logger.Panic(fmt.Sprintf(template, args...))
}

// --- io.Writer adapter ---

// logWriter adapts Logger to the io.Writer interface.
type logWriter struct {
	l   *zap.Logger
	lvl zapcore.Level
}

// Write implements io.Writer. Each call emits one log entry with the trimmed content.
func (w *logWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n\r")
	switch w.lvl {
	case zapcore.DebugLevel:
		w.l.Debug(msg)
	case zapcore.WarnLevel:
		w.l.Warn(msg)
	case zapcore.ErrorLevel:
		w.l.Error(msg)
	default:
		w.l.Info(msg)
	}
	return len(p), nil
}

// Writer returns an io.Writer that emits log entries at Info level.
// Useful for integrating with components that accept io.Writer
// (e.g. http.Server.ErrorLog, gorm logger, grpc logger).
func (l *Logger) Writer() io.Writer {
	return &logWriter{l: l.logger, lvl: zapcore.InfoLevel}
}

// WriterAt returns an io.Writer that emits log entries at the specified level.
func (l *Logger) WriterAt(level LogLevel) io.Writer {
	lvl, err := zapcore.ParseLevel(level.String())
	if err != nil {
		lvl = zapcore.InfoLevel
	}
	return &logWriter{l: l.logger, lvl: lvl}
}

// --- Lifecycle ---

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

// --- Internal helpers ---

// createLogWriter creates a raw log writer backed by lumberjack (without buffering).
// Buffering is handled separately by wrapWriteSyncer to avoid double-wrapping.
func createLogWriter(filename string, config *Config) (zapcore.WriteSyncer, error) {
	// ensure log directory exists
	logDir := filepath.Dir(filename)
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return nil, err
	}

	compress := config.Compress != nil && *config.Compress

	// create log file writer
	writer := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   compress,
		LocalTime:  true,
	}

	return zapcore.AddSync(writer), nil
}

// wrapWriteSyncer wraps a WriteSyncer with a single BufferedWriteSyncer layer.
//
// Buffering rules:
//   - EnableAsync=true: uses AsyncBufferSize with no FlushInterval (manual flush via ticker)
//   - BufferSize == DisableBuffer (-1): no buffering, every write is flushed immediately
//   - BufferSize > 0: synchronous buffering with the given size (min 4096)
//   - BufferSize == 0 (after mergeConfigWithDefault, this becomes defaultBufferSize): default 256KB buffer
func wrapWriteSyncer(ws zapcore.WriteSyncer, config *Config) zapcore.WriteSyncer {
	enableAsync := config.EnableAsync != nil && *config.EnableAsync
	if enableAsync {
		return &zapcore.BufferedWriteSyncer{
			WS:   ws,
			Size: config.AsyncBufferSize,
		}
	}
	if config.BufferSize == DisableBuffer {
		return ws // explicitly disabled
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

// --- Package-level context-aware functions ---
// These use the pre-cached pkgLogger (AddCallerSkip already applied) for zero overhead.

// Info logs a message at InfoLevel using the Logger extracted from Context.
func Info(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).pkgLogger.Info(msg, fields...)
}

// Debug logs a message at DebugLevel using the Logger extracted from Context.
func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).pkgLogger.Debug(msg, fields...)
}

// Warn logs a message at WarnLevel using the Logger extracted from Context.
func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).pkgLogger.Warn(msg, fields...)
}

// Error logs a message at ErrorLevel using the Logger extracted from Context.
func Error(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).pkgLogger.Error(msg, fields...)
}

// Fatal logs a message at FatalLevel using the Logger extracted from Context, then calls os.Exit(1).
func Fatal(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).pkgLogger.Fatal(msg, fields...)
}

// Panic logs a message at PanicLevel using the Logger extracted from Context, then panics.
func Panic(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).pkgLogger.Panic(msg, fields...)
}

// Infof formats and logs a message at InfoLevel using the Logger extracted from Context.
func Infof(ctx context.Context, template string, args ...any) {
	FromContext(ctx).pkgLogger.Info(fmt.Sprintf(template, args...))
}

// Debugf formats and logs a message at DebugLevel using the Logger extracted from Context.
func Debugf(ctx context.Context, template string, args ...any) {
	FromContext(ctx).pkgLogger.Debug(fmt.Sprintf(template, args...))
}

// Warnf formats and logs a message at WarnLevel using the Logger extracted from Context.
func Warnf(ctx context.Context, template string, args ...any) {
	FromContext(ctx).pkgLogger.Warn(fmt.Sprintf(template, args...))
}

// Errorf formats and logs a message at ErrorLevel using the Logger extracted from Context.
func Errorf(ctx context.Context, template string, args ...any) {
	FromContext(ctx).pkgLogger.Error(fmt.Sprintf(template, args...))
}

// Fatalf formats and logs a message at FatalLevel using the Logger extracted from Context, then calls os.Exit(1).
func Fatalf(ctx context.Context, template string, args ...any) {
	FromContext(ctx).pkgLogger.Fatal(fmt.Sprintf(template, args...))
}

// Panicf formats and logs a message at PanicLevel using the Logger extracted from Context, then panics.
func Panicf(ctx context.Context, template string, args ...any) {
	FromContext(ctx).pkgLogger.Panic(fmt.Sprintf(template, args...))
}

// --- Custom caller encoder ---

// customCallerEncoder dynamically trims the working directory prefix to output relative paths.
func customCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	if !caller.Defined {
		return
	}
	file := caller.File

	buf := _pool.Get()
	defer buf.Free()

	// If the file is inside the current working directory, trim the prefix to get the relative path
	if projectDir != "" && strings.HasPrefix(file, projectDir) {
		buf.AppendString(strings.TrimPrefix(file, projectDir))
	} else {
		// Otherwise, fall back to the standard short caller format (package/file.go:line)
		idx := len(file)
		slashCount := 0
		for i := len(file) - 1; i >= 0; i-- {
			if file[i] == '/' {
				slashCount++
				if slashCount == 2 {
					idx = i + 1
					break
				}
			}
		}
		buf.AppendString(file[idx:])
	}

	buf.AppendByte(':')
	buf.AppendInt(int64(caller.Line))
	enc.AppendString(buf.String())
}
