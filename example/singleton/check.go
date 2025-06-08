package singleton

import (
	"fmt"

	"zap-demo/example/demo"
	"zap-demo/example/demo_context"
)

func CheckSingleton() {
	// 确认变量 demo.L 和 demo_context.L 是同一个实例
	if demo.L != demo_context.L {
		panic(fmt.Sprintf("demo.L != demo_context.L, %p != %p。预期这两个是同一个实例，但是实际不是", demo.L, demo_context.L))
	}
}
