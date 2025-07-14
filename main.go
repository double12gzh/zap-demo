package main

import (
	"fmt"

	"github.com/double12gzh/zap-demo/logger"
	"github.com/double12gzh/zap-demo/router"
)

func main() {
	fmt.Println("main")

	// Initialize logger first
	if err := logger.InitLoggerFromYaml("config/log.yaml"); err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	// Start HTTP server after logger is initialized
	router.ServHTTP()
}
