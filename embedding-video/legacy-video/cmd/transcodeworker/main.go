package main

import (
	"legacy-video/internal/worker/combined"
)

// main 复用统一 worker 启动器。
// 当前仓库保留该入口，主要用于与既有启动方式兼容。
func main() {
	combined.Run()
}
