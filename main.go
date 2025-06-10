package main

import (
	"fmt"

	"github.com/double12gzh/zap-demo/logger"
	"github.com/double12gzh/zap-demo/router"
)

func init() {
	_ = logger.InitLoggerFromYaml("config/log.yaml")
}

func main() {
	fmt.Println("main")
	router.ServHTTP()
}
