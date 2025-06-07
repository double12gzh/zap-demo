package main

import (
	demo "my_logger/example/demo"
	ilog "my_logger/logger"
)

func main() {
	demo.Demo()

	logger := ilog.GetLogger()

	// GetLogger 返回的是同一个实例
	if logger != demo.L {
		panic("logger != myzaplog.L")
	}

	// 单例验证
	_ = ilog.InitLogger(nil)

	if logger != ilog.GetLogger() {
		panic("logger != ilog.GetLogger()")
	}

}
