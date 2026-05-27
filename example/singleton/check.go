// Package singleton verifies that the logger maintains a single global instance.
package singleton

import (
	"fmt"

	"github.com/double12gzh/zap-demo/example/demo"
	"github.com/double12gzh/zap-demo/example/demo_context"
)

// CheckSingleton verifies that demo.L and demo_context.L are the same instance.
func CheckSingleton() {
	// 先调用它们的初始化函数，确保真正获取到 Logger 实例而不再是 nil
	demo.CreateL()
	demo_context.Demo()

	if demo.L == nil {
		panic("demo.L 为 nil，未正确初始化")
	}

	// 确认变量 demo.L 和 demo_context.L 是同一个实例
	if demo.L != demo_context.L {
		panic(fmt.Sprintf("demo.L != demo_context.L, %p != %p。预期这两个是同一个实例，但是实际不是", demo.L, demo_context.L))
	}
}
