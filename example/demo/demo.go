package demo

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	contextx "my_logger/common/contextx"
	"my_logger/logger"
)

var L = logger.GetLogger()

// OptimizedDemo 展示 myzaplog 的优化使用方式
func Demo() {
	ctx := contextx.SetRequestID(context.Background(), "1234567890")
	logger := logger.GetLogger().WithContext(ctx)

	logger2 := logger.GetLogger()
	logger3 := logger.GetLogger()

	// 单例验证
	if logger2 != logger3 {
		panic("logger2 != logger3")
	}

	defer logger.Close()

	// 性能优化示例 - 使用预分配的字段
	start := time.Now()

	// 创建可重用的字段
	serviceField := zap.String("service", "payment")
	versionField := zap.String("version", "1.2.3")

	// 批量日志记录
	for i := 0; i < 10000; i++ {
		logger.Info("Payment processed",
			serviceField,
			versionField,
			zap.Int("transaction_id", i),
			zap.Float64("amount", float64(i)*0.99),
			zap.Duration("processing_time", time.Since(start)),
		)
	}

	elapsed := time.Since(start)
	fmt.Printf("Processed 10,000 log entries in %v (%.2f ns/op)\n",
		elapsed, float64(elapsed.Nanoseconds())/10000.0)

	// 使用上下文日志器
	contextLogger := logger.WithFields(
		zap.String("module", "database"),
		zap.String("operation", "query"),
	)

	start = time.Now()
	for i := 0; i < 1000; i++ {
		contextLogger.Info("Database query executed",
			zap.String("query", "SELECT * FROM users"),
			zap.Int("rows_returned", i%100),
			zap.Duration("query_time", time.Millisecond*time.Duration(i%50)),
		)
	}

	contextLogger.Error("Database query failed",
		zap.String("query", "SELECT * FROM users"),
		zap.Duration("query_time", time.Millisecond*time.Duration(2%50)),
	)

	fmt.Printf("Context logger: 1,000 entries in %v\n", time.Since(start))

	fmt.Println("Optimized demo completed.")
}
