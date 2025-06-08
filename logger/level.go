package logger

// LogLevel 定义日志级别类型
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelPanic LogLevel = "panic"
	LogLevelFatal LogLevel = "fatal"
)

func (l LogLevel) String() string {
	return string(l)
}
