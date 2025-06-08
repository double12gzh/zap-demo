package main

import (
	"fmt"

	"github.com/double12gzh/zap-demo/logger"
	"github.com/double12gzh/zap-demo/router"
)

func init() {
	_ = logger.InitLogger(nil)
}

func main() {
	fmt.Println("main")
	router.ServHTTP()
}
