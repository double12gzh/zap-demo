# Zap-Demo 高性能结构化日志库

这是一个基于 Uber `zap` 和 `lumberjack.v2` 深度封装的 Go 语言高性能结构化日志库。专为生产级微服务设计，支持开箱即用的配置、动态日志级别切换、链路追踪集成以及无感零配置降级。

---

## 核心特性

- 🚀 **极高性能**：基于 `zap` 结构化日志与内存缓冲区异步写盘（Buffered Write）。
- 🔄 **日志轮转与压缩**：集成 `lumberjack` 支持按大小、保留时间自动切割、备份和压缩日志文件。
- ⚡ **动态日志级别调整 (Dynamic Log Level)**：支持在不重启服务的情况下，通过 `SetLevel` 线程安全地在运行时热切换日志级别（例如从 `info` 切换到 `debug`）。
- 🔗 **链路追踪与 Gin 中间件解耦**：内置微服务 Trace ID 中间件（在独立子包 `logger/middleware/gin` 中），支持完全自定义 Trace ID Header，非 Web 服务引入日志核心无多余依赖。
- 🛡️ **优雅降级 (Graceful Fallback)**：支持零配置启动。如果 YAML 配置文件路径为空或文件不存在，自动降级为默认的控制台与本地文件输出，确保服务绝不因日志初始化失败而 Panic。

---

## 快速开始

### 1. 初始化日志库

支持从 YAML 文件初始化或直接使用默认配置初始化：

```go
package main

import (
	"context"
	"fmt"
	"github.com/double12gzh/zap-demo/logger"
)

func main() {
	// 方式 A：从 YAML 文件读取配置初始化（支持路径不存在时自动优雅降级）
	if err := logger.InitLoggerFromYaml("config/log.yaml"); err != nil {
		panic(err)
	}

	// 方式 B：无配置文件时，直接零配置默认初始化
	// logger.InitLogger(nil)

	// 必须：在程序退出前优雅关闭日志，将内存缓冲区中残留的日志刷入磁盘
	defer logger.GetLogger().Close()

	logger.Info(context.Background(), "系统初始化成功！")
}
```

### 2. 动态调整日志级别 (SetLevel)

在运行时，您可以通过 `SetLevel` 方法线程安全地调整日志输出级别：

```go
log := logger.GetLogger()

// 动态将级别提升为 debug
if err := log.SetLevel(logger.LogLevelDebug); err != nil {
    logger.Errorf(ctx, "修改日志级别失败: %v", err)
}

log.Debug("现在可以看到 debug 级别的日志了！")
```

### 3. 在 Gin 框架中集成链路追踪 (Trace ID)

我们已经将 Gin 框架的中间件完美解耦到了独立的子包中：

```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/double12gzh/zap-demo/logger"
	"github.com/double12gzh/zap-demo/logger/middleware/gin"
)

func main() {
	logger.InitLogger(nil)
	defer logger.GetLogger().Close()

	r := gin.Default()

	// 注入链路追踪中间件（自动提取/生成 Trace ID 并写入 Context）
	r.Use(ginlogger.RequestIDMiddleware())

	r.GET("/ping", func(c *gin.Context) {
		// 从 Context 中打印日志，会自动带上 Trace ID (X-Request-Id)
		logger.Info(c.Request.Context(), "收到 Ping 请求！")

		c.JSON(200, gin.H{"message": "pong"})
	})

	r.Run(":8080")
}
```

---

## 配置说明 (`log.yaml`)

```yaml
logger:
  level: info                # 默认日志级别: debug, info, warn, error, panic, fatal
  filename: logs/app.log     # 普通日志落盘路径
  error_filename: logs/error.log # 错误日志落盘路径（Error 及以上单独收集）
  time_format: 2006-01-02T15:04:05.000Z07:00 # 时间格式
  max_size: 100              # 单个日志文件最大大小（MB）
  max_backups: 5             # 保留的历史日志文件最大备份数
  max_age: 30                # 历史日志最大保留天数
  buffer_size: 262144        # 内存写盘缓冲区大小（256KB）
  compress: false            # 是否压缩历史日志
  console: true              # 是否同时输出到控制台
  disable_caller: false      # 是否禁用调用者位置打印
  disable_stacktrace: false  # 是否禁用堆栈打印
  enable_async: true         # 是否启用异步日志刷盘
  async_buffer_size: 262144  # 异步缓冲区大小
  async_flush_interval: 1000 # 异步刷盘间隔时间（毫秒）
  trace_key: X-Request-Id    # 自定义 Trace ID 请求头和日志字段键名
```

---

## 通用库使用与集成建议

1. **优雅退出**：在程序的生命周期结束（如收到操作系统信号）时，请务必执行 `logger.GetLogger().Close()`。对于启用了异步高并发写盘的场景，这能防止内存缓冲区中残留日志截断丢失。
2. **Context 级联传递**：在系统内部的方法调用中，请尽量传递 `context.Context`，并使用 `logger.Infof(ctx, ...)` 代替裸写，以实现链路 Trace ID 的完整贯通。

---

## 代码质量与规范检查

本项目已集成最严格的 Go 语言工程质量规范检查工具，保证每次提交的代码都符合高标准的最佳实践。

### 1. 静态代码分析 (`golangci-lint`)

项目根目录已预置配置文件 `.golangci.yml`，启用了包含安全漏洞扫描 (`gosec`)、未处理错误检查 (`errcheck`)、死代码分析 (`unused`) 等推荐的官方高优审查工具。

本地一键静态分析：
```bash
make lint
```

### 2. Git 自动提交拦截 (`pre-commit`)

项目已内置 `.pre-commit-config.yaml` 钩子配置，可在 `git commit` 时自动完成：
- 移除多余空白行与行尾空格。
- 确保所有文件以换行符正常收尾。
- 检查 YAML 文件配置语法合法性。
- 自动运行 `go fmt` 与 `goimports` 重新排版及组织 import 依赖。
- 自动运行 `golangci-lint` 进行代码质量与漏洞扫描。

#### 安装与启用方式：
1. **安装工具**：
   ```bash
   pip install pre-commit
   # 或使用 Homebrew (macOS)
   brew install pre-commit
   ```
2. **注册 Git Hook**（只需在项目根目录运行一次）：
   ```bash
   pre-commit install
   ```
3. **本地手动运行所有拦截校验**（可选，用于在提交前预检）：
   ```bash
   pre-commit run --all-files
   ```
