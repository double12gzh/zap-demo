package demo

import (
	"context"
	"time"

	"go.uber.org/zap"

	ilogger "github.com/double12gzh/zap-demo/logger"
)

var L *ilogger.Logger

func CreateL() {
	L = ilogger.GetLogger()
}

// OptimizedDemo 展示 myzaplog 的优化使用方式
func Demo(ctx context.Context) {
	// 通过全局  logger 新生成带有独特上下文的日志记录器
	// logger := ilogger.GetLogger().WithField("func", "Demo")
	logger := ilogger.FromContext(ctx)

	// 创建可重用的字段
	serviceField := zap.String("service", "payment")
	versionField := zap.String("version", "1.2.3")

	// 批量日志记录
	for i := 0; i < 4; i++ {
		// 继承 context 中的字段, 并添加额外的字段
		logger.Info("Payment processed",
			serviceField,
			versionField,
			zap.Int("transaction_id", i),
			zap.Float64("amount", float64(i)*0.99),
		)
		// {"level":"info","time":"2025-06-08T18:35:28.962763039+08:00","caller":"demo/demo.go:28","msg":"Payment processed","func":"Demo","X-Request-Id":"test-trace-5","service":"payment","version":"1.2.3","transaction_id":3,"amount":2.9699999999999998}
	}

	// 使用上下文日志器
	contextLogger := logger.WithFields(
		zap.String("module", "database"),
	)

	for i := 0; i < 5; i++ {
		contextLogger.Info("Database query executed",
			zap.Int("rows_returned", i%100),
			zap.Duration("query_time", time.Millisecond*time.Duration(i%50)),
		)
		// {"level":"info","time":"2025-06-08T18:35:28.991271188+08:00","caller":"demo/demo.go:42","msg":"Database query executed","func":"Demo","module":"database","X-Request-Id":"test-trace-5","rows_returned":4,"query_time":0.004}
	}

	// 使用 WithLogFields 方法添加字段到上下文,预期在 subDemo 中使用
	ctx = ilogger.WithLogFields(ctx, zap.String("child", "myson"))

	subDemo(ctx)
}

func subDemo(ctx context.Context) {
	l := ilogger.FromContext(ctx)
	logger := l.WithField("func", "subDemo")
	logger.Info("i am sub demo")
	// {"level":"info","time":"2025-06-08T18:35:28.991516226+08:00","caller":"demo/demo.go:55","msg":"i am sub demo","func":"subDemo","X-Request-Id":"test-trace-5","child":"myson"}
}
