package main

import (
	"zap-demo/logger"
	"zap-demo/router"
)

func init() {
	_ = logger.InitLogger(nil)
}

func main() {
	router.ServHTTP()
}
