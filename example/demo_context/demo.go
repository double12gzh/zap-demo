package demo_context

import "zap-demo/logger"

var L *logger.Logger

func Demo() {
	L = logger.GetLogger()
}
