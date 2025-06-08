package demo_context

import "github.com/double12gzh/zap-demo/logger"

var L *logger.Logger

func Demo() {
	L = logger.GetLogger()
}
